package cdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/procexec"
	"github.com/yucn6201g/tag-refactor/internal/redact"
)

// DefaultSynthTimeout is the time limit of cdk synth (NFR3.1).
const DefaultSynthTimeout = 10 * time.Minute

// FailureReason says why an app was not synthesized (contract C2).
type FailureReason string

// The reasons of Failure.
const (
	ReasonToolMissing         FailureReason = "toolMissing"
	ReasonNeedsAWS            FailureReason = "needsAWS"
	ReasonTimeout             FailureReason = "timeout"
	ReasonCDKV1               FailureReason = "cdkV1"
	ReasonUnknownVersion      FailureReason = "unknownVersion"
	ReasonUnsupportedLanguage FailureReason = "unsupportedLanguage"
	ReasonSynthError          FailureReason = "synthError"
)

// Failure is why an app was not synthesized.
type Failure struct {
	Reason FailureReason
	// Tool is the program missing or failing: cdk or node.
	Tool string
	// Detail is the kept lines of standard error, already redacted. It goes to
	// --debug only, never to a pull request (contract X2).
	Detail string
}

// Synthesis is the result of one app (contract C2).
type Synthesis struct {
	App     App
	Stacks  []Stack
	Elapsed time.Duration
	// Failure is set when the app was not synthesized; Stacks is then empty.
	Failure *Failure
	// Debug holds the lines for --debug: the command, the names of the
	// environment and the kept lines of standard error. No values.
	Debug []string
}

// Stack is one stack of the cloud assembly.
type Stack struct {
	// Name is the stack name that is deployed: stackName of the manifest, or
	// the ID of the artifact.
	Name string
	// Account and Region come from the environment of the stack; empty when it
	// is not decided.
	Account, Region string
	Resources       []SynthesizedResource
}

// SynthesizedResource is one resource of a template.
type SynthesizedResource struct {
	LogicalID, Type string
	// ConstructPath is aws:cdk:path of the metadata of the resource.
	ConstructPath string
	// Tags are the string tags of Properties.Tags.
	Tags map[string]string
}

// Options are the settings of one synth.
type Options struct {
	// Timeout is the limit of cdk synth; zero means DefaultSynthTimeout.
	Timeout time.Duration
}

// ProgressSink receives the states of the apps (contract C6). display.Printer
// implements it.
type ProgressSink interface {
	// AppsWaiting lists the apps to synthesize; iac sends it once.
	AppsWaiting(apps []string)
	AppStarted(app, language string)
	AppDone(app string, elapsed time.Duration)
	// AppFailed gets the reason (a FailureReason or a failure of the helper),
	// never the detail.
	AppFailed(app string, elapsed time.Duration, reason string)
	AppSkipped(app, reason string)
}

// NopSink is a ProgressSink that shows nothing.
type NopSink struct{}

func (NopSink) AppsWaiting([]string)                    {}
func (NopSink) AppStarted(string, string)               {}
func (NopSink) AppDone(string, time.Duration)           {}
func (NopSink) AppFailed(string, time.Duration, string) {}
func (NopSink) AppSkipped(string, string)               {}

func orNop(s ProgressSink) ProgressSink {
	if s == nil {
		return NopSink{}
	}
	return s
}

// Config is what the synthesizer and the analyzer take from their
// surroundings. The zero value runs the programs of the computer with the
// environment of the process.
type Config struct {
	Runner  procexec.Runner
	Environ func() []string
	Now     func() time.Time
}

