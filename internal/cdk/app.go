// Package cdk synthesizes the CDK apps below --dir and reads their sources, so
// that iac can find where a resource of a plan is made and tagged (v0.2.0 U1,
// contracts C2 to C6). Synthesizer finds the apps and runs cdk synth on them;
// Analyzer runs the helper embedded in this package, which reads the
// TypeScript sources with the app's own TypeScript and answers in JSON.
//
// Both run their programs through procexec.Runner with an environment that
// carries no AWS and no GitHub credentials (BR1.3, security design §2), and
// clean up every temporary file and folder they make, also when they are
// stopped (NFR4.3).
package cdk

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Language is the language of a CDK app (contract C2).
type Language string

// The languages of the apps.
const (
	TypeScript Language = "typescript"
	Python     Language = "python"
	Other      Language = "other"
)

// App is a folder with a cdk.json (CdkApp).
type App struct {
	// Dir is the folder relative to --dir, with /; "." is --dir itself.
	Dir string
	// Abs is the folder as it is on disk.
	Abs string
	// Language comes from the app command of cdk.json (BR1.2).
	Language Language
	// CDKMajor is 2 for aws-cdk-lib, 1 for @aws-cdk/core, 0 when not known.
	CDKMajor int
	// Command is the app command of cdk.json.
	Command string
}

// The folders Discover never enters (BR1.1).
var skippedFolders = map[string]bool{".git": true, "node_modules": true, "cdk.out": true}

// maxDepth is the deepest folder Discover looks into, from --dir, as iac.
const maxDepth = 32

// maxConfigBytes bounds what is read of cdk.json and package.json.
const maxConfigBytes = 1 << 20

// Discover finds the CDK apps below root, in name order (contract C2). It goes
// into plain folders only: not into links, junctions or the folders of
// skippedFolders. When only is not empty, the apps whose Dir is not in it are
// left out.
func (s *Synthesizer) Discover(ctx context.Context, root string, only []string) ([]App, error) {
	var apps []App
	err := discover(ctx, root, root, ".", 0, &apps)
	if err != nil {
		return nil, err
	}
	if len(only) > 0 {
		apps = slices.DeleteFunc(apps, func(a App) bool { return !slices.Contains(only, a.Dir) })
	}
	slices.SortFunc(apps, func(a, b App) int { return strings.Compare(a.Dir, b.Dir) })
	return apps, nil
}

func discover(ctx context.Context, root, dir, rel string, depth int, apps *[]App) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		// A folder that cannot be read holds no app this tool can use.
		return nil
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		switch {
		case info.Mode().IsRegular() && e.Name() == "cdk.json":
			*apps = append(*apps, newApp(root, dir, rel))
		case info.Mode().Type() == fs.ModeDir && !skippedFolders[e.Name()] && depth+1 <= maxDepth:
			if err := discover(ctx, root, filepath.Join(dir, e.Name()), path.Join(rel, e.Name()), depth+1, apps); err != nil {
				return err
			}
		}
	}
	return nil
}

// newApp reads cdk.json and decides the language and the version of CDK.
func newApp(root, dir, rel string) App {
	a := App{Dir: rel, Abs: dir, Language: Other}
	var config struct {
		App string `json:"app"`
	}
	if data, err := readLimited(filepath.Join(dir, "cdk.json")); err == nil && json.Unmarshal(data, &config) == nil {
		a.Command = config.App
	}
	a.Language = languageOf(a.Command)
	if a.Language == TypeScript {
		a.CDKMajor = cdkMajor(root, dir)
	}
	return a
}

// languageOf decides the language from the app command (contract C2
// detection): TypeScript for node, npx, ts-node and tsx or a .ts or .js file,
// Python for python, python3 and py or a .py file, other otherwise.
func languageOf(command string) Language {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return Other
	}
	first := strings.ToLower(filepath.Base(fields[0]))
	for _, ext := range []string{".exe", ".cmd"} {
		first = strings.TrimSuffix(first, ext)
	}
	switch first {
	case "node", "npx", "ts-node", "tsx":
		return TypeScript
	case "python", "python3", "py":
		return Python
	}
	for _, f := range fields {
		switch strings.ToLower(path.Ext(f)) {
		case ".ts", ".js", ".mjs", ".cjs":
			return TypeScript
		case ".py":
			return Python
		}
	}
	return Other
}

// cdkMajor reads the first package.json from the app folder up to root.
func cdkMajor(root, dir string) int {
	for {
		data, err := readLimited(filepath.Join(dir, "package.json"))
		if err == nil {
			return majorOf(data)
		}
		if samePath(dir, root) {
			return 0
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return 0
		}
		dir = parent
	}
}

func majorOf(packageJSON []byte) int {
	var p struct {
		Dependencies    map[string]any `json:"dependencies"`
		DevDependencies map[string]any `json:"devDependencies"`
	}
	if json.Unmarshal(packageJSON, &p) != nil {
		return 0
	}
	has := func(name string) bool {
		_, a := p.Dependencies[name]
		_, b := p.DevDependencies[name]
		return a || b
	}
	switch {
	case has("aws-cdk-lib"):
		return 2
	case has("@aws-cdk/core"):
		return 1
	}
	return 0
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// readLimited reads a regular file up to maxConfigBytes.
func readLimited(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fs.ErrInvalid
	}
	return io.ReadAll(io.LimitReader(f, maxConfigBytes))
}
