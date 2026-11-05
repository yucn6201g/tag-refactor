package iac

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Unit tests of the collection of files, the limits and the nesting scan
// (BR3.1, NFR3.17 to NFR3.19, security design §2), written after the
// implementation.

func candidateNames(w *walked) string {
	names := make([]string, 0, len(w.candidates))
	for _, c := range w.candidates {
		names = append(names, c.rel)
	}
	return strings.Join(names, " ")
}

func TestWalkFindsCandidatesAndSkipsToolFolders(t *testing.T) {
	f := newFixture(t)
	for _, rel := range []string{"a.yaml", "b.yml", "c.json", "d.template", "main.tf", "sub/e.yaml", "sub/deep.tf", "x.txt", "UP.YAML",
		".git/h.yaml", ".terraform/i.yaml", "node_modules/j.yaml", "cdk.out/k.json", "sub/.git/l.yaml"} {
		f.file(rel, "x")
	}

	w, err := walkRoot(f.dir, true, true)

	if err != nil {
		t.Fatal(err)
	}
	if got := candidateNames(w); got != "UP.YAML a.yaml b.yml c.json d.template main.tf sub/e.yaml" {
		t.Fatalf("candidates = %s", got)
	}
}

func TestWalkLooksOnlyForWhatTheTargetsNeed(t *testing.T) {
	f := newFixture(t)
	f.file("a.yaml", "x")
	f.file("main.tf", "x")
	w, err := walkRoot(f.dir, false, true)
	if err != nil || candidateNames(w) != "main.tf" || w.seen != 2 {
		t.Fatalf("only Terraform: %v %+v", err, w)
	}
	w, err = walkRoot(f.dir, true, false)
	if err != nil || candidateNames(w) != "a.yaml" {
		t.Fatalf("only CloudFormation: %v %+v", err, w)
	}
}

// makeLink makes a link to a folder: a symbolic link, or on Windows a junction,
// which needs no rights.
func makeLink(t *testing.T, link, target string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
		if err != nil {
			t.Skipf("no junction: %v %s", err, out)
		}
		return
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("no symbolic link: %v", err)
	}
}

func TestLinksAreNotFollowed(t *testing.T) {
	f := newFixture(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "out.yaml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.file("inner/real.yaml", "x")
	makeLink(t, filepath.Join(f.dir, "to-outside"), outside)
	makeLink(t, filepath.Join(f.dir, "to-inside"), filepath.Join(f.dir, "inner"))

	w, err := walkRoot(f.dir, true, true)

	if err != nil {
		t.Fatal(err)
	}
	if got := candidateNames(w); got != "inner/real.yaml" {
		t.Fatalf("candidates = %s", got)
	}
}

func TestSymbolicLinkToAFileIsNotACandidate(t *testing.T) {
	f := newFixture(t)
	target := f.file("real.yaml", "x")
	if err := os.Symlink(target, filepath.Join(f.dir, "link.yaml")); err != nil {
		t.Skipf("no symbolic link: %v", err)
	}
	w, err := walkRoot(f.dir, true, true)
	if err != nil || candidateNames(w) != "real.yaml" {
		t.Fatalf("%v %s", err, candidateNames(w))
	}
}

// NFR3.18: --dir itself may be a link; the files of the real place are changed.
func TestDirThatIsALinkIsResolvedOnce(t *testing.T) {
	f := newFixture(t)
	f.file("t.yaml", yamlTemplate("Q1", "        - Key: a\n          Value: b\n"))
	link := filepath.Join(t.TempDir(), "link")
	makeLink(t, link, f.dir)
	f.commit()

	res, err := New().Run(t.Context(), Request{PlanPath: f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))), Dir: link, Write: true})

	if err != nil {
		t.Fatal(err)
	}
	if res.WriteErr != nil || len(res.Files) != 1 || !res.Files[0].Written {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(f.read("t.yaml"), "Value: c") {
		t.Fatal(f.read("t.yaml"))
	}
}

func TestFolderDepthLimit(t *testing.T) {
	f := newFixture(t)
	rel := ""
	for i := range maxFolderDepth + 1 {
		rel += "d" + strconv.Itoa(i) + "/"
		if i == maxFolderDepth-1 {
			f.file(rel+"last.yaml", "x")
		}
	}
	f.file(rel+"tooDeep.yaml", "x")

	w, err := walkRoot(f.dir, true, true)

	if err != nil {
		t.Fatal(err)
	}
	if len(w.candidates) != 1 || !strings.HasSuffix(w.candidates[0].rel, "/last.yaml") {
		t.Fatalf("candidates = %s", candidateNames(w))
	}
	if len(w.folders) != 1 {
		t.Fatalf("folders left out = %v", w.folders)
	}
}

