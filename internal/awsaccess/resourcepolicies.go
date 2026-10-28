package awsaccess

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/smithy-go"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Reading the resource policies of S3 buckets, KMS keys, SQS queues, SNS
// topics and Secrets Manager secrets (U5 BR1.3, security design §1). Every
// call goes through invoke, so the time limits, the retries and the --debug
// lines apply. Nothing here reads a secret's value.

// Page sizes of the lists, the largest each API accepts (U5
// logical-components.md). ListTopics has no page size: SNS gives 100.
const (
	bucketPageSize = 10000
	keyPageSize    = 1000
	queuePageSize  = 1000
	secretPageSize = 100
)

// defaultKeyPolicy is the only name a KMS key policy can have.
const defaultKeyPolicy = "default"

// regionClients are the clients of the resource policy services in one
// region.
type regionClients struct {
	s3  *s3.Client
	kms *kms.Client
	sqs *sqs.Client
	sns *sns.Client
	sm  *secretsmanager.Client
}

// clientsIn returns the clients of region, creating them on first use. The
// region names come from the plan, whose regions were checked when the
// account was resolved.
func (s *Session) clientsIn(region string) *regionClients {
	s.regionalMu.Lock()
	defer s.regionalMu.Unlock()
	if c, ok := s.regional[region]; ok {
		return c
	}
	if s.regional == nil {
		s.regional = map[string]*regionClients{}
	}
	c := &regionClients{
		s3:  s3.NewFromConfig(s.cfg, func(o *s3.Options) { o.Region = region }),
		kms: kms.NewFromConfig(s.cfg, func(o *kms.Options) { o.Region = region }),
		sqs: sqs.NewFromConfig(s.cfg, func(o *sqs.Options) { o.Region = region }),
		sns: sns.NewFromConfig(s.cfg, func(o *sns.Options) { o.Region = region }),
		sm:  secretsmanager.NewFromConfig(s.cfg, func(o *secretsmanager.Options) { o.Region = region }),
	}
	s.regional[region] = c
	return c
}

// readPages reads every page of a list that is continued with a token. read
// asks for the page after token ("" for the first one) and returns the token
// of the next page, "" after the last one.
func readPages(op, region string, read func(token string) (string, error)) error {
	seen := map[string]bool{}
	token := ""
	for {
		next, err := read(token)
		if err != nil {
			return err
		}
		if next == "" {
			return nil
		}
		if seen[next] {
			return &errkind.Error{Kind: errkind.Internal, Op: op, Region: region, Detail: "AWS repeated a pagination token"}
		}
		seen[next] = true
		token = next
	}
}

// tokenParam is the token of a request: nil on the first page.
func tokenParam(token string) *string {
	if token == "" {
		return nil
	}
	return aws.String(token)
}

// partitionOf returns the ARN partition of a region.
func partitionOf(region string) string {
	switch {
	case strings.HasPrefix(region, "cn-"):
		return "aws-cn"
	case strings.HasPrefix(region, "us-gov-"):
		return "aws-us-gov"
	}
	return "aws"
}

// --- S3 ---

// ListBuckets reads the buckets of every region (s3:ListAllMyBuckets). The
// answer gives each bucket's region; a bucket without one is left for
// GetBucketRegion.
func (s *Session) ListBuckets(ctx context.Context) ([]Bucket, error) {
	client := s.clientsIn(s.region).s3
	var buckets []Bucket
	err := readPages(OpListBuckets, s.region, func(token string) (string, error) {
		var out *s3.ListBucketsOutput
		err := s.invoke(ctx, OpListBuckets, s.region, func(ctx context.Context, add apiOption) error {
			var err error
			out, err = client.ListBuckets(ctx, &s3.ListBucketsInput{MaxBuckets: aws.Int32(bucketPageSize), ContinuationToken: tokenParam(token)},
				func(o *s3.Options) { o.APIOptions = append(o.APIOptions, add) })
			return err
		})
		if err != nil {
			return "", err
		}
		for _, b := range out.Buckets {
			buckets = append(buckets, Bucket{Name: aws.ToString(b.Name), Region: aws.ToString(b.BucketRegion)})
		}
		return aws.ToString(out.ContinuationToken), nil
	})
	return buckets, err
}

// GetBucketRegion reads the region of a bucket (s3:GetBucketLocation). S3
// answers an empty location for us-east-1 and EU for eu-west-1.
func (s *Session) GetBucketRegion(ctx context.Context, bucket string) (string, error) {
	client := s.clientsIn(s.region).s3
	var out *s3.GetBucketLocationOutput
	err := s.invoke(ctx, OpGetBucketLocation, s.region, func(ctx context.Context, add apiOption) error {
		var err error
		out, err = client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(bucket)},
			func(o *s3.Options) { o.APIOptions = append(o.APIOptions, add) })
		return err
	})
	if err != nil {
		return "", err
	}
	switch location := string(out.LocationConstraint); location {
	case "":
		return "us-east-1", nil
	case "EU":
		return "eu-west-1", nil
	default:
		return location, nil
	}
}

