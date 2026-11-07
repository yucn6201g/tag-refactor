package iac

import (
	"strings"
	"testing"
)

// Unit tests of the readers of the three formats and of finding the place
// (BR3.2 to BR4.2, security design §3), written after the implementation.

func parseFor(t *testing.T, format fileFormat, text string) parsedFile {
	t.Helper()
	ct, reason, ok := prepare(&candidate{rel: "x", format: format}, []byte(text))
	if !ok || reason != "" {
		t.Fatalf("prepare: %q %v", reason, ok)
	}
	parsed, failure := parseContent(ct)
	if failure != nil {
		t.Fatalf("parse: %+v", failure)
	}
	return parsed
}

// siteOf reads the tags of the resource Q1, or the block aws_sqs_queue.q.
func yamlSite(t *testing.T, tags string) *tagSite {
	t.Helper()
	return parseFor(t, formatYAML, "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:"+tags).site("Q1")
}

func text(raw string, s span) string { return raw[s.start:s.end] }

// The bytes in a key's and a value's range are the text itself, whatever comes
// before them in the file (the reason yaml/v3 was chosen, security design §0).
func TestYAMLPositionsAfterCommentsBlockScalarsEscapesAndJapanese(t *testing.T) {
	head := "# コメント with a :colon and \"quotes\"\r\nParameters:\n  P: {Default: \"esc\\n\\u3042\\\"\"}\n" +
		"Description: |\n  block ### 日本語\n    more\n\n" +
		"Resources:\n  Other:\n    Type: AWS::SQS::Queue\n    Properties:\n      QueueName: 日本語のキュー\n" +
		"  Q1:  # 日本語\n    Type: AWS::SQS::Queue\n    Properties:\n      UserData: |\n        #!/bin/sh\n        echo \"Tags:\"\n" +
		"      Name: \"a\\tb\"\n      Tags:\n"
	raw := head + "        - Key: 環境   # 日本語のコメント\n          Value: 本番\n        - {Key: 'it''s', Value: \"x\\\"y\"}\n"
	site := parseFor(t, formatYAML, raw).site("Q1")

	if site == nil || site.issue != "" || len(site.entries) != 2 {
		t.Fatalf("site = %+v", site)
	}
	e := site.entries[0]
	if e.key != "環境" || e.value != "本番" || !e.literal || text(raw, e.keySpan) != "環境" || text(raw, e.valueSpan) != "本番" {
		t.Fatalf("entry = %+v", e)
	}
	if !e.hasComment {
		t.Fatal("the entry has a comment")
	}
	e = site.entries[1]
	if e.key != "it's" || e.value != `x"y` || text(raw, e.keySpan) != "'it''s'" || text(raw, e.valueSpan) != `"x\"y"` {
		t.Fatalf("entry = %+v", e)
	}
	if e.keyQuote != quoteSingle || e.valueQuote != quoteDouble {
		t.Fatalf("quotes = %d %d", e.keyQuote, e.valueQuote)
	}
}

func TestYAMLPositionsWithCRLFAndBOM(t *testing.T) {
	raw := "\xef\xbb\xbfResources:\r\n  Q1:\r\n    Type: AWS::SQS::Queue\r\n    Properties:\r\n      Tags:\r\n        - Key: a\r\n          Value: b\r\n"
	site := parseFor(t, formatYAML, raw).site("Q1")
	e := site.entries[0]
	if text(raw, e.keySpan) != "a" || text(raw, e.valueSpan) != "b" || text(raw, e.elem) != "- Key: a\r\n          Value: b" {
		t.Fatalf("entry = %+v", e)
	}
	if e.line != 6 {
		t.Fatalf("line = %d", e.line)
	}
}

func TestYAMLLineBreaksOfTheScanner(t *testing.T) {
	if yamlBreak([]byte("\r"), 0) != 1 || yamlBreak([]byte("\r\n"), 0) != 2 || yamlBreak([]byte("\xc2\x85"), 0) != 2 ||
		yamlBreak([]byte("\xe2\x80\xa8"), 0) != 3 || yamlBreak([]byte("\xe2\x80\xa9"), 0) != 3 ||
		yamlBreak([]byte("a"), 0) != 0 || yamlBreak([]byte("\xc2\xa0"), 0) != 0 || yamlBreak([]byte("\xe2\x80\x80"), 0) != 0 {
		t.Fatal("the breaks differ from the scanner's")
	}
	raw := "Description: a\xe2\x80\xa8  b\nResources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Key: k\n          Value: v\n"
	site := parseFor(t, formatYAML, raw).site("Q1")
	if site.issue != "" || text(raw, site.entries[0].valueSpan) != "v" {
		t.Fatalf("site = %+v", site)
	}
}

