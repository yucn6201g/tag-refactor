package display

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// NFR10.7: every message has English and Japanese, with the same values in
// the same order.

var verb = regexp.MustCompile(`%[-+# 0]*\d*(?:\.\d+)?[a-zA-Z%]`)

func verbs(format string) []string {
	var out []string
	for _, v := range verb.FindAllString(format, -1) {
		if v != "%%" {
			out = append(out, v)
		}
	}
	return out
}

func TestEveryMessageHasBothLanguagesWithTheSameValues(t *testing.T) {
	for id, m := range catalog {
		if m.en == "" || m.ja == "" {
			t.Errorf("%s: English %q, Japanese %q", id, m.en, m.ja)
			continue
		}
		en, ja := strings.Join(verbs(m.en), " "), strings.Join(verbs(m.ja), " ")
		if en != ja {
			t.Errorf("%s: English takes %q, Japanese %q", id, en, ja)
		}
		if m.one != "" && strings.Join(verbs(m.one), " ") != en {
			t.Errorf("%s: the singular takes %q, the plural %q", id, verbs(m.one), en)
		}
	}
	if T(English, "no.such.message") != "no.such.message" {
		t.Fatal("an unknown identifier comes back as it is")
	}
}

// The errors of a saved plan name catalog messages: every one of them is in
// the catalog with the planner's English text, so English output is the same
// whether the command line translates it or not.
func TestPlanFileMessagesMatchThePlanner(t *testing.T) {
	messages := planner.FileMessages()
	if len(messages) < 30 {
		t.Fatalf("only %d plan file messages", len(messages))
	}
	for id, en := range messages {
		m, ok := catalog[id]
		if !ok || m.en != en {
			t.Errorf("%s: catalog %q, planner %q", id, m.en, en)
		}
	}
}

// Every identifier the sources use is in the catalog; the ones put together
// at run time (kinds, states, statuses) are checked one by one.
func TestEveryIdentifierInTheSourcesIsInTheCatalog(t *testing.T) {
	prefixes := "unit|word|list|stderr|exit|scope|progress|state|result|status|discovery|scan|plan|kind|cause|diff|md|err|planfile|draft|apply|reason|journal"
	literal := regexp.MustCompile(`"((?:` + prefixes + `)\.[A-Za-z]+)"`)
	used := 0
	for _, dir := range []string{".", filepath.Join("..", "cli"), filepath.Join("..", "journal"), filepath.Join("..", "apply")} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") || strings.HasSuffix(f, "messages.go") && dir == "." {
				continue
			}
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range literal.FindAllStringSubmatch(string(src), -1) {
				used++
				if _, ok := catalog[m[1]]; !ok {
					t.Errorf("%s uses %q, which the catalog does not have", f, m[1])
				}
			}
		}
	}
	if used < 100 {
		t.Fatalf("only %d identifiers found; the search does not see the sources", used)
	}
	for _, k := range planner.Kinds {
		if _, ok := catalog["kind."+string(k)]; !ok {
			t.Errorf("no message for kind.%s", k)
		}
	}
	for _, s := range []string{stateWaiting, stateRunning, stateDone, stateFailed} {
		if _, ok := catalog["state."+s]; !ok {
			t.Errorf("no message for state.%s", s)
		}
	}
	for _, s := range []string{"complete", "partial", "unavailable", "failed"} {
		if _, ok := catalog["status."+s]; !ok {
			t.Errorf("no message for status.%s", s)
		}
	}
}
