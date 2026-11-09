package cdk

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/procexec"
	"github.com/yucn6201g/tag-refactor/internal/redact"
)

// tsHelper is the helper for TypeScript apps (contract C4, NFR6.3).
//
//go:embed helper/ts-helper.js
var tsHelper []byte

// HelperVersion is the version of the JSON of the helper (contract C4).
const HelperVersion = 1

// DefaultHelperTimeout is the limit of one call of the helper (NFR3.2).
const DefaultHelperTimeout = 2 * time.Minute

// The limits of the helper: its result may be large (16 MiB), the rest is
// small (reliability design §1).
const (
	helperOutputLimit = 16 << 20
	nodeVersionLimit  = 30 * time.Second
	minNodeMajor      = 18
)

// helperSum is the SHA-256 of the embedded helper, worked out at its first use
// (security design §5).
var helperSum = sync.OnceValue(func() [sha256.Size]byte { return sha256.Sum256(tsHelper) })

// afterHelperWritten, when set, is called between writing the helper and
// checking it: the tests change the file there (security design §5).
var afterHelperWritten func(path string)

// Edit is a range of a source the helper found (contract C3).
type Edit struct {
	// File is relative to the app, with /.
	File string `json:"file"`
	// Start and End are UTF-8 byte offsets; the range holds the quotes.
	Start int `json:"start"`
	End   int `json:"end"`
	// Shape is literal, sameFileConst, sameFileMap, crossFileArg or expression.
	Shape string `json:"shape"`
	// Text is the string without its quotes; empty for an expression.
	Text string `json:"text"`
	// SharedWith are the scopes of the other tag sites that write this range.
	SharedWith []string `json:"sharedWith"`
}

// The shapes of Edit.
const (
	ShapeLiteral       = "literal"
	ShapeSameFileConst = "sameFileConst"
	ShapeExpression    = "expression"
)

// TagSite is one place of the source that tags a scope (contract C3).
type TagSite struct {
	UseFile            string `json:"useFile"`
	UseLine            int    `json:"useLine"`
	Op                 string `json:"op"`
	ScopeConstructPath string `json:"scopeConstructPath"`
	KeyEdit            Edit   `json:"keyEdit"`
	ValueEdit          *Edit  `json:"valueEdit"`
	RemoveEdit         Edit   `json:"removeEdit"`
}

// ConstructSite is where the source makes a construct (contract C3).
type ConstructSite struct {
	ConstructPath string `json:"constructPath"`
	File          string `json:"file"`
	Line          int    `json:"line"`
	ConstructID   string `json:"constructId"`
	From          string `json:"from"`
}

// InsertSite is where a required tag could be added (contract C3); the helper
// of U1 finds none.
type InsertSite struct {
	ConstructPath string `json:"constructPath"`
	Kind          string `json:"kind"`
	File          string `json:"file"`
	At            int    `json:"at"`
	Indent        string `json:"indent"`
	Quote         string `json:"quote"`
	TrailingComma bool   `json:"trailingComma"`
	Target        string `json:"target"`
}

// UnsupportedSite is a place the helper could not read or did not understand.
type UnsupportedSite struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

// ReasonParseError is the reason of a file the helper could not parse.
const ReasonParseError = "parseError"

// TraceSite is a place a construct was made, from the trace of synth (U3).
type TraceSite struct {
	ConstructPath string `json:"constructPath"`
	File          string `json:"file"`
	Line          int    `json:"line"`
}

// AnalyzeRequest is what Analyze looks for.
type AnalyzeRequest struct {
	ConstructPaths []string
	TraceSites     []TraceSite
}

// Analysis is what the helper found in one app (contract C3).
type Analysis struct {
	TagSites          []TagSite         `json:"tagSites"`
	ConstructSites    []ConstructSite   `json:"constructSites"`
	InsertSites       []InsertSite      `json:"insertSites"`
	Unsupported       []UnsupportedSite `json:"unsupported"`
	TypeScriptVersion string            `json:"typescriptVersion"`
	// Debug holds the lines for --debug.
	Debug []string `json:"-"`
}

// HelperFailure is a failure that stops the reading of an app (BR1.6): the app
// fails and its resources go on the list to fix by hand.
type HelperFailure struct {
	// Reason is noTypeScript, nodeMissing, nodeTooOld, typescriptVersion,
	// timeout or internal.
	Reason string
	// Tool is node, typescript or helper.
	Tool string
}

