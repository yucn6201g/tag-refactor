#!/usr/bin/env node
import { App, DefaultStackSynthesizer, Stack, Tags } from 'aws-cdk-lib';
import * as sqs from 'aws-cdk-lib/aws-sqs';
import * as ssm from 'aws-cdk-lib/aws-ssm';

// cdk.json names a profile. A profile alone does not stop synth (the CDK CLI
// goes on without the account when the profile is not found), so the stack
// also looks up a parameter, which needs the credentials of that profile. The
// account is made up: synth in the tests never reaches AWS.
const app = new App();
const stack = new Stack(app, 'tag-refactor-cdk-profile', {
  stackName: 'tag-refactor-cdk-profile',
  env: { account: '000000000000', region: 'ap-northeast-1' },
  synthesizer: new DefaultStackSynthesizer({ generateBootstrapVersionRule: false }),
});
const name = ssm.StringParameter.valueFromLookup(stack, '/tag-refactor/profile-queue-name');
const queue = new sqs.Queue(stack, 'Queue', { queueName: name });
Tags.of(queue).add('Env', 'dev');
