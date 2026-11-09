#!/usr/bin/env node
import { App, DefaultStackSynthesizer } from 'aws-cdk-lib';
import { SkeletonStack } from '../lib/stack';

const app = new App();
new SkeletonStack(app, 'tag-refactor-cdk-skeleton', {
  stackName: 'tag-refactor-cdk-skeleton',
  synthesizer: new DefaultStackSynthesizer({ generateBootstrapVersionRule: false }),
});
