#!/usr/bin/env node
import * as cdk from '@aws-cdk/core';
import * as s3 from '@aws-cdk/aws-s3';

const app = new cdk.App();
const stack = new cdk.Stack(app, 'tag-refactor-cdk-v1');
const bucket = new s3.Bucket(stack, 'Bucket');
cdk.Tags.of(bucket).add('Env', 'dev');
