package cdk

// Unit tests of the discovery of v0.2.0 U3 (BR1.3, NFR3.15, security design
// §7), written after the implementation.

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// NFR3.15: Discover never reads below .venv or venv, whatever they hold.
func TestCDKDiscoverNeverEntersAVenv(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"infra/py/cdk.json":                        `{"app": "python3 app.py"}`,
		"infra/py/requirements.txt":                "aws-cdk-lib==2.276.0\n",
		"infra/py/.venv/lib/site/cdk.json":         `{"app": "python3 app.py"}`,
		"infra/py/.venv/Scripts/python.exe":        "",
		"infra/py/venv/deep/deeper/cdk.json":       `{"app": "node x.js"}`,
		"infra/ts/cdk.json":                        `{"app": "node bin/app.js"}`,
		"infra/ts/.Venv-not-skipped/x/placeholder": "",
	})
	var mu sync.Mutex
	var read []string
	readDir = func(name string) ([]os.DirEntry, error) {
		mu.Lock()
		read = append(read, name)
		mu.Unlock()
		return os.ReadDir(name)
	}
	t.Cleanup(func() { readDir = os.ReadDir })

	apps, err := NewSynthesizer(Config{Runner: &fakeRunner{}}).Discover(context.Background(), root, nil)

	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, a := range apps {
		dirs = append(dirs, a.Dir)
	}
	if !slices.Equal(dirs, []string{"infra/py", "infra/ts"}) {
		t.Fatalf("apps %v", dirs)
	}
	sawOther := false
	for _, p := range read {
		for _, part := range strings.Split(filepath.ToSlash(p), "/") {
			if part == ".venv" || part == "venv" {
				t.Errorf("read %s", p)
			}
		}
		sawOther = sawOther || strings.Contains(p, ".Venv-not-skipped")
	}
	if !sawOther {
		t.Errorf("a folder of another name was not read: %v", read)
	}
	if apps[0].Language != Python || apps[0].CDKMajor != 2 {
		t.Errorf("app %+v", apps[0])
	}
}

// BR1.3 table: the names of the dependencies of the three Python files.
func TestCDKPythonDependencyNames(t *testing.T) {
	for _, tc := range []struct {
		file, content string
		want          []string
	}{
		{"requirements.txt", "-r base.txt\n-e .\n--index-url https://x\naws-cdk-lib==2.276.0\n", []string{"aws-cdk-lib"}},
		{"requirements.txt", "git+https://example.com/aws-cdk-lib.git\ngit+https://u@example.com/x.git\n./local\n/abs/pkg\n", nil},
		{"requirements.txt", "aws-cdk-lib[extra]>=2.0\n", []string{"aws-cdk-lib"}},
		{"requirements.txt", "aws_cdk_lib ; python_version >= '3.10'\n", []string{"aws-cdk-lib"}},
		{"requirements.txt", "aws-cdk-lib @ https://example.com/a.whl\n", []string{"aws-cdk-lib"}},
		{"requirements.txt", "aws_cdk.core==1.204.0  # v1\n# aws-cdk-lib\n\n", []string{"aws-cdk-core"}},
		{"setup.py", "setup(install_requires=[\"aws-cdk-lib==2.1.0\", 'constructs>=10'])\n", []string{"aws-cdk-lib", "constructs"}},
		{"pyproject.toml", "[project]\nname = \"app\"\ndependencies = [\"aws-cdk-lib>=2\"]  # \"aws-cdk.core\"\n", []string{"aws-cdk-lib"}},
		{"pyproject.toml", "[project]\ndependencies = [\n  \"aws-cdk-lib[x]>=2\",\n  \"constructs\",\n]\nclassifiers = [\"aws-cdk.core\"]\n", []string{"aws-cdk-lib", "constructs"}},
		{"pyproject.toml", "[project.optional-dependencies]\ndev = [\"pytest\"]\n", []string{"pytest"}},
		{"pyproject.toml", "[tool.x]\nx = \"aws-cdk-lib\"\n[project]\nname = \"aws-cdk-lib\"\nkeywords = [\"aws-cdk-lib\"]\n", nil},
		{"pyproject.toml", "[tool.poetry.dependencies]\npython = \"^3.10\"\naws-cdk-lib = \"^2.0\"\n[tool.other]\nx = 1\n", []string{"python", "aws-cdk-lib"}},
	} {
		if got := dependencyNames(tc.file, tc.content); !slices.Equal(got, tc.want) {
			t.Errorf("dependencyNames(%s, %q) = %q, want %q", tc.file, tc.content, got, tc.want)
		}
	}
}

// BR1.3: the first folder from the app up to --dir with a Python file decides,
// with every Python file it has; nothing above --dir is read.
func TestCDKPythonMajor(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"requirements.txt":        "aws-cdk-lib==2.0.0\n",
		"inherit/app/cdk.json":    "{}",
		"v1/setup.py":             `install_requires=["aws-cdk.core"]`,
		"both/requirements.txt":   "constructs\n",
		"both/pyproject.toml":     "[project]\ndependencies = [\"aws-cdk-lib\"]\n",
		"none/requirements.txt":   "constructs==10.0.0\n",
		"none/sub/cdk.json":       "{}",
		"top/app/requirements.tx": "",
	})
	for dir, want := range map[string]int{"inherit/app": 2, "v1": 1, "both": 2, "none/sub": 0} {
		if got := pythonMajor(root, filepath.Join(root, filepath.FromSlash(dir))); got != want {
			t.Errorf("pythonMajor(%s) = %d, want %d", dir, got, want)
		}
	}
	top := filepath.Join(root, "top")
	if got := pythonMajor(top, filepath.Join(top, "app")); got != 0 {
		t.Errorf("the search went above --dir: %d", got)
	}
}

// NFR3.15: cdk.json and the files of the dependencies are read up to 1 MiB.
func TestCDKConfigFilesOverOneMiB(t *testing.T) {
	root := t.TempDir()
	padding := strings.Repeat("# padding\n", maxConfigBytes/10+1)
	writeFiles(t, root, map[string]string{
		"py/cdk.json":         `{"app": "python3 app.py"}`,
		"py/requirements.txt": padding + "aws-cdk-lib==2.276.0\n",
		"big/cdk.json":        `{"app": "node app.js", "context": "` + strings.Repeat("x", maxConfigBytes) + `"}`,
		"big/package.json":    `{"dependencies": {"aws-cdk-lib": "2.0.0"}}`,
	})

	apps, err := NewSynthesizer(Config{Runner: &fakeRunner{}}).Discover(context.Background(), root, nil)

	if err != nil || len(apps) != 2 {
		t.Fatalf("apps %+v, %v", apps, err)
	}
	if apps[0].Dir != "big" || apps[0].Command != "" || apps[0].Language != Other {
		t.Errorf("a cdk.json over 1 MiB was read: %+v", apps[0])
	}
	if apps[1].Dir != "py" || apps[1].CDKMajor != 0 {
		t.Errorf("the requirements after 1 MiB were read: %+v", apps[1])
	}
	if _, err := readLimited(root); err == nil {
		t.Error("a folder was read as a file")
	}
}
