package awsaccess

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	cfgtypes "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	re "github.com/aws/aws-sdk-go-v2/service/resourceexplorer2"
	retypes "github.com/aws/aws-sdk-go-v2/service/resourceexplorer2/types"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Reading AWS Config and Resource Explorer to find never-tagged resources
// (BR3.1, performance design §1 and §2).

const (
	// configPageSize is the Limit of SelectResourceConfig, its maximum.
	configPageSize = 100
	// indexPageSize is the MaxResults of ListIndexes, its maximum.
	indexPageSize = 100
	// DefaultExplorerPageSize is the MaxResults of ListResources. It stays
	// well below 1000: with 1000, AWS returns no NextToken and the list
	// stops at 1000 resources (NFR1.10).
	DefaultExplorerPageSize = 500
	// maxConfigResultSize bounds one result of the Config query (security
	// design §3).
	maxConfigResultSize = 256 << 10
	// configQuery reads every recorded resource with its tags. AWS Config
	// cannot select on the content of tags or on NULL, so the resources
	// without user tags are chosen by the caller (performance design §2).
	configQuery = "SELECT arn, resourceType, resourceId, awsRegion, tags"
	// untaggedFilter makes Resource Explorer return resources without tags.
	untaggedFilter = "tag:none"
)

// ConfigRecorders reads the configuration recorders of the region and, when
// there are any, their status (DescribeConfigurationRecorders and
// DescribeConfigurationRecorderStatus).
func (s *Session) ConfigRecorders(ctx context.Context, region string) ([]ConfigRecorder, error) {
	client := configservice.NewFromConfig(s.cfg, func(o *configservice.Options) { o.Region = region })
	var out *configservice.DescribeConfigurationRecordersOutput
	err := s.invoke(ctx, OpDescribeConfigurationRecorders, region, func(ctx context.Context, add apiOption) error {
		var err error
		out, err = client.DescribeConfigurationRecorders(ctx, &configservice.DescribeConfigurationRecordersInput{},
			func(o *configservice.Options) { o.APIOptions = append(o.APIOptions, add) })
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(out.ConfigurationRecorders) == 0 {
		return nil, nil
	}
	var status *configservice.DescribeConfigurationRecorderStatusOutput
	err = s.invoke(ctx, OpDescribeConfigurationRecorderStatus, region, func(ctx context.Context, add apiOption) error {
		var err error
		status, err = client.DescribeConfigurationRecorderStatus(ctx, &configservice.DescribeConfigurationRecorderStatusInput{},
			func(o *configservice.Options) { o.APIOptions = append(o.APIOptions, add) })
		return err
	})
	if err != nil {
		return nil, err
	}
	recording := map[string]bool{}
	for _, st := range status.ConfigurationRecordersStatus {
		recording[aws.ToString(st.Name)] = st.Recording
	}
	recorders := make([]ConfigRecorder, 0, len(out.ConfigurationRecorders))
	for _, r := range out.ConfigurationRecorders {
		recorders = append(recorders, recorderOf(r, recording[aws.ToString(r.Name)]))
	}
	return recorders, nil
}

func recorderOf(r cfgtypes.ConfigurationRecorder, recording bool) ConfigRecorder {
	rec := ConfigRecorder{Name: aws.ToString(r.Name), Recording: recording}
	if g := r.RecordingGroup; g != nil {
		rec.AllSupported = g.AllSupported
		rec.IncludedTypes = len(g.ResourceTypes)
		if g.RecordingStrategy != nil {
			rec.Strategy = string(g.RecordingStrategy.UseOnly)
		}
		if g.ExclusionByResourceTypes != nil {
			rec.ExcludedTypes = len(g.ExclusionByResourceTypes.ResourceTypes)
		}
	}
	return rec
}

// configResult is one result of the Config query, a JSON document.
type configResult struct {
	ARN          string `json:"arn"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	Region       string `json:"awsRegion"`
	Tags         []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"tags"`
}

// ConfigResources reads every resource AWS Config records in the region with
// SelectResourceConfig, 100 per page, until no NextToken is left. Each result
// is checked on its own: one that is not JSON, has no resourceType or is
// larger than 256 KiB makes the whole answer unexpected (security design §3),
// so the caller moves on to Resource Explorer.
func (s *Session) ConfigResources(ctx context.Context, region string) ([]ConfigResource, error) {
	client := configservice.NewFromConfig(s.cfg, func(o *configservice.Options) { o.Region = region })
	var resources []ConfigResource
	seen := map[string]bool{}
	token := ""
	for {
		in := &configservice.SelectResourceConfigInput{Expression: aws.String(configQuery), Limit: configPageSize}
		if token != "" {
			in.NextToken = aws.String(token)
		}
		var out *configservice.SelectResourceConfigOutput
		err := s.invoke(ctx, OpSelectResourceConfig, region, func(ctx context.Context, add apiOption) error {
			var err error
			out, err = client.SelectResourceConfig(ctx, in, func(o *configservice.Options) { o.APIOptions = append(o.APIOptions, add) })
			return err
		})
		if err != nil {
			return nil, err
		}
		for _, raw := range out.Results {
			r, err := parseConfigResult(raw)
			if err != nil {
				return nil, &errkind.Error{Kind: errkind.Internal, Op: OpSelectResourceConfig, Region: region, Detail: err.Error()}
			}
			resources = append(resources, r)
		}
		token = aws.ToString(out.NextToken)
		if token == "" {
			return resources, nil
		}
		if seen[token] {
			return nil, &errkind.Error{Kind: errkind.Internal, Op: OpSelectResourceConfig, Region: region,
				Detail: "AWS Config repeated a pagination token"}
		}
		seen[token] = true
	}
}

// parseConfigResult reads one result. Its error text is written by the tool
// and never quotes the result.
func parseConfigResult(raw string) (ConfigResource, error) {
	if len(raw) > maxConfigResultSize {
		return ConfigResource{}, errors.New("AWS Config returned a result larger than 256 KiB")
	}
	var r configResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return ConfigResource{}, errors.New("AWS Config returned a result that is not JSON")
	}
	if r.ResourceType == "" {
		return ConfigResource{}, errors.New("AWS Config returned a result without a resource type")
	}
	res := ConfigResource{ResourceType: r.ResourceType, ResourceID: r.ResourceID, Region: r.Region, Tags: make(map[string]string, len(r.Tags))}
	if strings.HasPrefix(r.ARN, "arn:") {
		res.ARN = r.ARN
	}
	for _, t := range r.Tags {
		res.Tags[t.Key] = t.Value
	}
	return res, nil
}

func (s *Session) explorerClient(region string) *re.Client {
	return re.NewFromConfig(s.cfg, func(o *re.Options) { o.Region = region })
}

// ExplorerIndexes reads the Resource Explorer indexes of every region from
// the session's region (ListIndexes), until no NextToken is left.
func (s *Session) ExplorerIndexes(ctx context.Context) ([]ExplorerIndex, error) {
	client := s.explorerClient(s.region)
	var indexes []ExplorerIndex
	seen := map[string]bool{}
	token := ""
	for {
		in := &re.ListIndexesInput{MaxResults: aws.Int32(indexPageSize)}
		if token != "" {
			in.NextToken = aws.String(token)
		}
		var out *re.ListIndexesOutput
		err := s.invoke(ctx, OpListIndexes, s.region, func(ctx context.Context, add apiOption) error {
			var err error
			out, err = client.ListIndexes(ctx, in, func(o *re.Options) { o.APIOptions = append(o.APIOptions, add) })
			return err
		})
		if err != nil {
			return nil, err
		}
		for _, ix := range out.Indexes {
			indexes = append(indexes, ExplorerIndex{Region: aws.ToString(ix.Region), Aggregator: ix.Type == retypes.IndexTypeAggregator})
		}
		token = aws.ToString(out.NextToken)
		if token == "" {
			return indexes, nil
		}
		if seen[token] {
			return nil, &errkind.Error{Kind: errkind.Internal, Op: OpListIndexes, Region: s.region,
				Detail: "Resource Explorer repeated a pagination token"}
		}
		seen[token] = true
	}
}

// DefaultView reads the default view of the region (GetDefaultView) and its
// filter (GetView). The view ARN comes from AWS, never from the user. A
// region without a default view (an empty ViewArn, or ResourceNotFound) is
// an AccessDenied with the reason no-default-view, and GetView is not called
// (security design §4).
func (s *Session) DefaultView(ctx context.Context, region string) (ExplorerView, error) {
	client := s.explorerClient(region)
	var out *re.GetDefaultViewOutput
	err := s.invoke(ctx, OpGetDefaultView, region, func(ctx context.Context, add apiOption) error {
		var err error
		out, err = client.GetDefaultView(ctx, &re.GetDefaultViewInput{}, func(o *re.Options) { o.APIOptions = append(o.APIOptions, add) })
		return err
	})
	noView := &errkind.Error{Kind: errkind.AccessDenied, Reason: errkind.ReasonNoDefaultView, Op: OpGetDefaultView, Region: region}
	if err != nil {
		var notFound *retypes.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return ExplorerView{}, noView
		}
		return ExplorerView{}, err
	}
	arn := aws.ToString(out.ViewArn)
	if arn == "" {
		return ExplorerView{}, noView
	}
	var view *re.GetViewOutput
	err = s.invoke(ctx, OpGetView, region, func(ctx context.Context, add apiOption) error {
		var err error
		view, err = client.GetView(ctx, &re.GetViewInput{ViewArn: aws.String(arn)}, func(o *re.Options) { o.APIOptions = append(o.APIOptions, add) })
		return err
	})
	if err != nil {
		return ExplorerView{}, err
	}
	result := ExplorerView{ARN: arn}
	if view.View != nil && view.View.Filters != nil {
		result.Filter = aws.ToString(view.View.Filters.FilterString)
	}
	return result, nil
}

