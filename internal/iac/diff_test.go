package iac

import (
	"strings"
	"testing"
)

// Unit tests of the difference (logical-components.md), written after the
// implementation.

func diffOf(t *testing.T, before string, edits ...edit) string {
	t.Helper()
	normal, err := normalize(edits)
	if err != nil {
		t.Fatal(err)
	}
	return unifiedDiff("f.yaml", []byte(before), applyEdits([]byte(before), normal), normal)
}

func at(s, sub string) int { return strings.Index(s, sub) }

func TestDiffReplaceWithContext(t *testing.T) {
	before := "1\n2\n3\n4\n5\n6\n7\n8\n9\n"
	got := diffOf(t, before, edit{at(before, "5"), at(before, "5") + 1, "five"})
	want := "--- a/f.yaml\n+++ b/f.yaml\n@@ -2,7 +2,7 @@\n 2\n 3\n 4\n-5\n+five\n 6\n 7\n 8"
	requireText(t, got, want)
}

func TestDiffInsertionHasNoOldLines(t *testing.T) {
	before := "1\n2\n3\n"
	got := diffOf(t, before, edit{at(before, "2") + 1, at(before, "2") + 1, "\nnew"})
	requireText(t, got, "--- a/f.yaml\n+++ b/f.yaml\n@@ -1,3 +1,4 @@\n 1\n 2\n+new\n 3")
}

func TestDiffInsertionInAOneLineFile(t *testing.T) {
	before := "1\n"
	got := diffOf(t, before, edit{1, 1, "\nnew"})
	requireText(t, got, "--- a/f.yaml\n+++ b/f.yaml\n@@ -1,1 +1,2 @@\n 1\n+new")
}

func TestDiffRemovalHasNoNewLines(t *testing.T) {
	before := "1\n2\n3\n4\n"
	got := diffOf(t, before, edit{at(before, "2"), at(before, "3"), ""})
	requireText(t, got, "--- a/f.yaml\n+++ b/f.yaml\n@@ -1,4 +1,3 @@\n 1\n-2\n 3\n 4")
}

func TestDiffRemovalOfTheOnlyLinesCountsZero(t *testing.T) {
	before := "1\n"
	got := diffOf(t, before, edit{0, 2, ""})
	requireText(t, got, "--- a/f.yaml\n+++ b/f.yaml\n@@ -1,1 +0,0 @@\n-1")
}

func TestDiffJoinsNearChangesAndSplitsFarOnes(t *testing.T) {
	var b strings.Builder
	for i := range 30 {
		b.WriteString("line" + itoa(i) + "\n")
	}
	before := b.String()
	one := func(name string) edit {
		return edit{at(before, name+"\n"), at(before, name+"\n") + len(name), name + "x"}
	}

	near := diffOf(t, before, one("line5"), one("line12"))
	if strings.Count(near, "@@ -") != 1 {
		t.Fatalf("six lines apart make one hunk:\n%s", near)
	}
	far := diffOf(t, before, one("line5"), one("line13"))
	if strings.Count(far, "@@ -") != 2 {
		t.Fatalf("seven lines apart make two hunks:\n%s", far)
	}
	if !strings.Contains(far, "@@ -3,7 +3,7 @@") || !strings.Contains(far, "@@ -11,7 +11,7 @@") {
		t.Fatalf("headers:\n%s", far)
	}
}

func TestDiffTwoEditsOnOneLineAreOneChange(t *testing.T) {
	before := `{"Key": "a", "Value": "b"}` + "\n"
	got := diffOf(t, before, edit{8, 11, `"A"`}, edit{22, 25, `"B"`})
	requireText(t, got, "--- a/f.yaml\n+++ b/f.yaml\n@@ -1,1 +1,1 @@\n-"+strings.TrimSuffix(before, "\n")+"\n+"+`{"Key": "A", "Value": "B"}`)
}

// A place replaced as a whole keeps the lines that do not change out of the
// marks.
func TestDiffSameLinesOfAReplacementAreContext(t *testing.T) {
	before := "a\nb\nc\nd\ne\n"
	got := diffOf(t, before, edit{at(before, "b"), at(before, "d") + 1, "b\nX\nd"})
	requireText(t, got, "--- a/f.yaml\n+++ b/f.yaml\n@@ -1,5 +1,5 @@\n a\n b\n-c\n+X\n d\n e")
}

func TestDiffWithoutALastLineBreak(t *testing.T) {
	before := "1\n2"
	got := diffOf(t, before, edit{2, 3, "two"})
	requireText(t, got, "--- a/f.yaml\n+++ b/f.yaml\n@@ -1,2 +1,2 @@\n 1\n-2\n\\ No newline at end of file\n+two\n\\ No newline at end of file")
	got = diffOf(t, before, edit{3, 3, "\n3"})
	requireText(t, got, "--- a/f.yaml\n+++ b/f.yaml\n@@ -1,2 +1,3 @@\n 1\n-2\n\\ No newline at end of file\n+2\n+3\n\\ No newline at end of file")
	// The last line goes with the line break before it: line 2 then ends the file
	// without one, which is a change of that line too.
	got = diffOf(t, "1\n2\n3", edit{3, 5, ""})
	requireText(t, got, "--- a/f.yaml\n+++ b/f.yaml\n@@ -1,3 +1,2 @@\n 1\n-2\n-3\n\\ No newline at end of file\n+2\n\\ No newline at end of file")
}

// Changes with no line between them show the old lines and then the new ones.
func TestDiffJoinsTouchingChanges(t *testing.T) {
	before := "a\nb\nc\nd\n"
	got := diffOf(t, before, edit{at(before, "a"), at(before, "a") + 1, "A"}, edit{at(before, "b"), at(before, "b") + 1, "B"})
	requireText(t, got, "--- a/f.yaml\n+++ b/f.yaml\n@@ -1,4 +1,4 @@\n-a\n-b\n+A\n+B\n c\n d")
}

func TestDiffShowsCRLFAsPlainLines(t *testing.T) {
	before := "a\r\nb\r\nc\r\n"
	got := diffOf(t, before, edit{at(before, "b"), at(before, "b") + 1, "B"})
	requireText(t, got, "--- a/f.yaml\n+++ b/f.yaml\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c")
}

func TestDiffFromARealRun(t *testing.T) {
	f := newFixture(t)
	f.file("sub/t.yaml", yamlTemplate("Q1", kv("env", "Prod", "old", "x")))
	res := f.run(f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"), remove("old", "x"), addTag("New", "y"))))
	want := "--- a/sub/t.yaml\n+++ b/sub/t.yaml\n@@ -3,7 +3,7 @@\n     Type: AWS::SQS::Queue\n     Properties:\n       Tags:\n" +
		"-        - Key: env\n+        - Key: Environment\n           Value: Prod\n-        - Key: old\n-          Value: x\n+        - Key: New\n+          Value: \"y\""
	requireText(t, res.Files[0].Diff, want)
}
