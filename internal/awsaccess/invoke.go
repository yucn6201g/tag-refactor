package awsaccess

import (
	"context"
	"errors"
	"net"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/middleware"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// apiOption adds a middleware to the stack of one API call.
type apiOption = func(*middleware.Stack) error

// invoke runs one API call with the call-level time limit, counts its
// attempts, converts its error into an error kind and reports it to the debug
// sink. fn performs the SDK call and must pass add to the call's APIOptions.
func (s *Session) invoke(ctx context.Context, op, region string, fn func(ctx context.Context, add apiOption) error) error {
	callCtx, cancel := context.WithTimeout(ctx, s.opts.CallTimeout)
	defer cancel()
	var attempts int32
	add := func(stack *middleware.Stack) error {
		counter := attemptCounter{n: &attempts}
		// After the retry middleware, so it runs once per attempt.
		if err := stack.Finalize.Insert(counter, "Retry", middleware.After); err != nil {
			return stack.Finalize.Add(counter, middleware.After)
		}
		return nil
	}
	start := s.opts.Now()
	err := fn(callCtx, add)
	elapsed := s.opts.Now().Sub(start)

	kerr := classify(callCtx, op, region, err)
	if s.opts.Debug != nil {
		rec := CallRecord{Op: op, Region: region, Duration: elapsed, Retries: int(atomic.LoadInt32(&attempts)) - 1}
		if rec.Retries < 0 {
			rec.Retries = -1
		}
		if kerr != nil {
			rec.ErrorKind = kerr.Kind.String()
			if kerr.Kind == errkind.Internal {
				rec.CauseType = errkind.CauseType(err)
			}
		}
		s.opts.Debug.APICall(rec)
	}
	if kerr == nil {
		return nil
	}
	return kerr
}

// attemptCounter counts the HTTP attempts of one call, so --debug can show
// the number of retries.
type attemptCounter struct{ n *int32 }

func (attemptCounter) ID() string { return "TagRefactorAttemptCounter" }

func (c attemptCounter) HandleFinalize(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (
	middleware.FinalizeOutput, middleware.Metadata, error,
) {
	atomic.AddInt32(c.n, 1)
	return next.HandleFinalize(ctx, in)
}

type kindReason struct {
	kind   errkind.Kind
	reason errkind.Reason
}

// errorCodes maps AWS error codes to error kinds. The names are shared by the
// tagging API, IAM, EC2, STS, AWS Config and Resource Explorer: the common
// errors of the query, JSON and REST-JSON protocols plus each service's own
// names for the same conditions (security design §4).
var errorCodes = map[string]kindReason{}

// opErrorCodes maps error codes that mean something else for one operation.
// Resource Explorer's ListResources answers 401 UnauthorizedException when
// the default view cannot be used; that is not a credential problem, and a
// region without a usable view must not stop the whole run (security design
// §4).
//
// The reads of single resources of the impact analysis name a resource that
// was deleted after it was listed with each service's own NotFound code
// (U5 security design §1). They are NotFound only for these operations, so
// the kinds of the U1 to U4 operations stay as they were. S3's
// NoSuchBucketPolicy is not an error at all: GetBucketPolicy reports it as a
// bucket without a policy.
var opErrorCodes = map[string]map[string]kindReason{
	OpListResources:      {"UnauthorizedException": {errkind.AccessDenied, errkind.ReasonNone}},
	OpGetBucketLocation:  notFound("NoSuchBucket"),
	OpGetBucketPolicy:    notFound("NoSuchBucket"),
	OpDescribeKey:        notFound("NotFoundException"),
	OpGetKeyPolicy:       notFound("NotFoundException"),
	OpGetQueueAttributes: notFound("QueueDoesNotExist", "AWS.SimpleQueueService.NonExistentQueue"),
	OpGetTopicAttributes: notFound("NotFound"),
	OpGetResourcePolicy:  notFound("ResourceNotFoundException"),
	// The run records of apply (U6 security design §3): a conditional write
	// refused because the lock exists, or because another conditional write
	// of it runs, is Conflict; a store without conditional writes is
	// StorageFailed.
	OpPutObject: {
		"PreconditionFailed":         {errkind.Conflict, errkind.ReasonNone},
		"ConditionalRequestConflict": {errkind.Conflict, errkind.ReasonNone},
		"NotImplemented":             {errkind.StorageFailed, errkind.ReasonNone},
	},
	OpGetObject: notFound("NoSuchKey", "NoSuchBucket"),
}

// notFound maps the codes to NotFound.
func notFound(codes ...string) map[string]kindReason {
	m := make(map[string]kindReason, len(codes))
	for _, c := range codes {
		m[c] = kindReason{errkind.NotFound, errkind.ReasonNone}
	}
	return m
}

func init() {
	add := func(kr kindReason, codes ...string) {
		for _, c := range codes {
			errorCodes[c] = kr
		}
	}
	add(kindReason{errkind.AccessDenied, errkind.ReasonNone},
		"AccessDenied", "AccessDeniedException", "UnauthorizedOperation", "UnauthorizedAccess", "AuthorizationError",
		"NotAuthorized")
	add(kindReason{errkind.CredentialsMissing, errkind.ReasonCredentialsExpired},
		"ExpiredToken", "ExpiredTokenException", "RequestExpired", "TokenRefreshRequired")
	add(kindReason{errkind.CredentialsMissing, errkind.ReasonCredentialsRejected},
		"InvalidClientTokenId", "UnrecognizedClientException", "SignatureDoesNotMatch", "AuthFailure",
		"InvalidAccessKeyId", "MissingAuthenticationToken", "MissingAuthenticationTokenException", "IncompleteSignature",
		"IncompleteSignatureException", "InvalidSignatureException", "InvalidToken")
	add(kindReason{errkind.Transient, errkind.ReasonRetriesExhausted},
		"Throttling", "ThrottlingException", "ThrottledException", "RequestThrottled", "RequestThrottledException",
		"RequestLimitExceeded", "TooManyRequestsException", "SlowDown", "EC2ThrottledException",
		"ServiceUnavailable", "ServiceUnavailableException", "Unavailable", "InternalError", "InternalFailure",
		"InternalServiceError", "InternalServerError", "InternalServerException", "InternalException",
		"RequestTimeout", "RequestTimeoutException", "PriorRequestNotComplete")
	add(kindReason{errkind.InvalidInput, errkind.ReasonRegionUnavailable},
		"RegionDisabledException", "OptInRequired")
}

// codeKind returns the kind of an AWS error code for op, and whether the code
// is known.
func codeKind(op, code string) (kindReason, bool) {
	if kr, ok := opErrorCodes[op][code]; ok {
		return kr, true
	}
	kr, ok := errorCodes[code]
	return kr, ok
}

func isKnownCode(op, code string) bool {
	_, ok := codeKind(op, code)
	return ok
}

// classify converts an SDK error into an error kind. The SDK error is kept
// as the cause, but none of its text reaches the output (BR1.4).
func classify(callCtx context.Context, op, region string, err error) *errkind.Error {
	if err == nil {
		return nil
	}
	e := &errkind.Error{Kind: errkind.Internal, Op: op, Region: region, Cause: err}
	var apiErr smithy.APIError
	var respErr *awshttp.ResponseError
	var maxAttempts *retry.MaxAttemptsError
	var quota ratelimit.QuotaExceededError
	var dnsErr *net.DNSError
	var credErr *credentialsError
	switch {
	case errors.Is(callCtx.Err(), context.DeadlineExceeded):
		e.Kind, e.Reason = errkind.Transient, errkind.ReasonTimeout
	case errors.Is(callCtx.Err(), context.Canceled):
		// The run was stopped, for example by expired credentials in another
		// region; the caller reports that error, not this one.
		e.Detail = "the call was cancelled"
	case errors.As(err, &credErr):
		// The SDK could not obtain credentials again during the call.
		e.Kind, e.Reason = errkind.CredentialsMissing, credErr.reason()
	case errors.As(err, &apiErr) && isKnownCode(op, apiErr.ErrorCode()):
		kr, _ := codeKind(op, apiErr.ErrorCode())
		e.Kind, e.Reason = kr.kind, kr.reason
	case errors.As(err, &respErr) && respErr.HTTPStatusCode() >= 500:
		e.Kind, e.Reason = errkind.Transient, errkind.ReasonRetriesExhausted
	case errors.As(err, &maxAttempts), errors.As(err, &quota):
		e.Kind, e.Reason = errkind.Transient, errkind.ReasonRetriesExhausted
	case errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		// The endpoint of the region does not exist.
		e.Kind, e.Reason = errkind.InvalidInput, errkind.ReasonRegionUnavailable
	}
	return e
}
