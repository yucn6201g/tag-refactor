import { Stack, StackProps, Tags } from 'aws-cdk-lib';
import * as sqs from 'aws-cdk-lib/aws-sqs';
import * as ssm from 'aws-cdk-lib/aws-ssm';
import { Construct } from 'constructs';

// LookupStack looks up a parameter of SSM at synth: without cdk.context.json,
// synth needs AWS.
export class LookupStack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    const name = ssm.StringParameter.valueFromLookup(this, '/tag-refactor/queue-name');
    const queue = new sqs.Queue(this, 'Queue', { queueName: name });
    Tags.of(queue).add('Env', 'dev');
  }
}