// GetBucketPolicy reads the bucket policy in the bucket's region. A bucket
// without a policy (NoSuchBucketPolicy) is not an error: Found is false.
func (s *Session) GetBucketPolicy(ctx context.Context, region, bucket string) (ResourcePolicy, error) {
	client := s.clientsIn(region).s3
	policy := ResourcePolicy{ARN: "arn:" + partitionOf(region) + ":s3:::" + bucket}
	err := s.invoke(ctx, OpGetBucketPolicy, region, func(ctx context.Context, add apiOption) error {
		out, err := client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)},
			func(o *s3.Options) { o.APIOptions = append(o.APIOptions, add) })
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchBucketPolicy" {
			return nil
		}
		if err != nil {
			return err
		}
		policy.Document, policy.Found = aws.ToString(out.Policy), out.Policy != nil
		return nil
	})
	return policy, err
}

// --- KMS ---

// ListKeys reads the KMS keys of the region.
func (s *Session) ListKeys(ctx context.Context, region string) ([]KMSKey, error) {
	client := s.clientsIn(region).kms
	var keys []KMSKey
	err := readPages(OpListKeys, region, func(token string) (string, error) {
		var out *kms.ListKeysOutput
		err := s.invoke(ctx, OpListKeys, region, func(ctx context.Context, add apiOption) error {
			var err error
			out, err = client.ListKeys(ctx, &kms.ListKeysInput{Limit: aws.Int32(keyPageSize), Marker: tokenParam(token)},
				func(o *kms.Options) { o.APIOptions = append(o.APIOptions, add) })
			return err
		})
		if err != nil {
			return "", err
		}
		for _, k := range out.Keys {
			keys = append(keys, KMSKey{ID: aws.ToString(k.KeyId), ARN: aws.ToString(k.KeyArn)})
		}
		if !out.Truncated {
			return "", nil
		}
		if aws.ToString(out.NextMarker) == "" {
			return "", &errkind.Error{Kind: errkind.Internal, Op: OpListKeys, Region: region, Detail: "KMS returned a truncated list without a marker"}
		}
		return aws.ToString(out.NextMarker), nil
	})
	return keys, err
}

// keyID is what the KMS calls name a key by: its ARN when known.
func keyID(key KMSKey) *string {
	if key.ARN != "" {
		return aws.String(key.ARN)
	}
	return aws.String(key.ID)
}

// IsCustomerManagedKey reads whether the account created the key
// (kms:DescribeKey, KeyManager CUSTOMER). AWS managed keys are not read (U5
// BR1.4).
func (s *Session) IsCustomerManagedKey(ctx context.Context, region string, key KMSKey) (bool, error) {
	client := s.clientsIn(region).kms
	var out *kms.DescribeKeyOutput
	err := s.invoke(ctx, OpDescribeKey, region, func(ctx context.Context, add apiOption) error {
		var err error
		out, err = client.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: keyID(key)},
			func(o *kms.Options) { o.APIOptions = append(o.APIOptions, add) })
		return err
	})
	if err != nil {
		return false, err
	}
	if out.KeyMetadata == nil {
		return false, &errkind.Error{Kind: errkind.Internal, Op: OpDescribeKey, Region: region, Detail: "KMS returned no key metadata"}
	}
	return out.KeyMetadata.KeyManager == kmstypes.KeyManagerTypeCustomer, nil
}

// GetKeyPolicy reads the key policy (its name is always default). Every key
// has one.
func (s *Session) GetKeyPolicy(ctx context.Context, region string, key KMSKey) (ResourcePolicy, error) {
	client := s.clientsIn(region).kms
	policy := ResourcePolicy{ARN: key.ARN}
	err := s.invoke(ctx, OpGetKeyPolicy, region, func(ctx context.Context, add apiOption) error {
		out, err := client.GetKeyPolicy(ctx, &kms.GetKeyPolicyInput{KeyId: keyID(key), PolicyName: aws.String(defaultKeyPolicy)},
			func(o *kms.Options) { o.APIOptions = append(o.APIOptions, add) })
		if err != nil {
			return err
		}
		policy.Document, policy.Found = aws.ToString(out.Policy), out.Policy != nil
		return nil
	})
	return policy, err
}

// --- SQS ---

// ListQueues reads the URLs of the queues of the region.
func (s *Session) ListQueues(ctx context.Context, region string) ([]string, error) {
	client := s.clientsIn(region).sqs
	var urls []string
	err := readPages(OpListQueues, region, func(token string) (string, error) {
		var out *sqs.ListQueuesOutput
		err := s.invoke(ctx, OpListQueues, region, func(ctx context.Context, add apiOption) error {
			var err error
			out, err = client.ListQueues(ctx, &sqs.ListQueuesInput{MaxResults: aws.Int32(queuePageSize), NextToken: tokenParam(token)},
				func(o *sqs.Options) { o.APIOptions = append(o.APIOptions, add) })
			return err
		})
		if err != nil {
			return "", err
		}
		urls = append(urls, out.QueueUrls...)
		return aws.ToString(out.NextToken), nil
	})
	return urls, err
}