func TestTooManyCandidatesStopOtherFilesDoNot(t *testing.T) {
	maxCandidates = 5
	t.Cleanup(func() { maxCandidates = 20000 })
	f := newFixture(t)
	for i := range 6 {
		f.file("c"+strconv.Itoa(i)+".yaml", "x")
	}
	_, err := walkRoot(f.dir, true, true)
	requireDetail(t, err, "iac.tooManyFiles")

	other := newFixture(t)
	for i := range 20 {
		other.file("n"+strconv.Itoa(i)+".txt", "x")
	}
	if _, err := walkRoot(other.dir, true, true); err != nil {
		t.Fatalf("files that are no candidates count: %v", err)
	}
}

func TestLargeFileIsLeftOutAndTheTotalStops(t *testing.T) {
	maxFileBytes = 10
	t.Cleanup(func() { maxFileBytes = 10 << 20 })
	f := newFixture(t)
	f.file("big.yaml", strings.Repeat("x", 11))
	f.file("ok.yaml", strings.Repeat("x", 10))
	w, err := walkRoot(f.dir, true, true)
	if err != nil || candidateNames(w) != "ok.yaml" || len(w.skipped) != 1 || w.skipped[0].Reason != SkipTooLarge {
		t.Fatalf("%v %+v", err, w)
	}

	maxTotalBytes = 15
	t.Cleanup(func() { maxTotalBytes = 500 << 20 })
	f.file("other.yaml", strings.Repeat("x", 10))
	_, err = walkRoot(f.dir, true, true)
	requireDetail(t, err, "iac.tooMuchData")
}

func TestContentChecks(t *testing.T) {
	c := func(format fileFormat) *candidate { return &candidate{rel: "x", format: format} }
	for name, raw := range map[string]string{
		"UTF-16 LE":   "\xff\xfeR\x00",
		"UTF-16 BE":   "\xfe\xff\x00R",
		"UTF-32 BE":   "\x00\x00\xfe\xffResources",
		"invalid":     "Resources: \xff\xff",
		"split rune":  "Resources: \xe3\x81",
		"bad in JSON": "{\"Resources\": \"\xc3\"}",
	} {
		if _, reason, _ := prepare(c(formatYAML), []byte(raw)); reason != SkipNotUTF8 {
			t.Errorf("%s: reason %q", name, reason)
		}
	}
	ct, reason, parse := prepare(c(formatYAML), []byte("\xef\xbb\xbfResources: {}"))
	if reason != "" || !parse || ct.base != 3 || string(ct.body) != "Resources: {}" {
		t.Fatalf("BOM: %+v %q %v", ct, reason, parse)
	}
	if _, reason, parse := prepare(c(formatJSON), []byte(`{"a":1}`)); reason != "" || parse {
		t.Fatalf("no Resources: %q %v", reason, parse)
	}
	if ct, _, parse := prepare(c(formatTerraform), []byte(`resource "a" "b" {}`)); !parse || ct == nil {
		t.Fatal("a .tf needs no word Resources")
	}
	if ct, _, _ := prepare(c(formatTemplate), []byte(" \n{\"Resources\":{}}")); ct.format != formatJSON {
		t.Fatalf("a template starting with { is JSON: %d", ct.format)
	}
	if ct, _, _ := prepare(c(formatTemplate), []byte("Resources: {}")); ct.format != formatYAML {
		t.Fatalf("other templates are YAML: %d", ct.format)
	}
}

func requireDeep(t *testing.T, name string, format fileFormat, data string, want bool) {
	t.Helper()
	if got := tooDeep(format, []byte(data)); got != want {
		t.Errorf("%s: too deep = %v, want %v", name, got, want)
	}
}

func TestNestingScansCatchWhatWouldBreakAParser(t *testing.T) {
	const many = 100000
	deepMap := strings.Builder{}
	for i := range 300 {
		deepMap.WriteString(strings.Repeat(" ", i) + "k:\n")
	}
	requireDeep(t, "YAML [", formatYAML, "a: "+strings.Repeat("[", many), true)
	requireDeep(t, "YAML {", formatYAML, strings.Repeat("{a: ", many), true)
	requireDeep(t, "YAML indents", formatYAML, deepMap.String(), true)
	requireDeep(t, "YAML dashes", formatYAML, strings.Repeat("- ", many)+"x\n", true)
	requireDeep(t, "YAML questions", formatYAML, strings.Repeat("? ", many)+"x\n", true)
	requireDeep(t, "JSON [", formatJSON, strings.Repeat("[", many), true)
	requireDeep(t, "JSON {", formatJSON, strings.Repeat(`{"a":`, many), true)
	requireDeep(t, "HCL [", formatTerraform, "a = "+strings.Repeat("[", many), true)
	requireDeep(t, "HCL (", formatTerraform, "a = "+strings.Repeat("(", many), true)
	requireDeep(t, "HCL {", formatTerraform, strings.Repeat("a {\n", many), true)
	requireDeep(t, "HCL !", formatTerraform, "a = "+strings.Repeat("!", many)+"b", true)
	requireDeep(t, "HCL - with spaces", formatTerraform, "a = "+strings.Repeat("- ", many)+"b", true)
	requireDeep(t, "HCL ?", formatTerraform, "a = "+strings.Repeat("b ? c : ", maxConditionals+1)+"d", true)
	requireDeep(t, "HCL ${", formatTerraform, `a = "`+strings.Repeat(`${"`, many), true)
	requireDeep(t, "HCL ${ in a heredoc", formatTerraform, "a = <<EOT\n"+strings.Repeat("${[", many)+"\nEOT\n", true)
}

