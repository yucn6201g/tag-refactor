package cdk

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The trace of cdk synth --debug (v0.2.0 U3 BR4.2, security design §5). With
// --debug, the CDK keeps for each construct the stack of the JavaScript (or
// Python) frames that made it, as entries of type aws:cdk:creationStack. The
// first check of the code generation found them in the metadata file of each
// stack artifact (additionalMetadataFile, by construct path with a leading /);
// the metadata of the artifact in the manifest is read too, for the older
// form, and the field trace of an entry besides its data. A line is
// "<name> (<file>:<line>:<column>)", with or without "at ", or the place
// alone. For each construct path the first frame in a source of the user is
// used: not outside the app, not in node_modules, .venv, venv or
// site-packages. The absolute paths stay in this file: a TraceSite holds the
// file relative to the app.

// creationStack is the type of the metadata entries of the trace.
const creationStack = "aws:cdk:creationStack"

// frameEnd reads <file>:<line> and an optional :<column> at the end of a frame.
var frameEnd = regexp.MustCompile(`^(.*?):(\d+)(?::\d+)?$`)

// traceEntry is one entry of the metadata of a construct.
type traceEntry struct {
	Type  string          `json:"type"`
	Data  json.RawMessage `json:"data"`
	Trace []string        `json:"trace"`
}

// lines returns the lines of the trace an entry holds.
func (e traceEntry) lines() []string {
	var data []string
	if e.Type == creationStack {
		_ = json.Unmarshal(e.Data, &data)
	}
	return append(data, e.Trace...)
}

// readTraceSites reads the trace of the cloud assembly in outDir and returns,
// by construct path, the first place in the user's sources, in the order of
// the paths, with the number of lines left out.
func readTraceSites(outDir, appAbs string, windows bool) (sites []TraceSite, dropped int, err error) {
	data, err := readAssemblyFile(outDir, "manifest.json")
	if err != nil {
		return nil, 0, err
	}
	var manifest struct {
		Artifacts map[string]struct {
			Type                   string                  `json:"type"`
			AdditionalMetadataFile string                  `json:"additionalMetadataFile"`
			Metadata               map[string][]traceEntry `json:"metadata"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, 0, err
	}
	ids := make([]string, 0, len(manifest.Artifacts))
	for id := range manifest.Artifacts {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	found := map[string]TraceSite{}
	for _, id := range ids {
		a := manifest.Artifacts[id]
		if a.Type != "aws:cloudformation:stack" {
			continue
		}
		groups := []map[string][]traceEntry{a.Metadata}
		if a.AdditionalMetadataFile != "" {
			data, err := readAssemblyFile(outDir, a.AdditionalMetadataFile)
			if err != nil {
				return nil, 0, err
			}
			var more map[string][]traceEntry
			if err := json.Unmarshal(data, &more); err != nil {
				return nil, 0, err
			}
			groups = append(groups, more)
		}
		for _, group := range groups {
			dropped += collectSites(group, appAbs, windows, found)
		}
	}
	for _, s := range found {
		sites = append(sites, s)
	}
	slices.SortFunc(sites, func(a, b TraceSite) int { return strings.Compare(a.ConstructPath, b.ConstructPath) })
	return sites, dropped, nil
}

// collectSites adds to found the first place of the user of each construct
// path of group that has none yet, and returns the lines it left out.
func collectSites(group map[string][]traceEntry, appAbs string, windows bool, found map[string]TraceSite) int {
	paths := make([]string, 0, len(group))
	for p := range group {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	dropped := 0
	for _, key := range paths {
		constructPath := strings.TrimPrefix(key, "/")
		var lines []string
		for _, e := range group[key] {
			lines = append(lines, e.lines()...)
		}
		if _, ok := found[constructPath]; ok {
			dropped += len(lines)
			continue
		}
		used := false
		for _, line := range lines {
			if used {
				dropped++
				continue
			}
			file, n, ok := traceFrame(line, windows)
			if ok && n >= 1 {
				if rel, ok := userFile(file, appAbs, windows); ok {
					found[constructPath] = TraceSite{ConstructPath: constructPath, File: rel, Line: n}
					used = true
					continue
				}
			}
			dropped++
		}
	}
	return dropped
}

// traceFrame takes the absolute file and the line of one frame of a trace.
func traceFrame(line string, windows bool) (file string, lineNo int, ok bool) {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "at ")
	if strings.HasSuffix(s, ")") {
		if i := strings.LastIndex(s, "("); i >= 0 {
			s = s[i+1 : len(s)-1]
		}
	}
	m := frameEnd.FindStringSubmatch(s)
	if m == nil {
		return "", 0, false
	}
	file = m[1]
	if strings.HasPrefix(file, "file://") {
		u, err := url.Parse(file)
		if err != nil {
			return "", 0, false
		}
		file = u.Path
		if windows && len(file) >= 3 && file[0] == '/' && file[2] == ':' {
			file = file[1:]
		}
		file = filepath.FromSlash(file)
	}
	if !filepath.IsAbs(file) {
		return "", 0, false
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, false
	}
	return file, n, true
}

// thirdParty are the folders whose files are not the user's sources.
var thirdParty = []string{"node_modules", ".venv", "venv", "site-packages"}

// userFile returns file relative to the app, with /, when it is in the app and
// not in a folder of thirdParty.
func userFile(file, appAbs string, windows bool) (string, bool) {
	rel, err := filepath.Rel(appAbs, filepath.Clean(file))
	sep := string(filepath.Separator)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+sep) || rel == "." {
		return "", false
	}
	for _, part := range strings.Split(rel, sep) {
		for _, folder := range thirdParty {
			if part == folder || windows && strings.EqualFold(part, folder) {
				return "", false
			}
		}
	}
	return filepath.ToSlash(rel), true
}
