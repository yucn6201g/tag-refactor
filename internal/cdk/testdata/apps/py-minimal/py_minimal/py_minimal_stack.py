from aws_cdk import (
    RemovalPolicy,
    Stack,
    Tags,
    aws_s3 as s3,
)
from constructs import Construct


class PyMinimalStack(Stack):

    def __init__(self, scope: Construct, construct_id: str, **kwargs) -> None:
        super().__init__(scope, construct_id, **kwargs)

        s3.Bucket(self, "Bucket",
            block_public_access=s3.BlockPublicAccess.BLOCK_ALL,
            encryption=s3.BucketEncryption.S3_MANAGED,
            removal_policy=RemovalPolicy.DESTROY,
        )
        Tags.of(self).add("Env", "dev")