// UntaggedResources reads the resources of the view that have no tags, with
// ListResources and the filter tag:none, until no NextToken is left. The API
// Search is never used: it stops at 1,000 results. A 401 means the view
// cannot be used (no permission for resource-explorer-2:Search, or the view
// is not usable) and is returned as AccessDenied.
func (s *Session) UntaggedResources(ctx context.Context, region, viewARN string) ([]ExplorerResource, error) {
	client := s.explorerClient(region)
	var resources []ExplorerResource
	seen := map[string]bool{}
	token := ""
	for {
		in := &re.ListResourcesInput{
			Filters:    &retypes.SearchFilter{FilterString: aws.String(untaggedFilter)},
			MaxResults: aws.Int32(int32(s.opts.ExplorerPageSize)),
			ViewArn:    aws.String(viewARN),
		}
		if token != "" {
			in.NextToken = aws.String(token)
		}
		var out *re.ListResourcesOutput
		err := s.invoke(ctx, OpListResources, region, func(ctx context.Context, add apiOption) error {
			var err error
			out, err = client.ListResources(ctx, in, func(o *re.Options) { o.APIOptions = append(o.APIOptions, add) })
			return err
		})
		if err != nil {
			return nil, err
		}
		for _, r := range out.Resources {
			resources = append(resources, ExplorerResource{
				ARN: aws.ToString(r.Arn), ResourceType: aws.ToString(r.ResourceType), Region: aws.ToString(r.Region),
			})
		}
		token = aws.ToString(out.NextToken)
		if token == "" {
			return resources, nil
		}
		if seen[token] {
			return nil, &errkind.Error{Kind: errkind.Internal, Op: OpListResources, Region: region,
				Detail: "Resource Explorer repeated a pagination token"}
		}
		seen[token] = true
	}
}
