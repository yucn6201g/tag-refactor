#!/usr/bin/env python3
from aws_cdk import core
from aws_cdk import aws_s3 as s3


app = core.App()
stack = core.Stack(app, "tag-refactor-cdk-py-v1")
bucket = s3.Bucket(stack, "Bucket")
core.Tags.of(bucket).add("Env", "dev")

app.synth()
