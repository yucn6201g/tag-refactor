package testutil

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

// LargeIac is a folder of IaC sources with a plan, the measuring conditions of
// U8 (NFR5.25): 10,000 resources with about 30,000 changes, half in CloudFormation
// templates and half in Terraform files, about 100 MB, about one change in ten
// to fix by hand, and 1,000 JSON files that are no templates. Everything is
// divided by the scale, and the same seed gives the same folder.
type LargeIac struct {
	// Dir holds the sources.
	Dir string
	// PlanPath and StatePath are outside Dir.
	PlanPath, StatePath string
	// Candidates counts the files with a candidate extension.
	Candidates int
	// FilesWithTargets counts the files that hold resources of the plan.
	FilesWithTargets int
	// Resources and Changes count what the plan holds.
	Resources, Changes int
	// ManualResources counts the resources written so that a change of theirs
	// has to be fixed by hand.
	ManualResources int
}

const (
	largeResourcesPerFile = 5
	// The bytes of padding of a resource, to reach about 60 and 40 kilobytes a
	// file.
	largeTemplatePadding  = 11500
	largeTerraformPadding = 7800
)

// largeIacChange is one change of the plan as contract C2 writes it.
type largeIacChange map[string]any

// WriteLargeIac writes the folder and the plan below root.
func WriteLargeIac(root string, scale int) (*LargeIac, error) {
	l := &LargeIac{Dir: filepath.Join(root, "src"), PlanPath: filepath.Join(root, "plan.json"), StatePath: filepath.Join(root, "state.json")}
	rng := rand.New(rand.NewSource(8))
	files := 1000 / scale
	var resources []map[string]any
	var state []map[string]any
	for i := range files {
		// YAML, YAML, YAML, YAML, JSON: 800 and 200 of the 1,000.
		var body string
		var err error
		rel := fmt.Sprintf("stacks/s%03d/t%04d.yaml", i%50, i)
		if i%5 == 4 {
			rel = fmt.Sprintf("stacks/s%03d/t%04d.json", i%50, i)
		}
		var entries []largeEntry
		for j := range largeResourcesPerFile {
			e := newLargeEntry(rng, fmt.Sprintf("Q%dR%d", i, j), l)
			entries = append(entries, e)
			before := map[string]string{"aws:cloudformation:stack-name": fmt.Sprintf("t%04d", i), "aws:cloudformation:logical-id": e.id, "env": "Prod", "obsolete": "x"}
			resources = append(resources, largeResource(e, before))
		}
		if i%5 == 4 {
			body = largeJSONTemplate(entries)
		} else {
			body = largeYAMLTemplate(entries)
		}
		if err = l.write(rel, body); err != nil {
			return nil, err
		}
		l.FilesWithTargets++
	}
	for i := range files {
		var blocks strings.Builder
		for j := range largeResourcesPerFile {
			e := newLargeEntry(rng, fmt.Sprintf("q%dr%d", i, j), l)
			writeLargeBlock(&blocks, e)
			resources = append(resources, largeResource(e, map[string]string{"env": "Prod", "obsolete": "x"}))
			state = append(state, map[string]any{"address": "aws_sqs_queue." + e.id, "mode": "managed", "type": "aws_sqs_queue", "name": e.id,
				"values": map[string]any{"arn": e.arn, "name": e.id}})
		}
		if err := l.write(fmt.Sprintf("m%04d.tf", i), blocks.String()); err != nil {
			return nil, err
		}
		l.FilesWithTargets++
	}
	for i := range files {
		if err := l.write(fmt.Sprintf("config/c%04d.json", i), fmt.Sprintf(`{"name": "config-%d", "values": [1, 2, 3]}`, i)); err != nil {
			return nil, err
		}
	}
	l.Resources = len(resources)
	if err := writeLargeJSON(l.PlanPath, map[string]any{"version": 1, "toolVersion": "test", "planId": "large", "accountId": "111122223333",
		"createdAt": "2026-10-31T00:00:00Z", "regions": []string{"ap-northeast-1"}, "inventoryComplete": true, "impactAnalysisComplete": true,
		"ruleFile": "r.yaml", "resources": resources}); err != nil {
		return nil, err
	}
	err := writeLargeJSON(l.StatePath, map[string]any{"format_version": "1.0", "values": map[string]any{"root_module": map[string]any{"resources": state}}})
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (l *LargeIac) write(rel, body string) error {
	path := filepath.Join(l.Dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if !strings.HasPrefix(rel, "config/") || strings.HasSuffix(rel, ".json") {
		l.Candidates++
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func writeLargeJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// largeEntry is one resource of the folder: its tags are env, obsolete and
// Owner; about one in ten has its env written so that it has to be fixed by hand.
type largeEntry struct {
	id, arn string
	manual  bool
	changes []largeIacChange
}

func newLargeEntry(rng *rand.Rand, id string, l *LargeIac) largeEntry {
	e := largeEntry{id: id, arn: "arn:aws:sqs:ap-northeast-1:111122223333:" + id, manual: rng.Intn(10) == 0}
	if e.manual {
		l.ManualResources++
	}
	number := l.Changes
	next := func() string {
		number++
		return fmt.Sprintf("c-%06d", number)
	}
	e.changes = []largeIacChange{
		{"changeId": next(), "kind": "renameKey", "fromKey": "env", "toKey": "Environment", "fromValue": "Prod", "mergesIntoExisting": false, "impacted": false},
		{"changeId": next(), "kind": "changeValue", "fromKey": "Environment", "fromValue": "Prod", "toValue": "prod", "impacted": false},
	}
	if rng.Intn(2) == 0 {
		e.changes = append(e.changes, largeIacChange{"changeId": next(), "kind": "removeKey", "fromKey": "obsolete", "fromValue": "x", "impacted": false})
	} else {
		e.changes = append(e.changes, largeIacChange{"changeId": next(), "kind": "addRequired", "toKey": "CostCenter", "toValue": "100", "impacted": false})
	}
	l.Changes = number
	return e
}

func largeResource(e largeEntry, before map[string]string) map[string]any {
	return map[string]any{"arn": e.arn, "region": "ap-northeast-1", "global": false, "resourceType": "sqs", "beforeTags": before,
		"iacManagedBy": nil, "changes": e.changes}
}

func largeYAMLTemplate(entries []largeEntry) string {
	var b strings.Builder
	b.Grow(70000)
	b.WriteString("AWSTemplateFormatVersion: \"2010-09-09\"\n# 大きなテストのテンプレート\nResources:\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "  %s:\n    Type: AWS::SQS::Queue\n    Properties:\n      QueueName: !Sub \"%s-${AWS::StackName}\"\n      Description: |\n", e.id, e.id)
		line := "        padding text of the template, with [brackets] and {braces} # and no comment\n"
		for range largeTemplatePadding / len(line) {
			b.WriteString(line)
		}
		env := "Prod"
		if e.manual {
			env = "!Ref EnvParam"
		}
		fmt.Fprintf(&b, "      Tags:\n        - Key: env  # 環境\n          Value: %s\n        - Key: obsolete\n          Value: x\n        - Key: Owner\n          Value: \"team-a\"\n", env)
	}
	return b.String()
}

func largeJSONTemplate(entries []largeEntry) string {
	var b strings.Builder
	b.Grow(70000)
	b.WriteString("{\n  \"Resources\": {\n")
	for i, e := range entries {
		env := `"Prod"`
		if e.manual {
			env = `{"Ref": "EnvParam"}`
		}
		fmt.Fprintf(&b, "    \"%s\": {\n      \"Type\": \"AWS::SQS::Queue\",\n      \"Properties\": {\n        \"QueueName\": \"%s\",\n        \"Description\": \"%s\",\n        \"Tags\": [\n"+
			"          {\"Key\": \"env\", \"Value\": %s},\n          {\"Key\": \"obsolete\", \"Value\": \"x\"},\n          {\"Key\": \"Owner\", \"Value\": \"team-a\"}\n        ]\n      }\n    }",
			e.id, e.id, strings.Repeat("padding [ { text ", largeTemplatePadding/17), env)
		if i < len(entries)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("  }\n}\n")
	return b.String()
}

func writeLargeBlock(b *strings.Builder, e largeEntry) {
	env := `"Prod"`
	if e.manual {
		env = "var.env"
	}
	fmt.Fprintf(b, "resource \"aws_sqs_queue\" \"%s\" {\n  name = \"%s\"\n  policy = <<EOT\n", e.id, e.id)
	line := "padding text of the policy with [brackets] and {braces}\n"
	for range largeTerraformPadding / len(line) {
		b.WriteString(line)
	}
	fmt.Fprintf(b, "EOT\n  tags = {\n    env      = %s  # 環境\n    obsolete = \"x\"\n    Owner    = \"team-a\"\n  }\n}\n\n", env)
}