func (c Config) withDefaults() Config {
	if c.Runner == nil {
		c.Runner = procexec.System{}
	}
	if c.Environ == nil {
		c.Environ = os.Environ
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// Synthesizer finds and synthesizes the apps (CdkSynthesizer).
type Synthesizer struct {
	cfg Config
}

// NewSynthesizer returns a synthesizer.
func NewSynthesizer(cfg Config) *Synthesizer { return &Synthesizer{cfg: cfg.withDefaults()} }

// Synthesizable reports an app U1 synthesizes: TypeScript with CDK v2 (BR1.2).
func (a App) Synthesizable() bool { return skipReason(a) == "" }

// skipReason is why an app is not synthesized at all (BR1.2).
func skipReason(a App) FailureReason {
	switch {
	case a.Language != TypeScript:
		return ReasonUnsupportedLanguage
	case a.CDKMajor == 1:
		return ReasonCDKV1
	case a.CDKMajor != 2:
		return ReasonUnknownVersion
	}
	return ""
}

// Synthesize synthesizes one app (BR1.3 to BR1.5, BR1.7). A failure of the app
// is in Synthesis.Failure; the error is only the one of the context.
func (s *Synthesizer) Synthesize(ctx context.Context, app App, opts Options, sink ProgressSink) (Synthesis, error) {
	sink = orNop(sink)
	syn := Synthesis{App: app}
	if reason := skipReason(app); reason != "" {
		syn.Failure = &Failure{Reason: reason}
		sink.AppSkipped(app.Dir, string(reason))
		return syn, nil
	}
	start := s.cfg.Now()
	sink.AppStarted(app.Dir, string(app.Language))
	stacks, failure, err := s.run(ctx, app, opts, &syn.Debug)
	syn.Elapsed = s.cfg.Now().Sub(start)
	if err != nil {
		return syn, err
	}
	if failure != nil {
		syn.Failure = failure
		sink.AppFailed(app.Dir, syn.Elapsed, string(failure.Reason))
		return syn, nil
	}
	syn.Stacks = stacks
	sink.AppDone(app.Dir, syn.Elapsed)
	return syn, nil
}

// run runs cdk synth into a temporary folder and reads what it wrote.
func (s *Synthesizer) run(ctx context.Context, app App, opts Options, debug *[]string) ([]Stack, *Failure, error) {
	if _, err := s.cfg.Runner.LookPath("node"); err != nil {
		return nil, &Failure{Reason: ReasonToolMissing, Tool: "node"}, nil
	}
	cdk := s.findCDK(app)
	if cdk == "" {
		return nil, &Failure{Reason: ReasonToolMissing, Tool: "cdk"}, nil
	}
	out, err := os.MkdirTemp("", "tag-refactor-synth-")
	if err != nil {
		return nil, &Failure{Reason: ReasonSynthError, Tool: "cdk", Detail: "no temporary folder for the output"}, nil
	}
	defer func() { _ = os.RemoveAll(out) }()
	env, err := isolate(s.cfg.Environ())
	if err != nil {
		return nil, &Failure{Reason: ReasonSynthError, Tool: "cdk", Detail: "no temporary files for the environment"}, nil
	}
	defer env.close()
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultSynthTimeout
	}
	args := []string{"synth", "--quiet", "--no-notices", "--output", out}
	*debug = append(*debug, fmt.Sprintf("cdk: %s: %s synth --quiet --no-notices --output <temporary folder>", app.Dir, filepath.Base(cdk)),
		"cdk: "+app.Dir+": environment: "+env.names())
	res, err := s.cfg.Runner.Run(ctx, procexec.Command{Name: cdk, Args: args, Dir: app.Abs, Env: env.vars, Timeout: timeout})
	if failure, err := classifyRun(ctx, res, err, debug, app.Dir); failure != nil || err != nil {
		return nil, failure, err
	}
	stacks, err := readAssembly(out)
	if err != nil {
		*debug = append(*debug, "cdk: "+app.Dir+": the cloud assembly could not be read: "+redact.String(err.Error()))
		return nil, &Failure{Reason: ReasonSynthError, Tool: "cdk", Detail: "the cloud assembly could not be read"}, nil
	}
	return stacks, nil, nil
}

// findCDK returns the cdk of the app's node_modules, else the cdk of PATH
// (NFR6.6); npx is never used, it could fetch cdk from the network.
func (s *Synthesizer) findCDK(app App) string {
	name := "cdk"
	if runtime.GOOS == "windows" {
		name = "cdk.cmd"
	}
	local := filepath.Join(app.Abs, "node_modules", ".bin", name)
	if info, err := os.Stat(local); err == nil && info.Mode().IsRegular() {
		return local
	}
	if p, err := s.cfg.Runner.LookPath("cdk"); err == nil {
		return p
	}
	return ""
}

// The known lines of cdk synth that mean it needs AWS (BR1.5); U3 adds more.
var needsAWSLines = []string{
	"Unable to resolve AWS account to use",
	"Need to perform AWS calls",
	"Could not load credentials",
	"Unable to locate credentials",
}

// classifyRun turns the end of cdk synth into a failure (BR1.5), or nothing
// when it succeeded. The error is the one of the context.
func classifyRun(ctx context.Context, res procexec.Result, err error, debug *[]string, dir string) (*Failure, error) {
	var start *procexec.StartError
	switch {
	case errors.Is(err, procexec.ErrTimeout):
		return &Failure{Reason: ReasonTimeout, Tool: "cdk"}, nil
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case errors.As(err, &start):
		return &Failure{Reason: ReasonToolMissing, Tool: "cdk"}, nil
	case err != nil:
		return &Failure{Reason: ReasonSynthError, Tool: "cdk"}, nil
	case res.ExitCode == 0:
		return nil, nil
	}
	detail := keptLines(res.Stderr, debug, "cdk: "+dir+": ")
	reason := ReasonSynthError
	for _, line := range needsAWSLines {
		if strings.Contains(string(res.Stderr), line) {
			reason = ReasonNeedsAWS
		}
	}
	return &Failure{Reason: reason, Tool: "cdk", Detail: detail}, nil
}

// keptLines keeps the known lines of standard error for --debug (security
// design §4) and says how many it dropped.
func keptLines(stderr []byte, debug *[]string, prefix string) string {
	kept, dropped := redact.Lines(stderr)
	for _, line := range kept {
		*debug = append(*debug, prefix+line)
	}
	if dropped > 0 {
		*debug = append(*debug, fmt.Sprintf("%s(%d lines of standard error left out)", prefix, dropped))
	}
	return strings.Join(kept, "\n")
}

// maxAssemblyFileBytes bounds what is read of the manifest and of a template.
const maxAssemblyFileBytes = 64 << 20

// readAssembly reads the stacks of the cloud assembly in dir (BR1.4, rules.md
// on the open question of C2): the artifacts of type aws:cloudformation:stack,
// with properties.stackName (the ID of the artifact when it is missing), the
// environment aws://<account>/<region>, and properties.templateFile.
func readAssembly(dir string) ([]Stack, error) {
	data, err := readAssemblyFile(dir, "manifest.json")
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Artifacts map[string]struct {
			Type        string `json:"type"`
			Environment string `json:"environment"`
			Properties  struct {
				TemplateFile string `json:"templateFile"`
				StackName    string `json:"stackName"`
			} `json:"properties"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	var stacks []Stack
	for id, a := range manifest.Artifacts {
		if a.Type != "aws:cloudformation:stack" {
			continue
		}
		st := Stack{Name: a.Properties.StackName}
		if st.Name == "" {
			st.Name = id
		}
		st.Account, st.Region = parseEnvironment(a.Environment)
		if st.Resources, err = readTemplate(dir, a.Properties.TemplateFile); err != nil {
			return nil, err
		}
		stacks = append(stacks, st)
	}
	slices.SortFunc(stacks, func(a, b Stack) int { return strings.Compare(a.Name, b.Name) })
	return stacks, nil
}

// readAssemblyFile reads a file of the assembly by a plain name: a name that
// leaves the folder is refused.
func readAssemblyFile(dir, name string) ([]byte, error) {
	if name == "" || filepath.IsAbs(name) || name != filepath.Base(name) {
		return nil, fmt.Errorf("the assembly names the file %q", name)
	}
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxAssemblyFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxAssemblyFileBytes {
		return nil, fmt.Errorf("%s is too large", name)
	}
	return data, nil
}

// parseEnvironment reads aws://<account>/<region>; unknown-account and
// unknown-region are empty.
func parseEnvironment(env string) (account, region string) {
	rest, ok := strings.CutPrefix(env, "aws://")
	if !ok {
		return "", ""
	}
	account, region, _ = strings.Cut(rest, "/")
	if account == "unknown-account" {
		account = ""
	}
	if region == "unknown-region" {
		region = ""
	}
	return account, region
}

// readTemplate reads the resources of a template: the type, aws:cdk:path and
// the string tags, as a list of Key and Value or as a map.
func readTemplate(dir, name string) ([]SynthesizedResource, error) {
	data, err := readAssemblyFile(dir, name)
	if err != nil {
		return nil, err
	}
	var template struct {
		Resources map[string]struct {
			Type       string `json:"Type"`
			Properties struct {
				Tags json.RawMessage `json:"Tags"`
			} `json:"Properties"`
			Metadata struct {
				Path string `json:"aws:cdk:path"`
			} `json:"Metadata"`
		} `json:"Resources"`
	}
	if err := json.Unmarshal(data, &template); err != nil {
		return nil, err
	}
	resources := make([]SynthesizedResource, 0, len(template.Resources))
	for id, r := range template.Resources {
		resources = append(resources, SynthesizedResource{LogicalID: id, Type: r.Type, ConstructPath: r.Metadata.Path,
			Tags: readTags(r.Properties.Tags)})
	}
	slices.SortFunc(resources, func(a, b SynthesizedResource) int { return strings.Compare(a.LogicalID, b.LogicalID) })
	return resources, nil
}

func readTags(raw json.RawMessage) map[string]string {
	tags := map[string]string{}
	var list []struct {
		Key   any `json:"Key"`
		Value any `json:"Value"`
	}
	if json.Unmarshal(raw, &list) == nil {
		for _, t := range list {
			k, kok := t.Key.(string)
			v, vok := t.Value.(string)
			if kok && vok {
				tags[k] = v
			}
		}
		return tags
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) == nil {
		for k, v := range m {
			if s, ok := v.(string); ok {
				tags[k] = s
			}
		}
	}
	return tags
}
