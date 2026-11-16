package cdk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yucn6201g/tag-refactor/internal/procexec"
	"github.com/yucn6201g/tag-refactor/internal/redact"
)

// The Python of a Python app (v0.2.0 U3, security design §2 to §4). The Python
// of the .venv right in the app folder comes first: its folder of programs is
// put first on PATH with VIRTUAL_ENV, and the app command, when it starts
// with python, python3 or py, is given to cdk synth with --app with that
// Python in its place. Without a .venv, the first word of the app command is
// looked for on PATH. Its version is checked before synth. Nothing is made,
// installed or written in a .venv.

// interpreter is the Python of the synth of a Python app (BR2.2).
type interpreter struct {
	// path is the program the version is checked with: the absolute path of
	// the Python of the .venv, or what LookPath found. Empty: not checked.
	path string
	// venv is the absolute path of the .venv right in the app folder, or empty.
	venv string
	// appArg is the command given to cdk synth with --app, or empty.
	appArg string
	// notReplaced is set when the app command starts with a Python word that
	// was not replaced in --app: path is then the Python that word reaches.
	notReplaced bool
}

// minPython is the oldest Python: the lower bound of Requires-Python of the
// aws-cdk-lib pinned in the test app py-minimal (TestMinPythonMatchesTheTestApp).
var minPython = [2]int{3, 10}

// MinPythonVersion is the oldest Python, as the messages write it (NFR6.16).
func MinPythonVersion() string {
	return strconv.Itoa(minPython[0]) + "." + strconv.Itoa(minPython[1])
}

// pythonVersionLimit is the limit of one python --version (NFR3.14).
const pythonVersionLimit = 30 * time.Second

// pythonVersion finds "Python <major>.<minor>" at the start of a line.
var pythonVersion = regexp.MustCompile(`(?m)^Python (\d+)\.(\d+)`)

// windowsQuotedApp says whether, on Windows, a path with a space is given to
// --app in double quotes (security design §4). The first check of the code
// generation found that the quotes do not reach cdk.cmd as they were given
// (TestQuotedArgReachesCmd), so iac does not give --app on Windows when the
// path would need them, and leans on the .venv first on PATH alone.
const windowsQuotedApp = false

// venvPython returns the Python of the .venv right in the app folder.
func venvPython(appAbs string, windows bool) (python, venv string, ok bool) {
	venv = filepath.Join(appAbs, ".venv")
	python = filepath.Join(venv, "bin", "python")
	if windows {
		python = filepath.Join(venv, "Scripts", "python.exe")
	}
	info, err := os.Stat(python)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", false
	}
	return python, venv, true
}

