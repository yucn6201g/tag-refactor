#!/usr/bin/env node
import { App, DefaultStackSynthesizer } from 'aws-cdk-lib';
import { LookupStack } from '../lib/stack';

const app = new App();
// The account is made up: synth in the tests never reaches AWS.
new LookupStack(app, 'tag-refactor-cdk-lookup', {
  stackName: 'tag-refactor-cdk-lookup',
  env: { account: '000000000000', region: 'ap-northeast-1' },
  synthesizer: new DefaultStackSynthesizer({ generateBootstrapVersionRule: false }),
});