// The scans must not leave out a sound file: brackets in strings, comments,
// block scalars and heredocs are only text.
func TestNestingScansLeaveSoundFilesAlone(t *testing.T) {
	brackets := strings.Repeat("[{(", 200)
	requireDeep(t, "YAML block scalar", formatYAML, "a: |\n  "+brackets+"\nb: >-\n  "+brackets+"\n", false)
	requireDeep(t, "YAML list block scalar", formatYAML, "- |\n  "+brackets+"\n- x\n", false)
	requireDeep(t, "YAML strings", formatYAML, "a: \""+brackets+"\"\nb: '"+brackets+"'\nc: 'it''s "+brackets+"'\n", false)
	requireDeep(t, "YAML string over lines", formatYAML, "a: \"x\n  "+brackets+"\"\nb: 1\n", false)
	requireDeep(t, "YAML comments", formatYAML, "# "+brackets+"\na: 1 # "+brackets+"\n", false)
	requireDeep(t, "YAML plain", formatYAML, "a: x"+brackets+"\nb: it's [not a list\n", false)
	requireDeep(t, "YAML CRLF", formatYAML, "a: |\r\n  "+brackets+"\r\nb: 1\r\n", false)
	requireDeep(t, "YAML tag", formatYAML, "a: !If [x, y]\nb: &anchor {c: 1}\nd: *anchor\n", false)
	requireDeep(t, "YAML wide list", formatYAML, strings.Repeat("- a: 1\n  b: 2\n", 5000), false)
	requireDeep(t, "YAML many levels under the limit", formatYAML, "a:\n  b:\n    c:\n      - d: [1, {e: 2}]\n", false)
	requireDeep(t, "JSON strings", formatJSON, `{"a":"`+brackets+`\"","b":"\\"}`, false)
	requireDeep(t, "JSON closed brackets", formatJSON, strings.Repeat("[1],", 1000), false)
	requireDeep(t, "HCL strings", formatTerraform, `a = "`+brackets+`"`+"\n", false)
	requireDeep(t, "HCL escaped template", formatTerraform, `a = "$${`+brackets+`"`+"\n", false)
	requireDeep(t, "HCL comments", formatTerraform, "# "+brackets+"\n// "+brackets+"\n/* "+brackets+" */\na = 1\n", false)
	requireDeep(t, "HCL heredoc", formatTerraform, "a = <<EOT\n"+brackets+"\nEOT\nb = <<-X\n  "+brackets+"\n  X\n", false)
	requireDeep(t, "HCL template", formatTerraform, `a = "${var.x}-${[1, 2][0]}"`+"\nb = 1\n", false)
	requireDeep(t, "HCL minus and conditionals", formatTerraform, "a = 1 - 2\nb = c ? d : e\nf-g = 1\n", false)
}

// NFR3.19: the deep files are left out and counted, without a crash, and the
// other files of the folder still work.
func TestDeepFilesAreLeftOutAndOthersStillWork(t *testing.T) {
	f := newFixture(t)
	f.file("deep.yaml", "Resources: "+strings.Repeat("[", 100000))
	f.file("deep.json", `{"Resources":`+strings.Repeat("[", 100000))
	f.file("dash.yaml", "Resources:\n  "+strings.Repeat("- ", 100000))
	f.file("t.yaml", yamlTemplate("Q1", "        - Key: a\n          Value: b\n"))

	res := f.run(f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))))

	if len(res.Files) != 1 || len(res.SkippedFiles) != 3 {
		t.Fatalf("result = %+v", res)
	}
	for _, s := range res.SkippedFiles {
		if s.Reason != SkipTooDeep {
			t.Errorf("%+v", s)
		}
	}
}

