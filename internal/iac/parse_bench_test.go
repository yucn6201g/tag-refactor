package iac

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	yaml "go.yaml.in/yaml/v3"
)

// The parsers' speed and memory on 10 MB (logical-components.md, NFR5.25):
// the time, the bytes allocated, and the heap still held after the parse.

const benchSize = 10 << 20

// benchYAML builds a CloudFormation-like YAML template of about size bytes.
func benchYAML(size int) []byte {
	var b strings.Builder
	b.Grow(size + 1024)
	b.WriteString("AWSTemplateFormatVersion: '2010-09-09'\nResources:\n")
	for i := 0; b.Len() < size; i++ {
		fmt.Fprintf(&b, "  Queue%d:\n    Type: AWS::SQS::Queue\n    Properties:\n      QueueName: !Sub 'queue-%d-${AWS::StackName}'\n"+
			"      VisibilityTimeout: 30\n      Tags:\n        - Key: env   # environment\n          Value: Prod\n"+
			"        - Key: Owner\n          Value: \"team-%d\"\n        - Key: obsolete\n          Value: x\n", i, i, i)
	}
	return []byte(b.String())
}

// benchTF builds a Terraform file of about size bytes.
func benchTF(size int) []byte {
	var b strings.Builder
	b.Grow(size + 1024)
	for i := 0; b.Len() < size; i++ {
		fmt.Fprintf(&b, "resource \"aws_sqs_queue\" \"q%d\" {\n  name                       = \"queue-%d\"\n  visibility_timeout_seconds = 30\n"+
			"  tags = {\n    env      = \"Prod\"\n    Owner    = \"team-%d\"\n    obsolete = \"x\"\n  }\n}\n\n", i, i, i)
	}
	return []byte(b.String())
}

// heapHeld returns the heap in use after a collection.
func heapHeld() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func reportHeld(b *testing.B, before uint64, keep any) {
	b.Helper()
	after := heapHeld()
	runtime.KeepAlive(keep)
	if after > before {
		b.ReportMetric(float64(after-before)/(1<<20), "MBheld")
	}
}

func BenchmarkParseYAML(b *testing.B) {
	data := benchYAML(benchSize)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	before := heapHeld()
	var node yaml.Node
	for b.Loop() {
		node = yaml.Node{}
		if err := yaml.Unmarshal(data, &node); err != nil {
			b.Fatal(err)
		}
	}
	reportHeld(b, before, &node)
}

func BenchmarkParseHCL(b *testing.B) {
	data := benchTF(benchSize)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	before := heapHeld()
	var file *hcl.File
	for b.Loop() {
		var diags hcl.Diagnostics
		file, diags = hclsyntax.ParseConfig(data, "bench.tf", hcl.InitialPos)
		if diags.HasErrors() {
			b.Fatal("parse failed")
		}
	}
	reportHeld(b, before, file)
}