func TestYAMLLiteralValues(t *testing.T) {
	cases := map[string]struct {
		value   string
		literal bool
		want    string
	}{
		"plain":        {"Prod", true, "Prod"},
		"number stays": {"100", true, "100"},
		"bool stays":   {"yes", true, "yes"},
		"float stays":  {"1.0", true, "1.0"},
		"null stays":   {"null", true, "null"},
		"single":       {"'a b'", true, "a b"},
		"double":       {`"a\tb"`, true, "a\tb"},
		"empty double": {`""`, true, ""},
		"ref":          {"!Ref X", false, ""},
		"sub":          {"!Sub '${X}'", false, ""},
		"explicit str": {"!!str x", false, ""},
		"block":        {"|\n            x", false, ""},
		"folded":       {">-\n            x", false, ""},
		"two lines":    {"a\n            b", false, ""},
		"quoted lines": {"\"a\n            b\"", false, ""},
		"single lines": {"'a\n            b'", false, ""},
		"empty":        {"", false, ""},
		"mapping":      {"{Ref: X}", false, ""},
		"long form":    {"\n            Fn::Sub: x", false, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			site := yamlSite(t, "\n        - Key: k\n          Value: "+tc.value+"\n")
			if site.issue != "" || len(site.entries) != 1 {
				t.Fatalf("site = %+v", site)
			}
			e := site.entries[0]
			if e.literal != tc.literal || e.key != "k" {
				t.Fatalf("entry = %+v", e)
			}
			if tc.literal && e.value != tc.want {
				t.Fatalf("value = %q, want %q", e.value, tc.want)
			}
		})
	}
}

