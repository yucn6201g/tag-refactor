package testutil

import (
	"crypto/sha1" // the AWS SSO cache names its files by SHA-1; not used for security
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// Fake credentials written to the isolated AWS files. They are easy to spot in
// output, so tests can assert that they never appear.
const (
	FakeAccessKeyID     = "AKIAFAKEFAKEFAKE0000"
	FakeSecretAccessKey = "fakeSecretAccessKeyDoNotUse0000000000000"
	FakeSessionToken    = "FAKESESSIONTOKENDONOTUSE00000000000000"
)

// Profile is one profile of the isolated AWS files.
type Profile struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

// AWSEnv is an isolated AWS environment: its own config and credentials files
// and home directory, no AWS_* variables from the real environment, and the
// EC2 instance metadata service disabled.
type AWSEnv struct {
	t               testing.TB
	Dir             string
	ConfigFile      string
	CredentialsFile string
	profiles        map[string]Profile
}

// IsolateAWSEnv points the AWS SDK at empty files in a temporary directory and
// removes every AWS_* variable for the duration of the test. The user's real
// ~/.aws is never read.
func IsolateAWSEnv(t testing.TB) *AWSEnv {
	t.Helper()
	dir := t.TempDir()
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(strings.ToUpper(name), "AWS_") {
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatalf("unset %s: %v", name, err)
			}
		}
	}
	e := &AWSEnv{
		t:               t,
		Dir:             dir,
		ConfigFile:      filepath.Join(dir, "config"),
		CredentialsFile: filepath.Join(dir, "credentials"),
		profiles:        map[string]Profile{},
	}
	e.WriteConfig("")
	e.WriteCredentials("")
	t.Setenv("AWS_CONFIG_FILE", e.ConfigFile)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", e.CredentialsFile)
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return e
}

// WriteConfig replaces the content of the isolated config file.
func (e *AWSEnv) WriteConfig(content string) {
	e.t.Helper()
	if err := os.WriteFile(e.ConfigFile, []byte(content), 0o600); err != nil {
		e.t.Fatalf("write AWS config: %v", err)
	}
}

// WriteCredentials replaces the content of the isolated credentials file.
func (e *AWSEnv) WriteCredentials(content string) {
	e.t.Helper()
	if err := os.WriteFile(e.CredentialsFile, []byte(content), 0o600); err != nil {
		e.t.Fatalf("write AWS credentials: %v", err)
	}
}

// SetProfile adds or replaces a profile and rewrites both files. The name
// "default" is the default profile.
func (e *AWSEnv) SetProfile(name string, p Profile) {
	e.t.Helper()
	e.profiles[name] = p
	names := make([]string, 0, len(e.profiles))
	for n := range e.profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	var cfg, creds strings.Builder
	for _, n := range names {
		prof := e.profiles[n]
		section := "profile " + n
		if n == "default" {
			section = "default"
		}
		fmt.Fprintf(&cfg, "[%s]\n", section)
		if prof.Region != "" {
			fmt.Fprintf(&cfg, "region = %s\n", prof.Region)
		}
		fmt.Fprintf(&creds, "[%s]\naws_access_key_id = %s\naws_secret_access_key = %s\n", n, prof.AccessKeyID, prof.SecretAccessKey)
		if prof.SessionToken != "" {
			fmt.Fprintf(&creds, "aws_session_token = %s\n", prof.SessionToken)
		}
	}
	e.WriteConfig(cfg.String())
	e.WriteCredentials(creds.String())
}

// UseFakeCredentials makes the default profile use the fake credentials and
// the given region (empty for no region).
func (e *AWSEnv) UseFakeCredentials(region string) {
	e.t.Helper()
	e.SetProfile("default", Profile{
		Region:          region,
		AccessKeyID:     FakeAccessKeyID,
		SecretAccessKey: FakeSecretAccessKey,
		SessionToken:    FakeSessionToken,
	})
}

// WriteSSOToken writes a cached AWS SSO token for cacheKey (the start URL of a
// legacy SSO profile, or the sso-session name) that expires at expiresAt.
func (e *AWSEnv) WriteSSOToken(cacheKey string, expiresAt time.Time) {
	e.t.Helper()
	sum := sha1.Sum([]byte(cacheKey))
	dir := filepath.Join(e.Dir, ".aws", "sso", "cache")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		e.t.Fatalf("create SSO cache: %v", err)
	}
	body := fmt.Sprintf(`{"accessToken":"fake-sso-access-token","expiresAt":%q,"region":"us-east-1","startUrl":"https://example.awsapps.com/start"}`,
		expiresAt.UTC().Format(time.RFC3339))
	path := filepath.Join(dir, hex.EncodeToString(sum[:])+".json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		e.t.Fatalf("write SSO token: %v", err)
	}
}