// venvProgram returns the program named word in the folder of the programs of
// the .venv, or empty when there is none.
func venvProgram(venv, word string, windows bool) string {
	dir, names := filepath.Join(venv, "bin"), []string{word}
	if windows {
		dir = filepath.Join(venv, "Scripts")
		if !strings.HasSuffix(strings.ToLower(word), ".exe") {
			names = []string{word + ".exe"}
		}
	}
	for _, name := range names {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

// firstWord returns the first word of the app command, without the spaces
// before it, and the rest after it, with the spaces that part them.
func firstWord(command string) (word, rest string) {
	command = strings.TrimLeftFunc(command, unicode.IsSpace)
	i := strings.IndexFunc(command, unicode.IsSpace)
	if i < 0 {
		return command, ""
	}
	return command[:i], command[i:]
}

// isPythonWord reports python, python3 or py, without .exe, in any case.
func isPythonWord(word string) bool {
	w := strings.ToLower(word)
	w = strings.TrimSuffix(w, ".exe")
	return w == "python" || w == "python3" || w == "py"
}

// maxToolName is the longest name of a tool shown, in characters.
const maxToolName = 64

// toolName makes the name of a tool to show from the first word of the app
// command: its last part only (an absolute path in cdk.json does not show the
// home folder), with the characters that cannot be shown replaced, and cut
// at 64 characters. An empty name, ., / or \ is python.
func toolName(word string) string {
	name := filepath.Base(strings.ReplaceAll(word, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if r == utf8.RuneError || !unicode.IsPrint(r) {
			return '?'
		}
		return r
	}, name)
	if utf8.RuneCountInString(name) > maxToolName {
		name = string([]rune(name)[:maxToolName])
	}
	switch name {
	case "", ".", "/", `\`:
		return "python"
	}
	return name
}

// pythonFor decides the Python of a Python app (BR2.2, NFR2.6): the .venv of
// the app folder, else the first word of the app command on PATH.
func (s *Synthesizer) pythonFor(app App) (interpreter, *Failure) {
	windows := runtime.GOOS == "windows"
	if python, venv, ok := venvPython(app.Abs, windows); ok {
		in := interpreter{path: python, venv: venv}
		if arg, ok := appCommand(app.Command, python, windows); ok {
			in.appArg = arg
			return in, nil
		}
		// Not replaced: the shell of cdk resolves the first word, with the
		// folder of the programs of the .venv first on PATH. Check that one.
		if word, _ := firstWord(app.Command); isPythonWord(word) {
			in.notReplaced = true
			if own := venvProgram(venv, word, windows); own != "" {
				in.path = own
			} else if found, err := s.cfg.Runner.LookPath(word); err == nil {
				in.path = found
			} else {
				return interpreter{}, &Failure{Reason: ReasonToolMissing, Tool: "python"}
			}
		}
		return in, nil
	}
	word, _ := firstWord(app.Command)
	lookFor := word
	if strings.ContainsAny(word, `/\`) && !filepath.IsAbs(word) && !strings.ContainsAny(word[:1], `/\`) {
		// A relative path is the app's, as cdk synth runs in the app folder.
		lookFor = filepath.Join(app.Abs, filepath.FromSlash(strings.ReplaceAll(word, `\`, "/")))
	}
	found, err := s.cfg.Runner.LookPath(lookFor)
	if word == "" || err != nil {
		tool := "python"
		if !isPythonWord(word) {
			tool = toolName(word)
		}
		return interpreter{}, &Failure{Reason: ReasonToolMissing, Tool: tool}
	}
	if isPythonWord(word) {
		return interpreter{path: found}, nil
	}
	return interpreter{}, nil
}

// withVenv returns the environment vars with the folder of the programs of
// venv first on the one PATH variable and VIRTUAL_ENV set to venv (NFR2.5).
// vars does not change.
func withVenv(vars []string, venv string, windows bool) []string {
	bin := filepath.Join(venv, "bin")
	if windows {
		bin = filepath.Join(venv, "Scripts")
	}
	same := func(a, b string) bool {
		if windows {
			return strings.EqualFold(a, b)
		}
		return a == b
	}
	out := make([]string, 0, len(vars)+2)
	pathName, pathValue, found := "PATH", "", false
	for _, kv := range vars {
		name, value, _ := strings.Cut(kv, "=")
		switch {
		case same(name, "PATH"):
			if !found {
				pathName, pathValue, found = name, value, true
			}
		case same(name, "VIRTUAL_ENV"):
		default:
			out = append(out, kv)
		}
	}
	newPath := bin
	if pathValue != "" {
		first, _, _ := strings.Cut(pathValue, string(os.PathListSeparator))
		if same(filepath.Clean(first), filepath.Clean(bin)) {
			newPath = pathValue
		} else {
			newPath = bin + string(os.PathListSeparator) + pathValue
		}
	}
	return append(out, pathName+"="+newPath, "VIRTUAL_ENV="+venv)
}

func (s *Synthesizer) versionTimeout() time.Duration {
	if s.VersionTimeout > 0 {
		return s.VersionTimeout
	}
	return pythonVersionLimit
}

// maxVersionLine is the longest answer of python --version kept for --debug,
// in characters.
const maxVersionLine = 200

// checkPython asks in.path its version and refuses one older than minPython
// (NFR6.16, NFR3.14). A failure is the Failure; a stop is the error of the
// context.
func (s *Synthesizer) checkPython(ctx context.Context, app App, in interpreter, env []string, debug *[]string) (*Failure, error) {
	windows := runtime.GOOS == "windows"
	shown := shortHome(in.path, s.homes, windows)
	res, err := s.cfg.Runner.Run(ctx, procexec.Command{Name: in.path, Args: []string{"--version"}, Dir: app.Abs, Env: env,
		Timeout: s.versionTimeout(), StdoutLimit: 4 << 10, StderrLimit: 4 << 10})
	missing := &Failure{Reason: ReasonToolMissing, Tool: "python"}
	switch {
	case errors.Is(err, procexec.ErrTimeout):
		return &Failure{Reason: ReasonTimeout, Tool: "python"}, nil
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case err != nil || res.ExitCode != 0:
		return missing, nil
	}
	m := pythonVersion.FindStringSubmatch(string(res.Stdout))
	if m == nil {
		m = pythonVersion.FindStringSubmatch(string(res.Stderr))
	}
	if m == nil {
		answer := strings.TrimSpace(string(res.Stdout) + string(res.Stderr))
		answer, _, _ = strings.Cut(answer, "\n")
		answer = strings.TrimSpace(answer)
		if utf8.RuneCountInString(answer) > maxVersionLine {
			answer = string([]rune(answer)[:maxVersionLine])
		}
		*debug = append(*debug, "cdk: "+app.Dir+": python: "+shown+" answered "+redact.String(shortHome(answer, s.homes, windows)))
		return missing, nil
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major < minPython[0] || major == minPython[0] && minor < minPython[1] {
		*debug = append(*debug, "cdk: "+app.Dir+": python: "+shown+" is older than "+MinPythonVersion())
		return &Failure{Reason: ReasonToolTooOld, Tool: "python"}, nil
	}
	*debug = append(*debug, fmt.Sprintf("cdk: %s: python: %s (Python %d.%d)", app.Dir, shown, major, minor))
	return nil, nil
}

// The characters a shell reads specially even inside double quotes, and those
// it reads specially outside quotes, for appCommand (security design §4).
const (
	unixInQuotes     = "\"$`\\"
	unixOutQuotes    = "'&;|<>()*?[#~!{}"
	windowsInQuotes  = "\"%"
	windowsOutQuotes = "&|<>^()"
)

// appCommand returns the app command with its first word replaced by python,
// the absolute path of the Python of the .venv, quoted when it needs to be;
// "", false when it is not replaced (security design §4, NFR6.17).
func appCommand(command, python string, windows bool) (string, bool) {
	word, rest := firstWord(command)
	if !isPythonWord(word) {
		return "", false
	}
	inQuotes, outQuotes := unixInQuotes, unixOutQuotes
	if windows {
		inQuotes, outQuotes = windowsInQuotes, windowsOutQuotes
	}
	if strings.ContainsAny(python, inQuotes) {
		return "", false
	}
	quote := strings.IndexFunc(python, unicode.IsSpace) >= 0 || strings.ContainsAny(python, outQuotes)
	switch {
	case quote && windows && !windowsQuotedApp:
		return "", false
	case quote:
		return `"` + python + `"` + rest, true
	}
	return python + rest, true
}
