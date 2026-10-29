package awsaccess

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	rgt "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	rgttypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// The largest number of resources of one call of the tagging API (U6
// tech-stack-decisions.md): TagResources and UntagResources write at most
// 20, GetResources reads at most 100 ARNs.
const (
	MaxWriteARNs = 20
	MaxReadARNs  = 100
)

// GetTags reads the tags of the resources with one tag:GetResources call that
// names their ARNs (U6 BR3.3). The API does not page such a call. A resource
// the answer leaves out is not in the map.
func (s *Session) GetTags(ctx context.Context, region string, arns []string) (map[string]map[string]string, error) {
	if len(arns) == 0 || len(arns) > MaxReadARNs {
		return nil, &errkind.Error{Kind: errkind.Internal, Op: OpGetResources, Region: region,
			Detail: fmt.Sprintf("%d ARNs asked for in one read", len(arns))}
	}
	client := rgt.NewFromConfig(s.cfg, func(o *rgt.Options) { o.Region = region })
	var out *rgt.GetResourcesOutput
	err := s.invoke(ctx, OpGetResources, region, func(ctx context.Context, add apiOption) error {
		var err error
		out, err = client.GetResources(ctx, &rgt.GetResourcesInput{ResourceARNList: arns},
			func(o *rgt.Options) { o.APIOptions = append(o.APIOptions, add) })
		return err
	})
	if err != nil {
		return nil, err
	}
	tags := make(map[string]map[string]string, len(out.ResourceTagMappingList))
	for _, m := range out.ResourceTagMappingList {
		t := make(map[string]string, len(m.Tags))
		for _, tag := range m.Tags {
			t[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
		}
		tags[aws.ToString(m.ResourceARN)] = t
	}
	return tags, nil
}

// Writer writes tags and run records (contract C6 TagWriter and
// ObjectStore). It is built from a Session by apply and rollback only; the
// readers scan, rules suggest and plan use are Session interfaces, which
// have no write method (U6 NFR3.10). Its methods may be called from several
// goroutines.
type Writer struct {
	s *Session
	// bucketRegions caches the region of each record bucket.
	mu            sync.Mutex
	bucketRegions map[string]string
}

var (
	_ TagWriter    = (*Writer)(nil)
	_ ObjectStore  = (*Writer)(nil)
	_ ARNTagReader = (*Session)(nil)
)

// NewWriter returns the writer of the session.
func NewWriter(s *Session) *Writer {
	return &Writer{s: s, bucketRegions: map[string]string{}}
}

// TagResources adds or replaces tags on at most MaxWriteARNs resources of a
// region (U6 BR4.1).
func (w *Writer) TagResources(ctx context.Context, region string, arns []string, tags map[string]string) (WriteResult, error) {
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	if err := checkWrite(OpTagResources, region, arns, keys); err != nil {
		return WriteResult{}, err
	}
	client := rgt.NewFromConfig(w.s.cfg, func(o *rgt.Options) { o.Region = region })
	return w.write(ctx, OpTagResources, region, arns, func(ctx context.Context, add apiOption, arns []string) (map[string]rgttypes.FailureInfo, error) {
		out, err := client.TagResources(ctx, &rgt.TagResourcesInput{ResourceARNList: arns, Tags: tags},
			func(o *rgt.Options) { o.APIOptions = append(o.APIOptions, add) })
		if err != nil {
			return nil, err
		}
		return out.FailedResourcesMap, nil
	})
}

// UntagResources removes keys from at most MaxWriteARNs resources of a
// region (U6 BR4.1).
func (w *Writer) UntagResources(ctx context.Context, region string, arns []string, keys []string) (WriteResult, error) {
	if err := checkWrite(OpUntagResources, region, arns, keys); err != nil {
		return WriteResult{}, err
	}
	client := rgt.NewFromConfig(w.s.cfg, func(o *rgt.Options) { o.Region = region })
	return w.write(ctx, OpUntagResources, region, arns, func(ctx context.Context, add apiOption, arns []string) (map[string]rgttypes.FailureInfo, error) {
		out, err := client.UntagResources(ctx, &rgt.UntagResourcesInput{ResourceARNList: arns, TagKeys: keys},
			func(o *rgt.Options) { o.APIOptions = append(o.APIOptions, add) })
		if err != nil {
			return nil, err
		}
		return out.FailedResourcesMap, nil
	})
}

// checkWrite refuses a write that AWS would refuse or that names a key
// starting with aws:, in any case, before anything is sent (U6 NFR3.12, the
// second guard after the applier's own exclusion).
func checkWrite(op, region string, arns, keys []string) error {
	if len(arns) == 0 || len(arns) > MaxWriteARNs || len(keys) == 0 {
		return &errkind.Error{Kind: errkind.InvalidInput, Op: op, Region: region,
			Detail: fmt.Sprintf("a write needs 1 to %d resources and at least one key", MaxWriteARNs)}
	}
	for _, k := range keys {
		if IsAWSKey(k) {
			return &errkind.Error{Kind: errkind.InvalidInput, Op: op, Region: region,
				Detail: "tag keys starting with aws: are never written or removed"}
		}
	}
	return nil
}

// writeCall makes one TagResources or UntagResources call for arns.
type writeCall func(ctx context.Context, add apiOption, arns []string) (map[string]rgttypes.FailureInfo, error)

// write runs a write and writes again, after the back-off of the SDK's
// retries, only the resources that failed with a transient error, up to
// MaxAttempts calls in all (U6 NFR5.12). An error of the first call is the
// error of the write. An error of a later call becomes the error of the
// resources it was for.
func (w *Writer) write(ctx context.Context, op, region string, arns []string, call writeCall) (WriteResult, error) {
	res := WriteResult{Failed: map[string]*errkind.Error{}}
	pending := arns
	for attempt := 1; ; attempt++ {
		var failed map[string]rgttypes.FailureInfo
		err := w.s.invoke(ctx, op, region, func(ctx context.Context, add apiOption) error {
			var err error
			failed, err = call(ctx, add, pending)
			return err
		})
		if err != nil {
			if attempt == 1 {
				return WriteResult{}, err
			}
			for _, arn := range pending {
				res.Failed[arn] = errkind.Ensure(err, op)
			}
			return res, nil
		}
		var again []string
		for _, arn := range pending {
			f, ok := failed[arn]
			if !ok {
				continue
			}
			e := classifyFailure(op, region, arn, f)
			if e.Kind == errkind.Transient && attempt < MaxAttempts {
				again = append(again, arn)
				continue
			}
			res.Failed[arn] = e
		}
		if len(again) == 0 {
			return res, nil
		}
		if err := w.s.wait(ctx, attempt); err != nil {
			for _, arn := range again {
				res.Failed[arn] = &errkind.Error{Kind: errkind.Internal, Op: op, Region: region, Detail: "the call was cancelled", Cause: err}
			}
			return res, nil
		}
		pending = again
	}
}

// wait waits the back-off before attempt+1, or until ctx ends.
func (s *Session) wait(ctx context.Context, attempt int) error {
	var d time.Duration
	if s.opts.RetryDelay != nil {
		d = s.opts.RetryDelay(attempt)
	} else {
		d, _ = retry.NewExponentialJitterBackoff(MaxBackoff).BackoffDelay(attempt, nil)
	}
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// failureCodes maps the codes of the per-resource failures that errorCodes
// does not hold: the tagging API's own and the services' NotFound codes. The
// HTTP status decides first; the code is only the fallback (U6 security
// design §5).
var failureCodes = map[string]errkind.Kind{
	"InternalServiceException":  errkind.Transient,
	"InvalidParameterException": errkind.InvalidInput,
	"ValidationException":       errkind.InvalidInput,
	"NoSuchEntity":              errkind.NotFound,
	"ResourceNotFoundException": errkind.NotFound,
	"NotFound":                  errkind.NotFound,
	"NoSuchBucket":              errkind.NotFound,
}

// classifyFailure turns one entry of FailedResourcesMap into an error kind
// (U6 security design §5): 429 and 5xx are Transient, 403 AccessDenied with
// the tagging action of the resource's service, 404 NotFound, 400
// InvalidInput; without a known status the code decides, else Internal. The
// message of the entry is never kept.
func classifyFailure(op, region, arn string, f rgttypes.FailureInfo) *errkind.Error {
	e := &errkind.Error{Kind: errkind.Internal, Op: op, Region: region}
	status := int(f.StatusCode)
	switch {
	case status == http.StatusTooManyRequests || status >= 500:
		e.Kind, e.Reason = errkind.Transient, errkind.ReasonRetriesExhausted
	case status == http.StatusForbidden:
		e.Kind = errkind.AccessDenied
	case status == http.StatusNotFound:
		e.Kind = errkind.NotFound
	case status == http.StatusBadRequest:
		e.Kind = errkind.InvalidInput
	default:
		code := string(f.ErrorCode)
		if kind, ok := failureCodes[code]; ok {
			e.Kind = kind
		} else if kr, ok := errorCodes[code]; ok {
			e.Kind, e.Reason = kr.kind, kr.reason
		}
		if e.Kind == errkind.Transient && e.Reason == errkind.ReasonNone {
			e.Reason = errkind.ReasonRetriesExhausted
		}
	}
	if e.Kind == errkind.AccessDenied {
		e.Op = tagActionFor(op, arn)
	}
	return e
}