// GetQueuePolicy reads the Policy and QueueArn attributes of a queue; a queue
// without the Policy attribute has no policy.
func (s *Session) GetQueuePolicy(ctx context.Context, region, queueURL string) (ResourcePolicy, error) {
	client := s.clientsIn(region).sqs
	var policy ResourcePolicy
	err := s.invoke(ctx, OpGetQueueAttributes, region, func(ctx context.Context, add apiOption) error {
		out, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       aws.String(queueURL),
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNamePolicy, sqstypes.QueueAttributeNameQueueArn},
		}, func(o *sqs.Options) { o.APIOptions = append(o.APIOptions, add) })
		if err != nil {
			return err
		}
		policy.ARN = out.Attributes[string(sqstypes.QueueAttributeNameQueueArn)]
		policy.Document, policy.Found = out.Attributes[string(sqstypes.QueueAttributeNamePolicy)]
		return nil
	})
	if err == nil && policy.ARN == "" {
		err = &errkind.Error{Kind: errkind.Internal, Op: OpGetQueueAttributes, Region: region, Detail: "SQS returned no QueueArn"}
	}
	return policy, err
}

// --- SNS ---

// ListTopics reads the ARNs of the topics of the region (100 per page).
func (s *Session) ListTopics(ctx context.Context, region string) ([]string, error) {
	client := s.clientsIn(region).sns
	var arns []string
	err := readPages(OpListTopics, region, func(token string) (string, error) {
		var out *sns.ListTopicsOutput
		err := s.invoke(ctx, OpListTopics, region, func(ctx context.Context, add apiOption) error {
			var err error
			out, err = client.ListTopics(ctx, &sns.ListTopicsInput{NextToken: tokenParam(token)},
				func(o *sns.Options) { o.APIOptions = append(o.APIOptions, add) })
			return err
		})
		if err != nil {
			return "", err
		}
		for _, t := range out.Topics {
			arns = append(arns, aws.ToString(t.TopicArn))
		}
		return aws.ToString(out.NextToken), nil
	})
	return arns, err
}

// GetTopicPolicy reads the Policy attribute of a topic.
func (s *Session) GetTopicPolicy(ctx context.Context, region, topicARN string) (ResourcePolicy, error) {
	client := s.clientsIn(region).sns
	policy := ResourcePolicy{ARN: topicARN}
	err := s.invoke(ctx, OpGetTopicAttributes, region, func(ctx context.Context, add apiOption) error {
		out, err := client.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{TopicArn: aws.String(topicARN)},
			func(o *sns.Options) { o.APIOptions = append(o.APIOptions, add) })
		if err != nil {
			return err
		}
		policy.Document, policy.Found = out.Attributes["Policy"]
		return nil
	})
	return policy, err
}

// --- Secrets Manager ---

// ListSecrets reads the ARNs of the secrets of the region. It never reads a
// secret's value.
func (s *Session) ListSecrets(ctx context.Context, region string) ([]string, error) {
	client := s.clientsIn(region).sm
	var arns []string
	err := readPages(OpListSecrets, region, func(token string) (string, error) {
		var out *secretsmanager.ListSecretsOutput
		err := s.invoke(ctx, OpListSecrets, region, func(ctx context.Context, add apiOption) error {
			var err error
			out, err = client.ListSecrets(ctx, &secretsmanager.ListSecretsInput{MaxResults: aws.Int32(secretPageSize), NextToken: tokenParam(token)},
				func(o *secretsmanager.Options) { o.APIOptions = append(o.APIOptions, add) })
			return err
		})
		if err != nil {
			return "", err
		}
		for _, sec := range out.SecretList {
			arns = append(arns, aws.ToString(sec.ARN))
		}
		return aws.ToString(out.NextToken), nil
	})
	return arns, err
}

// GetSecretPolicy reads the resource policy of a secret
// (secretsmanager:GetResourcePolicy), never its value.
func (s *Session) GetSecretPolicy(ctx context.Context, region, secretARN string) (ResourcePolicy, error) {
	client := s.clientsIn(region).sm
	policy := ResourcePolicy{ARN: secretARN}
	err := s.invoke(ctx, OpGetResourcePolicy, region, func(ctx context.Context, add apiOption) error {
		out, err := client.GetResourcePolicy(ctx, &secretsmanager.GetResourcePolicyInput{SecretId: aws.String(secretARN)},
			func(o *secretsmanager.Options) { o.APIOptions = append(o.APIOptions, add) })
		if err != nil {
			return err
		}
		policy.Document, policy.Found = aws.ToString(out.ResourcePolicy), out.ResourcePolicy != nil
		return nil
	})
	return policy, err
}
