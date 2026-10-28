package awsaccess

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/logging"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Default time limits (NFR5.1, NFR5.5, security design §3). A call of five
// minutes leaves room for the ten attempts of the retryer.
const (
	DefaultAttemptTimeout    = 30 * time.Second
	DefaultCallTimeout       = 5 * time.Minute
	DefaultCredentialTimeout = 30 * time.Second
)

// Retry settings of every AWS call (NFR5.4, reliability design §1).
const (
	// MaxAttempts is the number of attempts of one call, the first included.
	MaxAttempts = 10
	// MaxBackoff is the longest wait between two attempts. The SDK's jitter
	// keeps each wait at or below it.
	MaxBackoff = 20 * time.Second
)

// Options configures a Session. The zero value of every field except Profile
// and Region is a sensible production default.
type Options struct {
	// Profile is the --profile value; empty uses the SDK's standard resolution.
	Profile string
	// Region is the --region value; empty uses the profile or environment.
	Region string
	// Debug receives one record per API call; nil disables the records.
	Debug DebugSink
	// Transport replaces the HTTP transport of the SDK (tests only).
	Transport http.RoundTripper
	// Now returns the current time; nil means time.Now.
	Now func() time.Time
	// AttemptTimeout bounds one HTTP attempt (default 30s).
	AttemptTimeout time.Duration
	// CallTimeout bounds one API call including retries (default 5 minutes).
	CallTimeout time.Duration
	// CredentialTimeout bounds credential resolution (default 30s).
	CredentialTimeout time.Duration
	// RetryDelay replaces the SDK back-off between attempts (tests only).
	RetryDelay func(attempt int) time.Duration
	// ExplorerPageSize replaces the MaxResults of Resource Explorer's
	// ListResources (tests only, to read several pages of few resources).
	// Zero, and any value from 1000 up, mean DefaultExplorerPageSize.
	ExplorerPageSize int
}

func (o Options) withDefaults() Options {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.AttemptTimeout <= 0 {
		o.AttemptTimeout = DefaultAttemptTimeout
	}
	if o.CallTimeout <= 0 {
		o.CallTimeout = DefaultCallTimeout
	}
	if o.CredentialTimeout <= 0 {
		o.CredentialTimeout = DefaultCredentialTimeout
	}
	if o.ExplorerPageSize <= 0 || o.ExplorerPageSize >= 1000 {
		o.ExplorerPageSize = DefaultExplorerPageSize
	}
	return o
}

// Session holds one SDK configuration shared by every client. It implements
// AccountResolver, TagReader, PolicyReader, ConfigReader and ExplorerReader,
// and nothing that writes. Its methods may be called from several goroutines.
type Session struct {
	opts    Options
	cfg     aws.Config
	region  string
	profile string
	usesSSO bool
	sts     *sts.Client
	ec2     *ec2.Client
	iam     *iam.Client
	// regional holds the clients of the resource policy services per region
	// (resourcepolicies.go).
	regionalMu sync.Mutex
	regional   map[string]*regionClients
}

// regionPattern matches AWS region names such as us-east-1 or us-gov-west-1.
// Checking the form first keeps arbitrary text out of endpoint host names.
var regionPattern = regexp.MustCompile(`^[a-z]{2,4}(-[a-z0-9]+)+-[0-9]+$`)

// IsRegionName reports whether name has the form of an AWS region name. It
// does not say whether the region is enabled.
func IsRegionName(name string) bool {
	return regionPattern.MatchString(name)
}

// NewSession loads the SDK configuration for the profile and decides the
// region (BR1.1, BR1.2). It does not call AWS.
func NewSession(ctx context.Context, opts Options) (*Session, error) {
	opts = opts.withDefaults()
	loadOpts := []func(*config.LoadOptions) error{
		config.WithHTTPClient(newHTTPClient(opts)),
		config.WithRetryer(func() aws.Retryer { return newRetryer(opts.RetryDelay) }),
		// The SDK must not log requests, responses or anything else (NFR8.8).
		config.WithClientLogMode(0),
		config.WithLogger(logging.Nop{}),
	}
	if opts.Profile != "" {
		loadOpts = append(loadOpts, config.WithSharedConfigProfile(opts.Profile))
	}
	if opts.Region != "" {
		loadOpts = append(loadOpts, config.WithRegion(opts.Region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, configError(err, opts.Profile)
	}
	s := &Session{opts: opts, cfg: cfg, region: cfg.Region, profile: opts.Profile}
	s.inspectSharedConfig()
	if cfg.Credentials != nil {
		// Mark failures to obtain credentials during a call, so they are
		// reported as expired credentials rather than as unexpected errors.
		s.cfg.Credentials = &credentialsMarker{inner: cfg.Credentials, sso: s.usesSSO}
	}
	if s.region == "" {
		return nil, &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRegionNotSet, Subject: s.profile}
	}
	if !regionPattern.MatchString(s.region) {
		return nil, &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRegionInvalid, Region: s.region}
	}
	s.sts = sts.NewFromConfig(s.cfg)
	s.ec2 = ec2.NewFromConfig(s.cfg)
	s.iam = iam.NewFromConfig(s.cfg)
	return s, nil
}

