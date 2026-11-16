package com.example;

import software.amazon.awscdk.Stack;
import software.amazon.awscdk.Tags;
import software.amazon.awscdk.services.s3.Bucket;

public final class App {
    public static void main(final String[] args) {
        software.amazon.awscdk.App app = new software.amazon.awscdk.App();
        Stack stack = new Stack(app, "tag-refactor-cdk-java");
        Bucket bucket = new Bucket(stack, "Bucket");
        Tags.of(bucket).add("Env", "dev");
        app.synth();
    }
}
