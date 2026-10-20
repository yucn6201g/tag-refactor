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

// errorCodes maps AWS error codes to error kinds.
var errorCodes = map[string]kindReason{}

func init() {
	add := func(kr kindReason, codes ...string) {
		for _, c := range codes {
			errorCodes[c] = kr
		}
	}
	add(kindReason{errkind.AccessDenied, errkind.ReasonNone},
		"AccessDenied", "AccessDeniedException", "UnauthorizedOperation", "UnauthorizedAccess", "AuthorizationError")
	add(kindReason{errkind.CredentialsMissing, errkind.ReasonCredentialsExpired},
		"ExpiredToken", "ExpiredTokenException", "RequestExpired", "TokenRefreshRequired")
	add(kindReason{errkind.CredentialsMissing, errkind.ReasonCredentialsRejected},
		"InvalidClientTokenId", "UnrecognizedClientException", "SignatureDoesNotMatch", "AuthFailure",
		"InvalidAccessKeyId", "MissingAuthenticationToken", "IncompleteSignature", "InvalidToken")
	add(kindReason{errkind.Transient, errkind.ReasonRetriesExhausted},
		"Throttling", "ThrottlingException", "ThrottledException", "RequestThrottled", "RequestThrottledException",
		"RequestLimitExceeded", "TooManyRequestsException", "SlowDown", "EC2ThrottledException",
		"ServiceUnavailable", "ServiceUnavailableException", "Unavailable", "InternalError", "InternalFailure",
		"InternalServiceError", "InternalServerError", "InternalException", "RequestTimeout", "RequestTimeoutException",
		"PriorRequestNotComplete")
	add(kindReason{errkind.InvalidInput, errkind.ReasonRegionUnavailable},
		"RegionDisabledException", "OptInRequired")
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
	switch {
	case errors.Is(callCtx.Err(), context.DeadlineExceeded):
		e.Kind, e.Reason = errkind.Transient, errkind.ReasonTimeout
	case errors.As(err, &apiErr) && errorCodes[apiErr.ErrorCode()].kind != errkind.Internal:
		kr := errorCodes[apiErr.ErrorCode()]
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
