package iac

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

// The limits of the files of --dir (NFR3.19). They are variables so that the
// tests do not have to make files of these sizes.
var (
	maxCandidates       = 20000
	maxFileBytes  int64 = 10 << 20
	maxTotalBytes int64 = 500 << 20
)

// maxFolderDepth is the deepest folder iac looks into, from --dir.
const maxFolderDepth = 32

// excludedFolders are never entered (BR3.1): what they hold is made by tools.
var excludedFolders = map[string]bool{".git": true, ".terraform": true, "node_modules": true, "cdk.out": true}

// fileFormat is the format of a candidate file.
type fileFormat int

const (
	formatYAML fileFormat = iota
	formatJSON
	formatTerraform
	// formatTemplate is a .template file: YAML or JSON, told from its content.
	formatTemplate
)

// candidate is a file that can define resources.
type candidate struct {
	// rel is relative to the root with /; path is the path to open.
	rel, path string
	size      int64
	format    fileFormat
}

func (c *candidate) tool() Tool {
	if c.format == formatTerraform {
		return ToolTerraform
	}
	return ToolCloudFormation
}

// walked is what walking the folder finds.
type walked struct {
	candidates []*candidate
	skipped    []SkippedFile
	folders    []string
	// seen counts the candidates, read or not.
	seen int
	// total is the size of the candidates that will be read.
	total int64
}

// walkRoot lists the candidate files below root (BR3.1, security design §2).
// It looks only at plain folders and regular files, so a symbolic link, a
// junction or any other special file is neither read nor followed. wantCF and
// wantTF say which candidates are to be read; all of them count for the limit.
func walkRoot(root string, wantCF, wantTF bool) (*walked, error) {
	w := &walked{}
	if err := w.folder(root, "", 0, wantCF, wantTF); err != nil {
		return nil, err
	}
	slices.SortFunc(w.candidates, func(a, b *candidate) int { return strings.Compare(a.rel, b.rel) })
	slices.Sort(w.folders)
	return w, nil
}

func (w *walked) folder(dir, rel string, depth int, wantCF, wantTF bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if depth == 0 {
			return usageError("--dir", "iac.notFolder", "iac.notFolderFix", dir)
		}
		w.folders = append(w.folders, rel)
		return nil
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		name, childRel := e.Name(), path.Join(rel, e.Name())
		switch info.Mode().Type() {
		case fs.ModeDir:
			if excludedFolders[name] {
				continue
			}
			if depth+1 > maxFolderDepth {
				w.folders = append(w.folders, childRel)
				continue
			}
			if err := w.folder(filepath.Join(dir, name), childRel, depth+1, wantCF, wantTF); err != nil {
				return err
			}
		case 0:
			if err := w.file(filepath.Join(dir, name), childRel, depth, info, wantCF, wantTF); err != nil {
				return err
			}
		}
	}
	return nil
}

// file looks at one regular file.
func (w *walked) file(p, rel string, depth int, info fs.FileInfo, wantCF, wantTF bool) error {
	format, ok := formatOf(rel, depth)
	if !ok {
		return nil
	}
	w.seen++
	if w.seen > maxCandidates {
		return usageError("--dir", "iac.tooManyFiles", "iac.tooManyFilesFix", maxCandidates)
	}
	c := &candidate{rel: rel, path: p, size: info.Size(), format: format}
	if want := c.tool() == ToolTerraform && wantTF || c.tool() == ToolCloudFormation && wantCF; !want {
		return nil
	}
	if c.size > maxFileBytes {
		w.skipped = append(w.skipped, SkippedFile{File: rel, Reason: SkipTooLarge})
		return nil
	}
	w.total += c.size
	if w.total > maxTotalBytes {
		return usageError("--dir", "iac.tooMuchData", "iac.tooManyFilesFix")
	}
	w.candidates = append(w.candidates, c)
	return nil
}

// content is a candidate file ready for its parser.
type content struct {
	// raw is the file as it is; body is raw without a UTF-8 byte order mark,
	// which base counts: a position in body plus base is a position in raw.
	raw, body []byte
	base      int
	format    fileFormat
}

var (
	utf8BOM    = []byte("\xef\xbb\xbf")
	foreignBOM = [][]byte{[]byte("\xff\xfe"), []byte("\xfe\xff"), []byte("\x00\x00\xfe\xff")}
)

// prepare looks at the content of a candidate (security design §2): the skip
// reason of a file that cannot be parsed, and false for a template candidate
// without the word Resources, which needs no parsing and is not reported. The
// nesting check comes last, on the content its parser would read.
func prepare(c *candidate, raw []byte) (*content, SkipReason, bool) {
	for _, bom := range foreignBOM {
		if bytes.HasPrefix(raw, bom) {
			return nil, SkipNotUTF8, true
		}
	}
	if !utf8.Valid(raw) {
		return nil, SkipNotUTF8, true
	}
	ct := &content{raw: raw, body: raw, format: c.format}
	if bytes.HasPrefix(raw, utf8BOM) {
		ct.body, ct.base = raw[len(utf8BOM):], len(utf8BOM)
	}
	if ct.format == formatTemplate {
		ct.format = formatYAML
		if t := bytes.TrimLeft(ct.body, " \t\r\n"); len(t) > 0 && t[0] == '{' {
			ct.format = formatJSON
		}
	}
	if ct.format != formatTerraform && !bytes.Contains(ct.body, []byte("Resources")) {
		return nil, "", false
	}
	if tooDeep(ct.format, ct.body) {
		return nil, SkipTooDeep, true
	}
	return ct, "", true
}

// formatOf tells a candidate by its extension: a .tf counts only directly in
// --dir, the root module (BR3.1).
func formatOf(rel string, depth int) (fileFormat, bool) {
	switch strings.ToLower(path.Ext(rel)) {
	case ".yaml", ".yml":
		return formatYAML, true
	case ".json":
		return formatJSON, true
	case ".template":
		return formatTemplate, true
	case ".tf":
		return formatTerraform, depth == 0
	}
	return 0, false
}
