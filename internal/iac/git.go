package iac

import (
	"bytes"
	"context"
	"crypto/sha1" // git names its objects with SHA-1 here; this is not for security
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// The check before --write (BR5.2, security design §6). git status is not used:
// it can run the fsmonitor and the clean filters of a repository. Only
// rev-parse, ls-files and ls-tree are called, and this tool compares the hashes
// itself. The user's git itself is trusted.

// The limits of one call of git.
var (
	gitTimeout   = 60 * time.Second
	gitWaitDelay = 5 * time.Second
)

// maxGitArgsLength keeps a command line far from the limit of Windows.
const maxGitArgsLength = 30000

// The functions that reach git; the tests stand in for them.
var (
	lookGit = exec.LookPath
	runGit  = runGitCommand
)

// runGitCommand runs git with no shell, with a time limit on the process and on
// waiting for its output.
func runGitCommand(ctx context.Context, git string, env, args []string) (stdout, stderr []byte, err error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, git, args...)
	cmd.Env = env
	cmd.WaitDelay = gitWaitDelay
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err = cmd.Run()
	return out.Bytes(), errOut.Bytes(), err
}

// gitEnv is the environment of git: without any GIT_ variable, which could point
// it at another repository (on Windows names have no case), and with what keeps
// it from writing the index, from reading paths as patterns and from asking.
func gitEnv(environ []string, windows bool) []string {
	env := make([]string, 0, len(environ)+3)
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if windows {
			name = strings.ToUpper(name)
		}
		if !strings.HasPrefix(name, "GIT_") {
			env = append(env, kv)
		}
	}
	return append(env, "GIT_OPTIONAL_LOCKS=0", "GIT_LITERAL_PATHSPECS=1", "GIT_TERMINAL_PROMPT=0")
}

// gitFile is a file --write wants to change, with the hashes of its content as
// it was read.
type gitFile struct {
	rel string
	// sha1, sha256 and their LF forms are the blob hashes of the content, and of
	// the content with CRLF made LF, which is what core.autocrlf commits.
	blobSHA1, blobSHA256, lfSHA1, lfSHA256 string
}

// blobHashes works out the hashes of a file for the check.
func blobHashes(rel string, raw []byte) gitFile {
	f := gitFile{rel: rel, blobSHA1: blobSHA1(raw), blobSHA256: blobSHA256(raw)}
	if bytes.Contains(raw, []byte("\r\n")) {
		lf := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
		f.lfSHA1, f.lfSHA256 = blobSHA1(lf), blobSHA256(lf)
	}
	return f
}

func blobHeader(raw []byte) string { return fmt.Sprintf("blob %d\x00", len(raw)) }

func blobSHA1(raw []byte) string {
	h := sha1.New()
	h.Write([]byte(blobHeader(raw)))
	h.Write(raw)
	return hex.EncodeToString(h.Sum(nil))
}

func blobSHA256(raw []byte) string {
	h := sha256.New()
	h.Write([]byte(blobHeader(raw)))
	h.Write(raw)
	return hex.EncodeToString(h.Sum(nil))
}

// gitChecker runs the check for one run.
type gitChecker struct {
	ctx  context.Context
	root string
	git  string
	env  []string
	// debug collects what git said on standard error.
	debug *[]string
}

func gitStop(detailID string, args ...any) error {
	return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, DetailID: detailID, DetailArgs: args, HintID: "iac.gitFix"}
}

// checkGit checks that every file is committed and unchanged. Any doubt stops
// the writing.
func checkGit(ctx context.Context, root string, files []gitFile, debug *[]string) error {
	git, err := lookGit("git")
	if err != nil {
		return gitStop("iac.gitMissing")
	}
	c := &gitChecker{ctx: ctx, root: root, git: git, env: gitEnv(os.Environ(), runtime.GOOS == "windows"), debug: debug}
	out, _, err := c.run("rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return gitStop("iac.notWorkTree", root)
	}
	out, _, err = c.run("rev-parse", "--show-object-format")
	format := strings.TrimSpace(string(out))
	if err != nil || format != "sha1" && format != "sha256" {
		return gitStop("iac.gitFailed", "rev-parse")
	}
	index, err := c.listIndex(files)
	if err != nil {
		return err
	}
	tree, err := c.listTree(files)
	if err != nil {
		return err
	}
	for i := range files {
		if err := checkFile(&files[i], format, index, tree); err != nil {
			return err
		}
	}
	return nil
}