func TestDeepTerraformFilesAreLeftOut(t *testing.T) {
	f := newFixture(t)
	f.file("deep.tf", "a = "+strings.Repeat("[", 100000))
	f.file("bang.tf", "a = "+strings.Repeat("!", 100000)+"b\n")
	f.file("cond.tf", "a = "+strings.Repeat("b ? c : ", maxConditionals+1)+"d\n")
	f.file("main.tf", "resource \"aws_sqs_queue\" \"q\" {\n  tags = {\n    a = \"b\"\n  }\n}\n")
	r := tfResource(arnQ1, setValue("a", "b", "c"))

	res := f.runState(f.plan(r), f.state("aws_sqs_queue.q", arnQ1))

	if len(res.Files) != 1 || len(res.SkippedFiles) != 3 {
		t.Fatalf("result = %+v", res)
	}
}

// NFR3.19 (e): the alias bomb is never expanded, and is quick.
func TestYAMLAliasBombIsNotExpanded(t *testing.T) {
	f := newFixture(t)
	var b strings.Builder
	b.WriteString("x0: &x0 [a, a, a, a, a, a, a, a, a, a]\n")
	for i := 1; i < 40; i++ {
		b.WriteString("x" + strconv.Itoa(i) + ": &x" + strconv.Itoa(i) + " [")
		for j := range 10 {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString("*x" + strconv.Itoa(i-1))
		}
		b.WriteString("]\n")
	}
	f.file("bomb.yaml", b.String()+"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: *x39\n")
	start := time.Now()

	res := f.run(f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))))

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("took %v", elapsed)
	}
	requireReasons(t, res, "a tagsNotLiteral")
}

// One big file of many resources must not take time in the square of them: the
// readers find a resource through a map.
func TestManyResourcesInOneFile(t *testing.T) {
	const n = 20000
	var yamlText, jsonText, hcl strings.Builder
	var resources []resource
	yamlText.WriteString("Resources:\n")
	jsonText.WriteString("{\"Resources\": {\n")
	for i := range n {
		id := "Q" + strconv.Itoa(i)
		yamlText.WriteString("  " + id + ":\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Key: a\n          Value: b\n")
		if i > 0 {
			jsonText.WriteString(",\n")
		}
		jsonText.WriteString(`"J` + id + `": {"Type": "AWS::SQS::Queue", "Properties": {"Tags": [{"Key": "a", "Value": "b"}]}}`)
		hcl.WriteString("resource \"aws_sqs_queue\" \"t" + id + "\" {\n  tags = {\n    a = \"b\"\n  }\n}\n")
		resources = append(resources, stackResource("arn:aws:sqs:ap-northeast-1:111122223333:"+id, id, setValue("a", "b", "c")))
		resources = append(resources, stackResource("arn:aws:sqs:ap-northeast-1:111122223333:J"+id, "J"+id, setValue("a", "b", "c")))
	}
	jsonText.WriteString("\n}}\n")
	f := newFixture(t)
	f.file("big.yaml", yamlText.String())
	f.file("big.json", jsonText.String())
	f.file("big.tf", hcl.String())
	plan := f.plan(resources...)
	start := time.Now()

	res := f.run(plan)

	if elapsed := time.Since(start); elapsed > 30*time.Second*largeTimeFactor {
		t.Fatalf("took %v", elapsed)
	}
	if res.Reflected != 2*n || len(res.Files) != 2 {
		t.Fatalf("reflected %d, files %d, manual %d", res.Reflected, len(res.Files), len(res.Manual))
	}
	if parsed := parseFor(t, formatTerraform, hcl.String()); parsed.site("aws_sqs_queue.tQ19999") == nil {
		t.Fatal("no block")
	}
}

func TestUTF16FileIsLeftOutAndCounted(t *testing.T) {
	f := newFixture(t)
	f.file("wide.yaml", "\xff\xfeR\x00")
	f.file("t.yaml", yamlTemplate("Q1", "        - Key: a\n          Value: b\n"))

	res := f.run(f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))))

	if len(res.Files) != 1 || len(res.SkippedFiles) != 1 || res.SkippedFiles[0].Reason != SkipNotUTF8 {
		t.Fatalf("result = %+v", res)
	}
}

func TestFilesLeftOutAreNamedWhenThePlaceIsNotFound(t *testing.T) {
	f := newFixture(t)
	f.file("broken.yaml", "Resources: [\n")
	res := f.run(f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))))
	m := res.Manual[0]
	if m.Reason != ReasonTemplateNotFound || m.DetailKind != DetailSkipped || m.Detail != "broken.yaml" {
		t.Fatalf("item = %+v", m)
	}
	if s := res.SkippedFiles[0]; s.Reason != SkipParseError || s.Line == 0 || s.Kind != "yaml" {
		t.Fatalf("skipped = %+v", s)
	}
}