func TestYAMLPlacesThatAreNotLiteral(t *testing.T) {
	for name, tc := range map[string]struct {
		text string
		want ManualReason
	}{
		"Ref":             {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: !Ref T\n", ReasonTagsNotLiteral},
		"If":              {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: !If [a, b, c]\n", ReasonTagsNotLiteral},
		"long form":       {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        Fn::If: [a, b, c]\n", ReasonTagsNotLiteral},
		"alias":           {"X: &x\n  - {Key: a, Value: b}\nResources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: *x\n", ReasonTagsNotLiteral},
		"anchor on tags":  {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: &x\n        - {Key: a, Value: b}\n", ReasonTagsNotLiteral},
		"anchor on props": {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties: &p\n      Tags:\n        - {Key: a, Value: b}\n", ReasonTagsNotLiteral},
		"anchor on res":   {"Resources:\n  Q1: &r\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - {Key: a, Value: b}\n", ReasonTagsNotLiteral},
		"anchor on elem":  {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - &e {Key: a, Value: b}\n", ReasonTagsNotLiteral},
		"anchor on key":   {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Key: &k a\n          Value: b\n", ReasonTagsNotLiteral},
		"alias of value":  {"V: &v b\nResources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Key: a\n          Value: *v\n", ReasonTagsNotLiteral},
		"merge in props":  {"X: &x {A: 1}\nResources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      <<: *x\n      Tags:\n        - {Key: a, Value: b}\n", ReasonTagsNotLiteral},
		"merge in tags":   {"X: &x {a: b}\nResources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        <<: *x\n        c: d\n", ReasonTagsNotLiteral},
		"tagged elem":     {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - !If [a, {Key: b, Value: c}, d]\n", ReasonTagsNotLiteral},
		"long elem":       {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Fn::If: [a, b, c]\n", ReasonTagsNotLiteral},
		"props expr":      {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties: !If [a, b, c]\n", ReasonTagsNotLiteral},
		"list of strings": {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: [a, b]\n", ReasonUnsupportedTagShape},
		"no Value":        {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Key: a\n", ReasonUnsupportedTagShape},
		"a string":        {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: x\n", ReasonUnsupportedTagShape},
		"props string":    {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties: x\n", ReasonUnsupportedTagShape},
		"two Tags":        {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: []\n      Tags: []\n", ReasonTagsNotLiteral},
	} {
		t.Run(name, func(t *testing.T) {
			site := parseFor(t, formatYAML, tc.text).site("Q1")
			if site == nil || site.issue != tc.want {
				t.Fatalf("site = %+v, want %s", site, tc.want)
			}
		})
	}
}

func TestYAMLNoPlaceOfTags(t *testing.T) {
	for name, tail := range map[string]string{
		"no properties": "",
		"null props":    "    Properties:\n",
		"no tags":       "    Properties:\n      A: 1\n",
		"null tags":     "    Properties:\n      Tags:\n",
		"tilde tags":    "    Properties:\n      Tags: ~\n",
	} {
		if site := parseFor(t, formatYAML, "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n"+tail).site("Q1"); site != nil {
			t.Errorf("%s: site = %+v", name, site)
		}
	}
	if site := parseFor(t, formatYAML, "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n").site("Q2"); site != nil {
		t.Fatal("no such resource")
	}
}

func TestYAMLShapes(t *testing.T) {
	site := yamlSite(t, "\n        a: b   # c\n        d: 'e'\n")
	if site.shape != shapeMap || !site.multiline || len(site.entries) != 2 || site.entries[1].value != "e" {
		t.Fatalf("map = %+v", site)
	}
	if !site.entries[0].hasComment || site.entries[1].hasComment {
		t.Fatal("the comment belongs to the first entry")
	}
	site = yamlSite(t, " {a: b, c: d}\n")
	if site.shape != shapeMap || site.multiline || len(site.entries) != 2 {
		t.Fatalf("flow map = %+v", site)
	}
	site = yamlSite(t, " [{Key: a, Value: b}, {Key: c, Value: d}]\n")
	if site.shape != shapeKeyValueList || site.multiline {
		t.Fatalf("flow list = %+v", site)
	}
	site = yamlSite(t, "\n        - {Key: a, Value: b}\n        - {Key: c, Value: d}\n")
	if !site.multiline {
		t.Fatalf("a block list of flow maps has lines of its own: %+v", site)
	}
	site = yamlSite(t, "\n        - Key: a\n          Value: b\n          PropagateAtLaunch: true\n")
	if !site.extraFields || site.entries[0].elem.end <= site.entries[0].valueSpan.end {
		t.Fatalf("extra fields = %+v", site)
	}
	site = yamlSite(t, "\n        - Value: b\n          Key: a\n")
	if site.entries[0].key != "a" || site.entries[0].value != "b" {
		t.Fatalf("value first = %+v", site)
	}
	site = yamlSite(t, "\n        - Key: a\n          Value: b\n        - Key: a\n          Value: c\n")
	if site.entries[0].literal || site.entries[1].literal {
		t.Fatal("a key written twice is not for this tool to choose")
	}
	raw := "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        -\n          Key: a\n          Value: b\n"
	site = parseFor(t, formatYAML, raw).site("Q1")
	if !site.multiline || text(raw, site.entries[0].elem) != "-\n          Key: a\n          Value: b" {
		t.Fatalf("a dash on its own line: %+v", site)
	}
}

func TestYAMLElementsExcludeCommentsBetweenThem(t *testing.T) {
	raw := "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n" +
		"        # before\n        - Key: a  # own\n          Value: b  # own2\n\n        # - Key: off\n        #   Value: x\n" +
		"        - Key: c\n          Value: d\n\n      # after the tags\n      Other: 1\n"
	site := parseFor(t, formatYAML, raw).site("Q1")

	if got := text(raw, site.entries[0].elem); got != "- Key: a  # own\n          Value: b  # own2" {
		t.Fatalf("elem = %q", got)
	}
	if got := text(raw, site.entries[1].elem); got != "- Key: c\n          Value: d" {
		t.Fatalf("elem = %q", got)
	}
	want := "        # before\n\n        # - Key: off\n        #   Value: x\n\n      # after the tags\n"
	if site.others != want {
		t.Fatalf("others = %q, want %q", site.others, want)
	}
}

func TestYAMLResourcesWithoutTypeAreNoTemplate(t *testing.T) {
	for name, raw := range map[string]string{
		"list":        "- Resources\n",
		"no mapping":  "Resources: x\n",
		"no type":     "Resources:\n  A:\n    Foo: 1\n",
		"not a value": "Resources:\n  A: 1\n",
		"type expr":   "Resources:\n  A:\n    Type: [x]\n",
		"empty":       "# Resources\n",
	} {
		if refs := parseFor(t, formatYAML, raw).refs(); len(refs) != 0 {
			t.Errorf("%s: refs = %+v", name, refs)
		}
	}
	refs := parseFor(t, formatYAML, "X: &x\n  Type: AWS::SQS::Queue\nResources:\n  A: *x\n").refs()
	if len(refs) != 1 || refs[0].cloudFormationType != "AWS::SQS::Queue" {
		t.Fatalf("an alias still has a Type: %+v", refs)
	}
	if site := parseFor(t, formatYAML, "X: &x\n  Type: AWS::SQS::Queue\nResources:\n  A: *x\n").site("A"); site == nil || site.issue != ReasonTagsNotLiteral {
		t.Fatalf("site = %+v", site)
	}
}

func TestYAMLParseErrorKeepsOnlyThePlace(t *testing.T) {
	ct, _, _ := prepare(&candidate{rel: "x", format: formatYAML}, []byte("Resources:\n  a: [SECRET-VALUE\n"))
	_, failure := parseContent(ct)
	if failure == nil || failure.kind != "yaml" || failure.line == 0 {
		t.Fatalf("failure = %+v", failure)
	}
}

func jsonTemplate(tags string) string {
	return `{"Resources": {"Q1": {"Type": "AWS::SQS::Queue", "Properties": {"Tags": ` + tags + `}}}}`
}

func TestJSONReadsKeysAndValuesWithTheirRanges(t *testing.T) {
	raw := "\xef\xbb\xbf{\n  \"Description\": \"日本語 \\\" {\",\n  \"Other\": [1, {\"a\": null}],\n  \"Resources\": {\n    \"Q1\": {\n      \"Type\": \"AWS::SQS::Queue\",\n" +
		"      \"Properties\": {\n        \"Tags\": [\n          {\"Key\": \"日本語\", \"Value\": \"a\\\"b\\u3042\"},\n          {\"Value\": \"2\", \"Key\": \"k\", \"X\": 1}\n        ]\n      }\n    }\n  }\n}\n"
	parsed := parseFor(t, formatJSON, raw)
	site := parsed.site("Q1")

	if site == nil || site.issue != "" || site.shape != shapeKeyValueList || !site.multiline || !site.extraFields {
		t.Fatalf("site = %+v", site)
	}
	e := site.entries[0]
	if e.key != "日本語" || e.value != `a"bあ` || text(raw, e.keySpan) != `"日本語"` || text(raw, e.valueSpan) != `"a\"b\u3042"` {
		t.Fatalf("entry = %+v", e)
	}
	if got := text(raw, e.elem); !strings.HasPrefix(got, "{") || !strings.HasSuffix(got, "}") {
		t.Fatalf("elem = %q", got)
	}
	if e.line != 9 {
		t.Fatalf("line = %d", e.line)
	}
	if got := text(raw, site.value); !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
		t.Fatalf("value = %q", got)
	}
	refs := parsed.refs()
	if len(refs) != 1 || refs[0].line != 5 || refs[0].cloudFormationType != "AWS::SQS::Queue" {
		t.Fatalf("refs = %+v", refs)
	}
}

func TestJSONShapes(t *testing.T) {
	site := parseFor(t, formatJSON, jsonTemplate(`[{"Key": "a", "Value": "b"}, {"Key": "c", "Value": "d"}]`)).site("Q1")
	if site.multiline || len(site.entries) != 2 {
		t.Fatalf("one line: %+v", site)
	}
	site = parseFor(t, formatJSON, jsonTemplate("[{\"Key\": \"a\", \"Value\": \"b\"}, {\"Key\": \"c\",\n \"Value\": \"d\"}]")).site("Q1")
	if site.multiline {
		t.Fatalf("two elements share a line: %+v", site)
	}
	site = parseFor(t, formatJSON, jsonTemplate("{\n  \"a\": \"b\",\n  \"c\": {\"Ref\": \"X\"}\n}")).site("Q1")
	if site.shape != shapeMap || !site.multiline || !site.entries[0].literal || site.entries[1].literal || site.entries[1].key != "c" {
		t.Fatalf("map = %+v", site)
	}
	site = parseFor(t, formatJSON, jsonTemplate(`[{"Key": "a", "Value": {"Ref": "X"}}, {"Key": {"Ref": "Y"}, "Value": "d"}, {"Key": "e", "Value": 1}]`)).site("Q1")
	if site.entries[0].literal || site.entries[0].key != "a" || site.entries[1].key != "" || site.entries[2].literal {
		t.Fatalf("not literal: %+v", site.entries)
	}
	site = parseFor(t, formatJSON, jsonTemplate(`[{"Key": "a", "Value": "1"}, {"Key": "a", "Value": "2"}]`)).site("Q1")
	if site.entries[0].literal {
		t.Fatal("a key written twice is not literal")
	}
	site = parseFor(t, formatJSON, jsonTemplate(`[{"Key": "a", "Key": "b", "Value": "2"}]`)).site("Q1")
	if site.entries[0].literal {
		t.Fatal("Key written twice is not literal")
	}
}

func TestJSONPlacesThatAreNotLiteral(t *testing.T) {
	for name, tc := range map[string]struct {
		tags string
		want ManualReason
	}{
		"Fn::If":      {`{"Fn::If": ["a", [], []]}`, ReasonTagsNotLiteral},
		"Ref":         {`{"Ref": "T"}`, ReasonTagsNotLiteral},
		"If in list":  {`[{"Fn::If": ["a", {}, {}]}]`, ReasonTagsNotLiteral},
		"strings":     {`["a"]`, ReasonUnsupportedTagShape},
		"no Value":    {`[{"Key": "a"}]`, ReasonUnsupportedTagShape},
		"number":      {`7`, ReasonUnsupportedTagShape},
		"a text":      {`"x"`, ReasonUnsupportedTagShape},
		"two of Tags": {`[], "Tags": []`, ReasonTagsNotLiteral},
	} {
		t.Run(name, func(t *testing.T) {
			site := parseFor(t, formatJSON, jsonTemplate(tc.tags)).site("Q1")
			if site == nil || site.issue != tc.want {
				t.Fatalf("site = %+v, want %s", site, tc.want)
			}
		})
	}
	if site := parseFor(t, formatJSON, jsonTemplate("null")).site("Q1"); site != nil {
		t.Fatalf("null tags: %+v", site)
	}
	for name, raw := range map[string]string{
		"no props":  `{"Resources":{"Q1":{"Type":"AWS::SQS::Queue"}}}`,
		"no tags":   `{"Resources":{"Q1":{"Type":"AWS::SQS::Queue","Properties":{}}}}`,
		"other key": `{"Resources":{"Q2":{"Type":"AWS::SQS::Queue"}}}`,
		"empty":     `{"Resources":{}}`,
		"no object": `{"Resources":[]}`,
	} {
		if site := parseFor(t, formatJSON, raw).site("Q1"); site != nil {
			t.Errorf("%s: site = %+v", name, site)
		}
	}
	if site := parseFor(t, formatJSON, `{"Resources":{"Q1":{"Type":"x","Properties":{"Fn::If":[]}}}}`).site("Q1"); site.issue != ReasonTagsNotLiteral {
		t.Fatalf("site = %+v", site)
	}
	if site := parseFor(t, formatJSON, `{"Resources":{"Q1":{"Type":"x","Properties":1}}}`).site("Q1"); site.issue != ReasonUnsupportedTagShape {
		t.Fatalf("site = %+v", site)
	}
	if site := parseFor(t, formatJSON, `{"Resources":{"Q1":7}}`).site("Q1"); site.issue != ReasonUnsupportedTagShape {
		t.Fatalf("site = %+v", site)
	}
	if site := parseFor(t, formatJSON, `{"Resources":{"Q1":{"Type":"x"},"Q1":{"Type":"y"}}}`).site("Q1"); site.issue != ReasonTagsNotLiteral {
		t.Fatalf("site = %+v", site)
	}
}

func TestJSONOtherTopLevelsAndErrors(t *testing.T) {
	for _, raw := range []string{`[{"Resources":1}]`, `"Resources"`, `{"a":{"Resources":{"Q1":{"Type":"x"}}}}`} {
		if refs := parseFor(t, formatJSON, raw).refs(); len(refs) != 0 {
			t.Errorf("%s: refs %+v", raw, refs)
		}
	}
	for _, raw := range []string{"{\"Resources\": {\"a\": SECRET}}", "{\"Resources\": 1} extra", "{\"Resources\""} {
		ct, _, _ := prepare(&candidate{rel: "x", format: formatJSON}, []byte(raw))
		_, failure := parseContent(ct)
		if failure == nil || failure.kind != "json" || failure.line != 1 {
			t.Errorf("%s: failure %+v", raw, failure)
		}
	}
}

func hclSite(t *testing.T, tags string) *tagSite {
	t.Helper()
	return parseFor(t, formatTerraform, "resource \"aws_sqs_queue\" \"q\" {\n  name = \"x\"\n  tags = "+tags+"\n}\n").site("aws_sqs_queue.q")
}

func TestHCLReadsKeysAndValuesWithTheirRanges(t *testing.T) {
	raw := "\xef\xbb\xbf# 日本語のコメント\nresource \"aws_sqs_queue\" \"q\" {\n  name = \"日本語\"\n  tags = {\n    env = \"Prod\" # c\n    \"a/b\" = \"日本語\\\"x\"\n    multi = \"$${x}\",\n  }\n}\n"
	parsed := parseFor(t, formatTerraform, raw)
	site := parsed.site("aws_sqs_queue.q")

	if site == nil || site.issue != "" || site.shape != shapeMap || !site.multiline || len(site.entries) != 3 {
		t.Fatalf("site = %+v", site)
	}
	e := site.entries[0]
	if e.key != "env" || e.value != "Prod" || text(raw, e.keySpan) != "env" || text(raw, e.valueSpan) != `"Prod"` || !e.hasComment || e.keyQuote != quotePlain {
		t.Fatalf("entry = %+v", e)
	}
	e = site.entries[1]
	if e.key != "a/b" || e.value != `日本語"x` || text(raw, e.keySpan) != `"a/b"` || e.keyQuote != quoteDouble || e.hasComment {
		t.Fatalf("entry = %+v", e)
	}
	e = site.entries[2]
	if e.value != "${x}" || !e.literal || text(raw, e.elem) != `multi = "$${x}",` {
		t.Fatalf("entry = %+v", e)
	}
	if got := parsed.refs(); len(got) != 1 || got[0].key != "aws_sqs_queue.q" || got[0].line != 2 {
		t.Fatalf("refs = %+v", got)
	}
}

func TestHCLValuesThatAreNotLiteral(t *testing.T) {
	cases := map[string]string{
		"variable":      "var.x",
		"interpolation": `"a-${var.x}"`,
		"only template": `"${var.x}"`,
		"directive":     `"%{if true}a%{endif}"`,
		"number":        "1",
		"bool":          "true",
		"function":      `upper("a")`,
		"heredoc":       "<<EOT\nx\nEOT\n",
		"null":          "null",
		"object":        `{ a = "b" }`,
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			site := hclSite(t, "{\n    k = "+value+"\n  }")
			if site.issue != "" || len(site.entries) != 1 || site.entries[0].literal {
				t.Fatalf("site = %+v", site)
			}
		})
	}
	site := hclSite(t, `{ (var.k) = "a", "${var.k}" = "b", a.b = "c" }`)
	for i := range site.entries {
		if site.entries[i].literal {
			t.Errorf("entry %d is not literal: %+v", i, site.entries[i])
		}
	}
}

func TestHCLPlacesThatAreNotLiteral(t *testing.T) {
	for name, tags := range map[string]string{
		"merge":    `merge(local.c, { a = "b" })`,
		"local":    "local.common",
		"variable": "var.tags",
		"for":      `{ for k, v in var.t : k => v }`,
		"null":     "null",
	} {
		t.Run(name, func(t *testing.T) {
			if site := hclSite(t, tags); site == nil || site.issue != ReasonTagsNotLiteral {
				t.Fatalf("site = %+v", site)
			}
		})
	}
	for name, block := range map[string]string{
		"tag block":     "  tag {\n    key = \"a\"\n  }\n",
		"tags block":    "  tags {\n    a = \"b\"\n  }\n",
		"dynamic block": "  dynamic \"tag\" {\n    for_each = []\n    content {}\n  }\n",
	} {
		site := parseFor(t, formatTerraform, "resource \"aws_autoscaling_group\" \"g\" {\n"+block+"}\n").site("aws_autoscaling_group.g")
		if site == nil || site.issue != ReasonUnsupportedTagShape {
			t.Errorf("%s: site = %+v", name, site)
		}
	}
	if site := parseFor(t, formatTerraform, "resource \"a\" \"b\" {\n  dynamic \"other\" {\n    for_each = []\n    content {}\n  }\n}\n").site("a.b"); site != nil {
		t.Fatalf("no tags: %+v", site)
	}
	if site := parseFor(t, formatTerraform, "resource \"a\" \"b\" {}\n").site("a.c"); site != nil {
		t.Fatalf("no block: %+v", site)
	}
}

func TestHCLLines(t *testing.T) {
	for name, tc := range map[string]struct {
		tags      string
		multiline bool
	}{
		"own lines":    {"{\n    a = \"b\"\n    c = \"d\"\n  }", true},
		"one line":     {`{ a = "b", c = "d" }`, false},
		"same line":    {"{\n    a = \"b\", c = \"d\"\n  }", false},
		"open line":    {"{ a = \"b\"\n    c = \"d\"\n  }", false},
		"close line":   {"{\n    a = \"b\"\n    c = \"d\" }", false},
		"comment head": {"{\n    /* x */ a = \"b\"\n  }", false},
		"empty":        {"{}", false},
		"comments":     {"{\n    # top\n    a = \"b\" // x\n    c = \"d\" /* y */\n  }", true},
	} {
		t.Run(name, func(t *testing.T) {
			if site := hclSite(t, tc.tags); site.multiline != tc.multiline {
				t.Fatalf("multiline = %v", site.multiline)
			}
		})
	}
	site := hclSite(t, "{\n    # a comment\n    a = \"b\"\n\n    # another\n  }")
	if site.others != "    # a comment\n\n    # another\n" {
		t.Fatalf("others = %q", site.others)
	}
	if site := hclSite(t, "{\n    a = \"b\"\n    a = \"c\"\n  }"); site.entries[0].literal {
		t.Fatal("a key written twice is not literal")
	}
}

func TestHCLOtherBlocksAndErrors(t *testing.T) {
	parsed := parseFor(t, formatTerraform, "data \"a\" \"b\" {}\nresource \"one\" {}\nmodule \"m\" {}\nresource \"a\" \"c\" {}\n")
	if refs := parsed.refs(); len(refs) != 1 || refs[0].key != "a.c" {
		t.Fatalf("refs = %+v", refs)
	}
	ct, _, _ := prepare(&candidate{rel: "x", format: formatTerraform}, []byte("resource \"a\" {\n  secret = \"SECRET-VALUE\n"))
	_, failure := parseContent(ct)
	if failure == nil || failure.kind != "hcl" || failure.line == 0 {
		t.Fatalf("failure = %+v", failure)
	}
}

func sqsTemplate(id string) string {
	return "Resources:\n  " + id + ":\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Key: a\n          Value: b\n"
}

func TestPlaceOfATemplate(t *testing.T) {
	t.Run("one", func(t *testing.T) {
		f := newFixture(t)
		f.file("x/t.yaml", sqsTemplate("Q1"))
		res := f.run(f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))))
		if len(res.Files) != 1 || res.Files[0].File != "x/t.yaml" {
			t.Fatalf("result = %+v", res)
		}
	})
	t.Run("two, one named as the stack", func(t *testing.T) {
		f := newFixture(t)
		f.file("dev/stack.yaml", sqsTemplate("Q1"))
		f.file("prod/other.yaml", sqsTemplate("Q1"))
		res := f.run(f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))))
		if len(res.Files) != 1 || res.Files[0].File != "dev/stack.yaml" {
			t.Fatalf("result = %+v", res)
		}
	})
	t.Run("two, both named as the stack", func(t *testing.T) {
		f := newFixture(t)
		f.file("dev/stack.yaml", sqsTemplate("Q1"))
		f.file("prod/stack.yaml", sqsTemplate("Q1"))
		res := f.run(f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))))
		requireReasons(t, res, "a ambiguous")
		if m := res.Manual[0]; m.DetailKind != DetailCandidates || m.Detail != "dev/stack.yaml, prod/stack.yaml" {
			t.Fatalf("item = %+v", m)
		}
	})
	t.Run("another type", func(t *testing.T) {
		f := newFixture(t)
		f.file("t.yaml", strings.ReplaceAll(sqsTemplate("Q1"), "AWS::SQS::Queue", "AWS::SNS::Topic"))
		res := f.run(f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))))
		requireReasons(t, res, "a templateNotFound")
		if m := res.Manual[0]; m.DetailKind != DetailOtherType || m.Detail != "t.yaml" {
			t.Fatalf("item = %+v", m)
		}
	})
	t.Run("a type outside the table", func(t *testing.T) {
		f := newFixture(t)
		f.file("t.yaml", strings.ReplaceAll(sqsTemplate("Q1"), "AWS::SQS::Queue", "AWS::Foo::Bar"))
		res := f.run(f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))))
		requireReasons(t, res, "a templateNotFound")
	})
	t.Run("twice in one file", func(t *testing.T) {
		f := newFixture(t)
		f.file("t.yaml", sqsTemplate("Q1")+"  Q1:\n    Type: AWS::SQS::Queue\n")
		res := f.run(f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))))
		requireReasons(t, res, "a ambiguous")
	})
	t.Run("no such logical ID", func(t *testing.T) {
		f := newFixture(t)
		f.file("t.yaml", sqsTemplate("Q2"))
		res := f.run(f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))))
		requireReasons(t, res, "a templateNotFound")
		if res.Manual[0].DetailKind != DetailNone {
			t.Fatalf("item = %+v", res.Manual[0])
		}
	})
}