// credentialsMarker wraps the credential provider of the session. The SDK
// asks it again during a call when the cached credentials expire (an SSO
// session that ended, a role session past its duration); a failure then
// carries a credentialsError, which classify reports as CredentialsMissing
// (NFR8.11) instead of an unexpected error.
type credentialsMarker struct {
	inner aws.CredentialsProvider
	sso   bool
}

func (m *credentialsMarker) Retrieve(ctx context.Context) (aws.Credentials, error) {
	creds, err := m.inner.Retrieve(ctx)
	if err != nil {
		return creds, &credentialsError{err: err, sso: m.sso}
	}
	return creds, nil
}

// credentialsError is a failure to obtain credentials.
type credentialsError struct {
	err error
	sso bool
}

func (e *credentialsError) Error() string { return "credentials could not be obtained" }
func (e *credentialsError) Unwrap() error { return e.err }

// reason is the credential reason of the failure.
func (e *credentialsError) reason() errkind.Reason {
	var invalidToken *ssocreds.InvalidTokenError
	if e.sso || errors.As(e.err, &invalidToken) {
		return errkind.ReasonSSOSessionExpired
	}
	return errkind.ReasonCredentialsExpired
}

// Region returns the region the session reads.
func (s *Session) Region() string { return s.region }

// inspectSharedConfig finds the profile name the SDK used and whether it
// signs in with AWS SSO, for the credential messages.
func (s *Session) inspectSharedConfig() {
	for _, src := range s.cfg.ConfigSources {
		sc, ok := src.(config.SharedConfig)
		if !ok {
			continue
		}
		if s.profile == "" {
			s.profile = sc.Profile
		}
		if sc.SSOSessionName != "" || sc.SSOStartURL != "" {
			s.usesSSO = true
		}
	}
}

func newHTTPClient(opts Options) aws.HTTPClient {
	if opts.Transport != nil {
		// The per-attempt limit is the HTTP client timeout: a timed-out attempt
		// is a retryable error for the SDK, while a cancelled context is not.
		return &http.Client{Transport: opts.Transport, Timeout: opts.AttemptTimeout}
	}
	return awshttp.NewBuildableClient().WithTimeout(opts.AttemptTimeout)
}

// newRetryer returns the SDK's standard retryer with ten attempts and waits of
// at most 20 seconds (NFR5.4). The retry budget of the SDK (a token bucket
// that stops retrying once used up) is turned off with ratelimit.None, so a
// long run of rate limit answers is retried ten times rather than given up
// early. delay replaces the back-off only in tests.
func newRetryer(delay func(attempt int) time.Duration) aws.Retryer {
	return retry.NewStandard(func(o *retry.StandardOptions) {
		o.MaxAttempts = MaxAttempts
		o.MaxBackoff = MaxBackoff
		o.RateLimiter = ratelimit.None
		if delay != nil {
			o.Backoff = retry.BackoffDelayerFunc(func(attempt int, _ error) (time.Duration, error) {
				return delay(attempt), nil
			})
		}
	})
}

func configError(err error, profile string) error {
	var notExist config.SharedConfigProfileNotExistError
	if errors.As(err, &notExist) {
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonProfileNotFound, Subject: profile, Cause: err}
	}
	// A syntax error in the AWS files or any other unexpected failure.
	return &errkind.Error{Kind: errkind.Internal, Op: "load AWS configuration", Cause: err}
}

