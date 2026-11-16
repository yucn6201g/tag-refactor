#!/usr/bin/env python3
import aws_cdk as cdk

from py_minimal.py_minimal_stack import PyMinimalStack


app = cdk.App()
PyMinimalStack(app, "tag-refactor-cdk-py-minimal",
    stack_name="tag-refactor-cdk-py-minimal",
    synthesizer=cdk.DefaultStackSynthesizer(generate_bootstrap_version_rule=False),
)

app.synth()