func TestPlaceOfABlock(t *testing.T) {
	block := "resource \"aws_sqs_queue\" \"q\" {\n  tags = {\n    a = \"b\"\n  }\n}\n"
	r := tfResource(arnQ1, setValue("a", "b", "c"))
	t.Run("one", func(t *testing.T) {
		f := newFixture(t)
		f.file("main.tf", block)
		res := f.runState(f.plan(r), f.state("aws_sqs_queue.q", arnQ1))
		if len(res.Files) != 1 || res.Files[0].Tool != ToolTerraform {
			t.Fatalf("result = %+v", res)
		}
	})
	t.Run("none", func(t *testing.T) {
		f := newFixture(t)
		f.file("sub/main.tf", block)
		res := f.runState(f.plan(r), f.state("aws_sqs_queue.q", arnQ1))
		requireReasons(t, res, "a blockNotFound")
	})
	t.Run("none, with a file left out", func(t *testing.T) {
		f := newFixture(t)
		f.file("bad.tf", "resource {{{\n")
		res := f.runState(f.plan(r), f.state("aws_sqs_queue.q", arnQ1))
		requireReasons(t, res, "a blockNotFound")
		if m := res.Manual[0]; m.DetailKind != DetailSkipped || m.Detail != "bad.tf" {
			t.Fatalf("item = %+v", m)
		}
	})
	t.Run("two", func(t *testing.T) {
		f := newFixture(t)
		f.file("a.tf", block)
		f.file("b.tf", block)
		res := f.runState(f.plan(r), f.state("aws_sqs_queue.q", arnQ1))
		requireReasons(t, res, "a ambiguous")
	})
}

func TestAllTagsSignaturesTellResourcesApart(t *testing.T) {
	yamlFile := parseFor(t, formatYAML, sqsTemplate("Q1")+"  Q2:\n    Type: AWS::SNS::Topic\n    Properties:\n      Tags: !Ref X\n  Q3:\n    Type: AWS::SNS::Topic\n")
	tags := yamlFile.allTags()
	if len(tags) != 3 || tags[0].tags == tags[1].tags || tags[2].tags != "-" || tags[1].tags != string(ReasonTagsNotLiteral) {
		t.Fatalf("tags = %+v", tags)
	}
	jsonFile := parseFor(t, formatJSON, jsonTemplate(`[{"Key":"a","Value":"b"}]`))
	if got := jsonFile.allTags(); len(got) != 1 || !strings.Contains(got[0].tags, "a") {
		t.Fatalf("tags = %+v", got)
	}
	hcl := parseFor(t, formatTerraform, "resource \"a\" \"b\" {\n  tags = {\n    k = var.x\n  }\n}\n")
	if got := hcl.allTags(); len(got) != 1 || !strings.Contains(got[0].tags, "~") {
		t.Fatalf("tags = %+v", got)
	}
}
