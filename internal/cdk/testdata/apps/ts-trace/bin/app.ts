#!/usr/bin/env node
import { App, DefaultStackSynthesizer } from 'aws-cdk-lib';
import { TraceStack } from '../lib/stack';

const app = new App();
new TraceStack(app, 'tag-refactor-cdk-trace', {
  stackName: 'tag-refactor-cdk-trace',
  synthesizer: new DefaultStackSynthesizer({ generateBootstrapVersionRule: false }),
});
