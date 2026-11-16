import { RemovalPolicy, Stack, StackProps, Tags } from 'aws-cdk-lib';
import * as s3 from 'aws-cdk-lib/aws-s3';
import { Construct } from 'constructs';

// makeBucket makes the bucket outside the class of the stack: the helper does
// not follow the call, so the place of the bucket comes from the trace of synth.
function makeBucket(scope: Construct): s3.Bucket {
  return new s3.Bucket(scope, 'Logs', {
    blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
    encryption: s3.BucketEncryption.S3_MANAGED,
    removalPolicy: RemovalPolicy.DESTROY,
  });
}

export class TraceStack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    makeBucket(this);
    Tags.of(this).add('Owner', 'team-a');
  }
}