// Resolve checks the credentials, identifies the caller with STS and checks
// that the region is enabled for the account (BR1.1, BR1.3). A rejection by
// STS is re-checked, because a disabled opt-in region looks the same.
func (s *Session) Resolve(ctx context.Context) (AccountContext, error) {
	if err := s.checkCredentials(ctx); err != nil {
		return AccountContext{}, err
	}
	var out *sts.GetCallerIdentityOutput
	err := s.invoke(ctx, OpGetCallerIdentity, s.region, func(ctx context.Context, add apiOption) error {
		var err error
		out, err = s.sts.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}, func(o *sts.Options) { o.APIOptions = append(o.APIOptions, add) })
		return err
	})
	if err != nil {
		return AccountContext{}, s.recheckRejectedRegion(ctx, err)
	}
	enabled, err := s.checkRegionEnabled(ctx)
	if err != nil {
		return AccountContext{}, err
	}
	return AccountContext{
		AccountID:      aws.ToString(out.Account),
		CallerARN:      aws.ToString(out.Arn),
		Profile:        s.profile,
		Region:         s.region,
		EnabledRegions: enabled,
	}, nil
}

// checkCredentials resolves the credentials once, with a time limit, so a
// missing or expired login is reported before any API call (BR1.3).
func (s *Session) checkCredentials(ctx context.Context) error {
	if s.cfg.Credentials == nil {
		return &errkind.Error{Kind: errkind.CredentialsMissing, Reason: errkind.ReasonCredentialsNotFound, Subject: s.profile}
	}
	ctx, cancel := context.WithTimeout(ctx, s.opts.CredentialTimeout)
	defer cancel()
	_, err := s.cfg.Credentials.Retrieve(ctx)
	if err == nil {
		return nil
	}
	reason := errkind.ReasonCredentialsNotFound
	var invalidToken *ssocreds.InvalidTokenError
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		reason = errkind.ReasonCredentialsTimeout
	case errors.As(err, &invalidToken) || s.usesSSO:
		reason = errkind.ReasonSSOSessionExpired
	}
	return &errkind.Error{Kind: errkind.CredentialsMissing, Reason: reason, Subject: s.profile, Cause: err}
}

// checkRegionEnabled returns the regions enabled for the account, sorted, and
// fails with InvalidInput when the session's region is not one of them
// (BR1.1).
func (s *Session) checkRegionEnabled(ctx context.Context) ([]string, error) {
	names, err := s.enabledRegions(ctx, s.region)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(names, s.region) {
		return nil, &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRegionUnavailable, Region: s.region}
	}
	slices.Sort(names)
	return names, nil
}

// enabledRegions lists the regions enabled for the account, asking the EC2
// endpoint of the region from.
func (s *Session) enabledRegions(ctx context.Context, from string) ([]string, error) {
	var out *ec2.DescribeRegionsOutput
	err := s.invoke(ctx, OpDescribeRegions, from, func(ctx context.Context, add apiOption) error {
		var err error
		out, err = s.ec2.DescribeRegions(ctx, &ec2.DescribeRegionsInput{}, func(o *ec2.Options) {
			o.Region = from
			o.APIOptions = append(o.APIOptions, add)
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Regions))
	for _, r := range out.Regions {
		names = append(names, aws.ToString(r.RegionName))
	}
	return names, nil
}

// recheckRegion is the region asked for the enabled regions when STS in the
// chosen region rejects the credentials. It is enabled in every account of
// the commercial partition, the only partition with opt-in regions.
const recheckRegion = "us-east-1"

// recheckRejectedRegion handles a rejection of the credentials by STS in the
// chosen region. STS in an opt-in region the account has not enabled rejects
// the credentials as if they were invalid, so the enabled regions are read
// from recheckRegion: when the chosen region is not among them, the region
// is reported as unavailable instead (BR1.1). In every other case, including
// a failure of the re-check, the original error stands (BR1.3).
func (s *Session) recheckRejectedRegion(ctx context.Context, stsErr error) error {
	e, ok := errkind.As(stsErr)
	if !ok || e.Reason != errkind.ReasonCredentialsRejected || !hasOptInRegions(s.region) {
		return stsErr
	}
	names, err := s.enabledRegions(ctx, recheckRegion)
	if err != nil || slices.Contains(names, s.region) {
		return stsErr
	}
	return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRegionUnavailable, Region: s.region, Cause: stsErr}
}

// hasOptInRegions reports whether region is a region of the commercial
// partition other than recheckRegion: only those can be opt-in regions.
func hasOptInRegions(region string) bool {
	if region == recheckRegion {
		return false
	}
	for _, prefix := range []string{"us-gov-", "cn-", "us-iso", "eu-iso"} {
		if strings.HasPrefix(region, prefix) {
			return false
		}
	}
	return true
}