func (e *HelperFailure) Error() string { return "the helper failed: " + e.Reason + " (" + e.Tool + ")" }

// ErrHelperVersion is the error of a result of another version: a fault of the
// build, which stops iac before anything is written (BR1.6).
var ErrHelperVersion = errors.New("the helper answered with another version of its JSON")

// Analyzer runs the helper (CdkSourceAnalyzer).
type Analyzer struct {
	cfg Config
	// Timeout is the limit of one call; zero means DefaultHelperTimeout.
	Timeout time.Duration
}

// NewAnalyzer returns an analyzer.
func NewAnalyzer(cfg Config) *Analyzer { return &Analyzer{cfg: cfg.withDefaults()} }

// Analyze reads the sources of an app once (contract C3, NFR3.5).
func (a *Analyzer) Analyze(ctx context.Context, app App, req AnalyzeRequest) (Analysis, error) {
	return a.call(ctx, app, helperRequest{Version: HelperVersion, Mode: "analyze", AppDir: app.Abs,
		ConstructPaths: nonNil(req.ConstructPaths), TraceSites: nonNilTrace(req.TraceSites), Files: []helperFile{}})
}

// Recheck reads the app again with the edited contents in place of the files on
// disk, which it does not write (contract C3, BR3.4). The keys of edited are
// relative to the app, with /.
func (a *Analyzer) Recheck(ctx context.Context, app App, edited map[string][]byte) (Analysis, error) {
	files := make([]helperFile, 0, len(edited))
	for p, content := range edited {
		files = append(files, helperFile{Path: p, Content: string(content)})
	}
	return a.call(ctx, app, helperRequest{Version: HelperVersion, Mode: "recheck", AppDir: app.Abs,
		ConstructPaths: []string{}, TraceSites: []TraceSite{}, Files: files})
}

type helperFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type helperRequest struct {
	Version        int          `json:"version"`
	Mode           string       `json:"mode"`
	AppDir         string       `json:"appDir"`
	ConstructPaths []string     `json:"constructPaths"`
	TraceSites     []TraceSite  `json:"traceSites"`
	Files          []helperFile `json:"files"`
}

