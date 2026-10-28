package impact

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// fuzzChanges are matched against every document the fuzzer reads, so the
// binding and the verdicts run on whatever the document holds.
var fuzzChanges = []planner.TagChangeInput{
	{ChangeID: "c-000001", ResourceARN: "arn:aws:s3:::b", Kind: planner.KindRenameKey, FromKey: "env", ToKey: "Environment", FromValue: "Prod", ToValue: "Prod"},
	{ChangeID: "c-000002", ResourceARN: "arn:aws:iam::123456789012:role/r", Kind: planner.KindChangeValue, FromKey: "team", ToKey: "team", FromValue: "a", ToValue: "b"},
	{ChangeID: "c-000003", ResourceARN: "arn:aws:s3:::b", Kind: planner.KindAddRequired, ToKey: "Owner", ToValue: "x"},
	{ChangeID: "c-000004", ResourceARN: "arn:aws:s3:::b", Kind: planner.KindRemoveKey, FromKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab"},
}

// FuzzParsePolicy reads any text as a policy document (NFR8.21): reading it,
// undoing the URL encoding, finding the references and matching them never
// panics, and one input takes at most a second (ten under -race). In CI the
// seeds run as ordinary tests; locally
//
//	go test -run '^$' -fuzz FuzzParsePolicy -fuzztime 5m ./internal/impact/
//
// looks for more. Inputs that fail are written to
// testdata/fuzz/FuzzParsePolicy, where the seeds that are not text also live.
func FuzzParsePolicy(f *testing.F) {
	resource := `{"Version":"2012-10-17","Statement":[{"Sid":"S","Effect":"Deny","Principal":"*","Action":"s3:*",` +
		`"Resource":["arn:aws:s3:::b/${aws:PrincipalTag/team, 'none'}/*"],"NotResource":"arn:aws:s3:::x",` +
		`"Condition":{"ForAllValues:StringNotLikeIfExists":{"aws:TagKeys":["env*","50%"]},"StringEquals":{"aws:ResourceTag/env":"${aws:PrincipalTag/env}"}}}]}`
	seeds := []struct {
		doc     string
		encoded bool
	}{
		{url.PathEscape(policyWith("StringEquals", "aws:ResourceTag/env", "dev")), true},
		{resource, false},
		{`{"Statement":{"Sid":"One","Condition":{"Null":{"aws:RequestTag/Owner":"true"}}}}`, false},
		{`{"Statement":[{"Condition":{"StringEquals":{"aws:ResourceTag/a":["x",1,true,null,1.5e3]}}}]}`, false},
		{`{"Statement":[{"Condition":{"StringEquals":{"aws:ResourceTag/a":{"o":1}}}}]}`, false},
		{`{"Statement":[{"Resource":"arn:${aws:PrincipalTag/team","Condition":{"StringEquals":{"aws:ResourceTag/${aws:PrincipalTag/${x}}":"y"}}}]}`, false},
		{`{"Statement":[{"Condition":{"StringLike":{"aws:ResourceTag/a*a*a*a*a*a*a*a*b":"` + strings.Repeat("a", 4000) + `*b"}}}]}`, false},
		{`{"Statement":` + strings.Repeat("[", 5000) + strings.Repeat("]", 5000) + `}`, false},
		{"{\"Statement\":[{\"Sid\":\"\xff\xfe\",\"Condition\":{\"StringEquals\":{\"aws:ResourceTag/\xc3\":\"\xed\xa0\x80\"}}}]}", false},
		{"%7B%22Statement%22%3A%5B%5D%7D", true},
		{"%zz{", true},
		{`{"Statement":[{"Condition":{"StringEquals":{"aws:ResourceTag/env":"100%"}}}]}`, false},
		{"", false}, {"null", false}, {"[]", false}, {`{"Statement":"x"}`, false},
	}
	for _, s := range seeds {
		f.Add(s.doc, s.encoded)
	}
	limit := time.Duration(timeLimitFactor) * time.Second
	m := &matcher{index: newChangeIndex(fuzzChanges), patterns: newPatternCache()}
	sources := []source{{kind: planner.SourceIAMInline, ref: "r#p"}, {kind: planner.SourceS3Bucket, ref: "arn:aws:s3:::b", resourceARN: "arn:aws:s3:::b"}}
	f.Fuzz(func(t *testing.T, doc string, encoded bool) {
		start := time.Now()
		statements, err := parseDocument(doc, encoded)
		if err == nil {
			for _, src := range sources {
				pf := newPolicyFindings(src)
				for s := range statements {
					pf.stmtID = append(pf.stmtID, statements[s].id)
					refs := statements[s].references()
					for k := range refs {
						for _, i := range m.candidates(src, &refs[k]) {
							pf.add(s, &refs[k], i, m.judge(&refs[k], fuzzChanges[i]))
						}
					}
				}
				_ = pf.findings()
			}
		}
		if elapsed := time.Since(start); elapsed > limit {
			t.Fatalf("reading and matching took %v (limit %v) for %d bytes", elapsed, limit, len(doc))
		}
		if err != nil && err != errTooLarge && err != errNotPolicy {
			t.Fatalf("unexpected error %v", err)
		}
	})
}
