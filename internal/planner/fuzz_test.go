package planner

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// FuzzLoadPlan reads any bytes as a plan file (NFR8.18): LoadPlan never
// panics, gives an answer within one second (ten under -race), and a plan it
// reads saves again to bytes that read back to the same plan. In CI the
// seeds run as ordinary tests; locally
//
//	go test -run '^$' -fuzz FuzzLoadPlan -fuzztime 5m ./internal/planner/
//
// looks for more. Inputs that fail are written to
// testdata/fuzz/FuzzLoadPlan, where the seeds of the files also live.
func FuzzLoadPlan(f *testing.F) {
	valid := string(encode(f, samplePlan(f)))
	seeds := []string{
		valid,
		"\xef\xbb\xbf" + valid,
		"\xff\xfe{\x00\"\x00v\x00",
		"\xfe\xff\x00{\x00\"",
		strings.Replace(valid, `"version": 1`, `"version": 2`, 1),
		strings.Replace(valid, `"accountId": "123456789012",`, ``, 1),
		strings.Replace(valid, `"kind":"removeKey"`, `"kind":"dropKey"`, 1),
		strings.Replace(valid, `"changeId":"c-000002"`, `"changeId":"c-000001"`, 1),
		strings.Replace(valid, `"verdict":"affected"`, `"verdict":"none"`, 1),
		strings.Replace(valid, `"Name"`, "\"N\xffme\"", 1),
		`{"version":1,"x":` + strings.Repeat("[", 20000) + strings.Repeat("]", 20000) + `}`,
		`{"version":1,"toolVersion":"v","planId":"p","accountId":"a","createdAt":"2026-10-19T12:04:00.5+09:00","regions":[],` +
			`"inventoryComplete":true,"impactAnalysisComplete":true,"resources":[]}`,
		"", "null", "[]", "{}", `{"version":null}`, `{"version":1.5}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	path := filepath.Join(f.TempDir(), "plan.json")
	limit := time.Duration(timeFactor) * time.Second
	f.Fuzz(func(t *testing.T, data []byte) {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		p, err := LoadPlan(path)
		if elapsed := time.Since(start); elapsed > limit {
			t.Fatalf("LoadPlan took %v (limit %v) for %d bytes", elapsed, limit, len(data))
		}
		if err != nil {
			return
		}
		again := encode(t, p)
		q, err := ParsePlan("again.json", again)
		if err != nil {
			t.Fatalf("a plan that was read does not read back after saving: %v\n%s", err, again)
		}
		if !bytes.Equal(encode(t, q), again) {
			t.Fatal("saving the plan twice gives different bytes")
		}
		if !reflect.DeepEqual(p, q) {
			t.Fatalf("the plan changed through saving:\n%+v\n%+v", p, q)
		}
	})
}