type helperResponse struct {
	Version int `json:"version"`
	Analysis
	Error *struct {
		Kind string `json:"kind"`
	} `json:"error"`
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilTrace(s []TraceSite) []TraceSite {
	if s == nil {
		return []TraceSite{}
	}
	return s
}

// call checks Node.js, writes the helper, runs it once and reads its answer.
func (a *Analyzer) call(ctx context.Context, app App, req helperRequest) (Analysis, error) {
	var debug []string
	fail := func(err error) (Analysis, error) { return Analysis{Debug: debug}, err }
	node, err := a.cfg.Runner.LookPath("node")
	if err != nil {
		return fail(&HelperFailure{Reason: "nodeMissing", Tool: "node"})
	}
	env, err := isolate(a.cfg.Environ())
	if err != nil {
		return fail(&HelperFailure{Reason: "internal", Tool: "helper"})
	}
	defer env.close()
	if err := a.checkNode(ctx, node, env.vars, &debug); err != nil {
		return fail(err)
	}
	dir, err := privateDir()
	if err != nil {
		debug = append(debug, "cdk: helper: no private temporary folder")
		return fail(&HelperFailure{Reason: "internal", Tool: "helper"})
	}
	defer func() { _ = os.RemoveAll(dir) }()
	script, err := writeHelper(dir)
	if err != nil {
		debug = append(debug, "cdk: helper: "+err.Error())
		return fail(&HelperFailure{Reason: "internal", Tool: "helper"})
	}
	input, err := json.Marshal(req)
	if err != nil {
		return fail(&HelperFailure{Reason: "internal", Tool: "helper"})
	}
	debug = append(debug, fmt.Sprintf("cdk: %s: node <helper> (%s)", app.Dir, req.Mode))
	res, err := a.cfg.Runner.Run(ctx, procexec.Command{Name: node, Args: []string{script}, Dir: app.Abs, Env: env.vars,
		Stdin: input, Timeout: a.timeout(), StdoutLimit: helperOutputLimit})
	keptLines(res.Stderr, &debug, "cdk: "+app.Dir+": helper: ")
	if failure := runFailure(ctx, res, err); failure != nil {
		return fail(failure)
	}
	analysis, err := readResponse(res.Stdout)
	analysis.Debug = debug
	return analysis, err
}

func (a *Analyzer) timeout() time.Duration {
	if a.Timeout > 0 {
		return a.Timeout
	}
	return DefaultHelperTimeout
}

// runFailure turns the end of a run of the helper into its failure, if any.
func runFailure(ctx context.Context, res procexec.Result, err error) error {
	var start *procexec.StartError
	switch {
	case errors.Is(err, procexec.ErrTimeout):
		return &HelperFailure{Reason: "timeout", Tool: "helper"}
	case ctx.Err() != nil:
		return ctx.Err()
	case errors.As(err, &start):
		return &HelperFailure{Reason: "nodeMissing", Tool: "node"}
	case err != nil, res.ExitCode != 0, res.StdoutDropped > 0:
		return &HelperFailure{Reason: "internal", Tool: "helper"}
	}
	return nil
}

var nodeVersion = regexp.MustCompile(`^v(\d+)\.`)

// checkNode asks node its version and refuses one before 18 (NFR6.5).
func (a *Analyzer) checkNode(ctx context.Context, node string, env []string, debug *[]string) error {
	res, err := a.cfg.Runner.Run(ctx, procexec.Command{Name: node, Args: []string{"--version"}, Env: env, Timeout: nodeVersionLimit})
	if failure := runFailure(ctx, res, err); failure != nil {
		var hf *HelperFailure
		if errors.As(failure, &hf) {
			return &HelperFailure{Reason: "nodeMissing", Tool: "node"}
		}
		return failure
	}
	version := string(bytes.TrimSpace(res.Stdout))
	m := nodeVersion.FindStringSubmatch(version)
	if m == nil {
		*debug = append(*debug, "cdk: node --version answered "+redact.String(version))
		return &HelperFailure{Reason: "nodeTooOld", Tool: "node"}
	}
	if major, _ := strconv.Atoi(m[1]); major < minNodeMajor {
		*debug = append(*debug, "cdk: node "+version+" is older than 18")
		return &HelperFailure{Reason: "nodeTooOld", Tool: "node"}
	}
	return nil
}

// writeHelper writes the helper as a file only the user can read and write,
// reads it back and checks it against the embedded one (security design §5).
func writeHelper(dir string) (string, error) {
	script := filepath.Join(dir, "ts-helper.js")
	f, err := os.OpenFile(script, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(tsHelper); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if afterHelperWritten != nil {
		afterHelperWritten(script)
	}
	written, err := os.ReadFile(script)
	if err != nil {
		return "", err
	}
	if sha256.Sum256(written) != helperSum() {
		return "", errors.New("the helper written is not the embedded one")
	}
	return script, nil
}

// readResponse checks the version and the error of the helper's answer.
func readResponse(stdout []byte) (Analysis, error) {
	var resp helperResponse
	if err := json.Unmarshal(stdout, &resp); err != nil {
		return Analysis{}, &HelperFailure{Reason: "internal", Tool: "helper"}
	}
	if resp.Version != HelperVersion {
		return Analysis{}, ErrHelperVersion
	}
	if resp.Error != nil {
		if resp.Error.Kind == "noTypeScript" {
			return Analysis{}, &HelperFailure{Reason: "noTypeScript", Tool: "typescript"}
		}
		return Analysis{}, &HelperFailure{Reason: "internal", Tool: "helper"}
	}
	if !supportedTypeScript(resp.TypeScriptVersion) {
		return Analysis{}, &HelperFailure{Reason: "typescriptVersion", Tool: "typescript"}
	}
	return resp.Analysis, nil
}

var typeScriptMajor = regexp.MustCompile(`^(\d+)\.`)

// supportedTypeScript accepts TypeScript from 4.0 up to 5.x (NFR6.4).
func supportedTypeScript(v string) bool {
	m := typeScriptMajor.FindStringSubmatch(v)
	if m == nil {
		return false
	}
	major, _ := strconv.Atoi(m[1])
	return major == 4 || major == 5
}
