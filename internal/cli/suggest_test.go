package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/rules"
)

// Unit tests of the parts of rules suggest that the command line runs
// cannot reach easily, written after the implementation. The tests of the
// draft file moved to internal/fileout with writeDraftFileWith (U4).

func TestSuggestExitCodes(t *testing.T) {
	cases := []struct {
		o       outcome
		code    int
		meaning string
	}{
		{outcome{command: suggestCommandName}, ExitOK, "draft ready"},
		{outcome{command: suggestCommandName, needsReview: true}, ExitOK, "draft ready"},
		{outcome{command: suggestCommandName, regionsMissing: true, draftTooLarge: true}, ExitError, "results incomplete"},
		{outcome{command: suggestCommandName, draftTooLarge: true}, ExitError, "draft larger than 64 KiB"},
	}
	for _, tc := range cases {
		code, id := tc.o.exitCode()
		if meaning := display.T(display.English, id); code != tc.code || meaning != tc.meaning {
			t.Errorf("%+v: got %d %q, want %d %q", tc.o, code, meaning, tc.code, tc.meaning)
		}
	}
}

// The English comments of the display catalog are the rules package's own,
// so the English draft is the same whichever gives them; Japanese runs get
// the catalog's Japanese and English runs nothing (the default).
func TestDraftCommentsFollowTheLanguage(t *testing.T) {
	if got := catalogComments(display.English); !reflect.DeepEqual(got, rules.EnglishComments()) {
		t.Fatalf("catalog %+v\nrules %+v", got, rules.EnglishComments())
	}
	if draftComments(display.English) != nil {
		t.Fatal("English keeps the default comments")
	}
	ja := draftComments(display.Japanese)
	if ja == nil || ja.Review[0] == rules.EnglishComments().Review[0] || !strings.Contains(ja.Generated, "%s") {
		t.Fatalf("Japanese comments = %+v", ja)
	}
}

// The rules package keeps its own copy of the region name form; it must
// agree with the one --region uses.
func TestRegionNameFormsAgree(t *testing.T) {
	for _, name := range []string{"us-east-1", "ap-northeast-1", "us-gov-west-1", "cn-north-1", "il-central-1",
		"useast1", "US-EAST-1", "us-east", "us-east-1 ", "-east-1", "a-b-1", "abcde-west-1", ""} {
		if rules.IsRegionName(name) != awsaccess.IsRegionName(name) {
			t.Errorf("%q: rules says %v, awsaccess says %v", name, rules.IsRegionName(name), awsaccess.IsRegionName(name))
		}
	}
}

func TestPreferredSpellingAndToolVersion(t *testing.T) {
	p := preferredSpelling()
	for group, want := range map[string]string{"environment": "environment", "production": "production", "env": "", "owner": ""} {
		if got := p(group); got != want {
			t.Errorf("preferred(%q) = %q, want %q", group, got, want)
		}
	}
	if v := toolVersion(); v == "" || strings.ContainsAny(v, " \n()") {
		t.Fatalf("tool version = %q", v)
	}
}