// run calls git in the root.
func (c *gitChecker) run(args ...string) ([]byte, int, error) {
	full := append([]string{"-c", "core.fsmonitor=false", "-C", c.root}, args...)
	stdout, stderr, err := runGit(c.ctx, c.git, c.env, full)
	if err != nil {
		*c.debug = append(*c.debug, "git "+args[0]+": "+strings.TrimSpace(string(stderr)))
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, exit.ExitCode(), err
		}
		return nil, -1, err
	}
	return stdout, 0, nil
}

// batches splits the files so that no command line gets too long.
func (c *gitChecker) batches(files []gitFile) [][]string {
	var out [][]string
	var batch []string
	fixed := len(c.root) + 64
	length := fixed
	for i := range files {
		rel := files[i].rel
		if len(batch) > 0 && length+len(rel)+3 > maxGitArgsLength {
			out = append(out, batch)
			batch, length = nil, fixed
		}
		batch = append(batch, rel)
		length += len(rel) + 3
	}
	if len(batch) > 0 {
		out = append(out, batch)
	}
	return out
}

// indexEntry is one line of ls-files -s.
type indexEntry struct {
	mode, hash, stage string
}

// listIndex reads ls-files -s for the files. A path not in the answer gets one
// more question of its own, because git can answer with a path in another
// Unicode form than the one asked for (macOS): it knows its own setting.
func (c *gitChecker) listIndex(files []gitFile) (map[string]indexEntry, error) {
	index := map[string]indexEntry{}
	for _, batch := range c.batches(files) {
		out, _, err := c.run(append([]string{"ls-files", "-s", "-z", "--"}, batch...)...)
		if err != nil {
			return nil, gitStop("iac.gitFailed", "ls-files")
		}
		parseIndex(out, index)
	}
	for i := range files {
		rel := files[i].rel
		if _, ok := index[rel]; ok {
			continue
		}
		out, _, err := c.run("ls-files", "-s", "-z", "--", rel)
		if err != nil {
			return nil, gitStop("iac.gitFailed", "ls-files")
		}
		alone := map[string]indexEntry{}
		parseIndex(out, alone)
		if len(alone) == 1 {
			for _, e := range alone {
				index[rel] = e
			}
		}
	}
	return index, nil
}

// parseIndex reads "<mode> <hash> <stage>\t<path>\0" records. A path in conflict
// has several stages; one of them that is not 0 is enough to stop.
func parseIndex(out []byte, index map[string]indexEntry) {
	for _, rec := range bytes.Split(out, []byte{0}) {
		meta, path, ok := bytes.Cut(rec, []byte{'\t'})
		if !ok {
			continue
		}
		fields := strings.Fields(string(meta))
		if len(fields) != 3 {
			continue
		}
		if have, seen := index[string(path)]; seen && have.stage != "0" {
			continue
		}
		index[string(path)] = indexEntry{mode: fields[0], hash: fields[1], stage: fields[2]}
	}
}

// listTree reads ls-tree HEAD for the files: path to hash. A repository without
// a commit makes git fail here.
func (c *gitChecker) listTree(files []gitFile) (map[string]string, error) {
	tree := map[string]string{}
	for _, batch := range c.batches(files) {
		out, code, err := c.run(append([]string{"ls-tree", "-z", "HEAD", "--"}, batch...)...)
		if err != nil {
			if code > 0 {
				return nil, gitStop("iac.noCommit")
			}
			return nil, gitStop("iac.gitFailed", "ls-tree")
		}
		for _, rec := range bytes.Split(out, []byte{0}) {
			meta, path, ok := bytes.Cut(rec, []byte{'\t'})
			if !ok {
				continue
			}
			if fields := strings.Fields(string(meta)); len(fields) == 3 {
				tree[string(path)] = fields[2]
			}
		}
	}
	return tree, nil
}

// checkFile compares one file: it is tracked and regular, the index holds what
// HEAD holds, and the file holds what the index holds.
func checkFile(f *gitFile, format string, index map[string]indexEntry, tree map[string]string) error {
	e, ok := index[f.rel]
	if !ok {
		return gitStop("iac.notTracked", f.rel)
	}
	if e.stage != "0" {
		return gitStop("iac.conflict", f.rel)
	}
	if e.mode != "100644" && e.mode != "100755" {
		return gitStop("iac.notRegular", f.rel)
	}
	head, ok := tree[f.rel]
	if !ok {
		return gitStop("iac.notCommitted", f.rel)
	}
	if head != e.hash {
		return gitStop("iac.notCommitted", f.rel)
	}
	blob, lf := f.blobSHA1, f.lfSHA1
	if format == "sha256" {
		blob, lf = f.blobSHA256, f.lfSHA256
	}
	if blob != e.hash && (lf == "" || lf != e.hash) {
		return gitStop("iac.notCommitted", f.rel)
	}
	return nil
}
