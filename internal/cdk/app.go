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
	"regexp"
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

// The folders Discover never enters (BR1.1, U3 NFR3.15): the names are
// compared as they are.
var skippedFolders = map[string]bool{".git": true, "node_modules": true, "cdk.out": true, ".venv": true, "venv": true}

// readDir reads a folder for Discover; the tests count the folders it reads.
var readDir = os.ReadDir

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
	entries, err := readDir(dir)
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
	switch a.Language {
	case TypeScript:
		a.CDKMajor = cdkMajor(root, dir)
	case Python:
		a.CDKMajor = pythonMajor(root, dir)
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

// pythonFiles are the files that declare the dependencies of a Python app
// (U3 BR1.3, contract C2).
var pythonFiles = []string{"requirements.txt", "setup.py", "pyproject.toml"}

// pythonMajor reads the dependencies of a Python app (U3 BR1.3): from the app
// folder up to root, the first folder with one of pythonFiles decides, with
// every one of them it has. aws-cdk-lib is 2, aws-cdk-core (aws-cdk.core) is
// 1, nothing is 0. No file is run.
func pythonMajor(root, dir string) int {
	for {
		var names map[string]bool
		for _, name := range pythonFiles {
			data, err := readLimited(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			if names == nil {
				names = map[string]bool{}
			}
			for _, n := range dependencyNames(name, string(data)) {
				names[n] = true
			}
		}
		switch {
		case names["aws-cdk-lib"]:
			return 2
		case names["aws-cdk-core"]:
			return 1
		case names != nil:
			return 0
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

var (
	// leadingName is the name at the start of a requirement: letters, digits,
	// dots, underscores and hyphens, before extras, versions, markers or a URL.
	leadingName = regexp.MustCompile(`^[A-Za-z0-9._-]+`)
	// quoted is a string in single or double quotes.
	quoted = regexp.MustCompile(`"([^"\n]*)"|'([^'\n]*)'`)
	// separators are the runs that normalized names write as one hyphen.
	separators = regexp.MustCompile(`[-_.]+`)
)

// dependencyNames reads the names of the dependencies of one file, normalized
// (BR1.3 table).
func dependencyNames(file, content string) []string {
	var names []string
	add := func(requirement string) {
		if n, ok := requirementName(requirement); ok {
			names = append(names, n)
		}
	}
	// pyproject.toml: quoted strings are read in the dependencies array of
	// [project] and in the dependency tables only (R-09).
	section, inArray := "", false
	for _, line := range strings.Split(content, "\n") {
		switch file {
		case "requirements.txt":
			line, _, _ = strings.Cut(line, "#")
			add(line)
		case "setup.py":
			for _, s := range quotedStrings(line) {
				add(s)
			}
		default: // pyproject.toml
			line = withoutComment(line)
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "[") && !inArray {
				section = strings.ToLower(strings.Trim(trimmed, "[] "))
				continue
			}
			key, value, hasKey := strings.Cut(trimmed, "=")
			key = strings.Trim(strings.TrimSpace(key), `"'`)
			switch {
			case section == "project":
				if !inArray && hasKey && key == "dependencies" {
					inArray = true
					trimmed = value
				}
				if inArray {
					for _, s := range quotedStrings(trimmed) {
						add(s)
					}
					inArray = !strings.Contains(quoted.ReplaceAllString(trimmed, ""), "]")
				}
			case section == "project.optional-dependencies" || section == "dependency-groups":
				// The keys are group names; the strings are requirements.
				for _, s := range quotedStrings(trimmed) {
					add(s)
				}
			case strings.HasSuffix(section, "dependencies") && hasKey:
				// [tool.poetry.dependencies] and the like: the keys are names.
				add(key)
			}
		}
	}
	return names
}

// requirementName takes the name of one requirement, or nothing for an empty
// line, an option (-r, -e, --index-url), a URL or a path. The URL of a
// "name @ URL" requirement comes after the name, which is read.
func requirementName(requirement string) (string, bool) {
	r := strings.TrimSpace(requirement)
	beforeAt, _, _ := strings.Cut(r, "@")
	if r == "" || strings.HasPrefix(r, "-") || strings.Contains(beforeAt, "://") || strings.HasPrefix(r, ".") || strings.HasPrefix(r, "/") {
		return "", false
	}
	name := leadingName.FindString(r)
	if name == "" {
		return "", false
	}
	return separators.ReplaceAllString(strings.ToLower(name), "-"), true
}

func quotedStrings(line string) []string {
	var out []string
	for _, m := range quoted.FindAllStringSubmatch(line, -1) {
		out = append(out, m[1]+m[2])
	}
	return out
}

// withoutComment cuts a line of TOML at a # outside quotes.
func withoutComment(line string) string {
	var quote rune
	for i, r := range line {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '"' || r == '\''):
			quote = r
		case quote == 0 && r == '#':
			return line[:i]
		}
	}
	return line
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
