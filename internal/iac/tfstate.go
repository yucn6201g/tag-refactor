package iac

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// stateIndex lists, for each ARN of the state, the addresses of the resources
// with exactly that ARN (BR2.3).
type stateIndex map[string][]string

// maxTFStateBytes is the largest --tf-state iac reads (NFR3.19 (g)).
var maxTFStateBytes int64 = 500 << 20

// maxStateModuleDepth bounds the nesting of child_modules: the reading of
// modules is recursive, and a file is not to be trusted (NFR3.19).
const maxStateModuleDepth = 256

// stateResource is all that is taken from one resource of the state: the other
// values, secrets among them, are never kept (security design §2, §8).
type stateResource struct {
	Address string `json:"address"`
	Mode    string `json:"mode"`
	Type    string `json:"type"`
	Values  struct {
		ARN any `json:"arn"`
	} `json:"values"`
}

func stateError(path, detailID, hintID string) error {
	return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonPlanFile, Subject: path,
		DetailID: detailID, HintID: hintID}
}

// readTFState reads the output of terraform show -json (BR1.3), one resource
// at a time, so the file never stands in memory as a whole.
func readTFState(path string) (stateIndex, error) {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, &errkind.Error{Kind: errkind.NotFound, Subject: path, DetailID: "iac.stateNotExist", HintID: "iac.stateFix"}
	case err != nil || !info.Mode().IsRegular():
		return nil, stateError(path, "iac.stateUnreadable", "iac.stateFix")
	case info.Size() > maxTFStateBytes:
		return nil, stateError(path, "iac.stateTooLarge", "iac.stateTooLargeFix")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, stateError(path, "iac.stateUnreadable", "iac.stateFix")
	}
	defer func() { _ = f.Close() }()
	br := bufio.NewReader(io.LimitReader(f, maxTFStateBytes+1))
	head, _ := br.Peek(3)
	switch {
	case bytes.HasPrefix(head, []byte("\xff\xfe")), bytes.HasPrefix(head, []byte("\xfe\xff")):
		return nil, stateError(path, "iac.stateUTF16", "iac.stateUTF16Fix")
	case bytes.HasPrefix(head, []byte("\xef\xbb\xbf")):
		_, _ = br.Discard(3)
	}
	r := &stateReader{dec: json.NewDecoder(br), index: stateIndex{}}
	if err := r.read(); err != nil {
		if errors.Is(err, errStatePlan) {
			return nil, stateError(path, "iac.statePlan", "iac.stateFix")
		}
		return nil, stateError(path, "iac.stateNotOutput", "iac.stateFix")
	}
	return r.index, nil
}

// errStatePlan is the output of terraform show -json for a plan file.
var errStatePlan = errors.New("the output for a plan file")

// errStateShape is any other content that is not the expected output.
var errStateShape = errors.New("not the output of terraform show -json")

type stateReader struct {
	dec   *json.Decoder
	index stateIndex
}

// read walks the top level. A plan's output is told from a state's output by
// fields that can come after values, so the whole level is read first.
func (r *stateReader) read() error {
	if err := r.expect('{'); err != nil {
		return err
	}
	hasFormat, isPlan := false, false
	err := r.fields(func(key string) error {
		switch key {
		case "format_version":
			hasFormat = true
		case "planned_values", "resource_changes", "prior_state":
			isPlan = true
		case "values":
			return r.values()
		}
		return r.skip()
	})
	switch {
	case err != nil:
		return err
	case !hasFormat:
		return errStateShape
	case isPlan:
		return errStatePlan
	}
	return nil
}

// fields calls read for each key of the object just opened; read must read the
// value of the key.
func (r *stateReader) fields(read func(string) error) error {
	for r.dec.More() {
		tok, err := r.dec.Token()
		if err != nil {
			return err
		}
		key, ok := tok.(string)
		if !ok {
			return errStateShape
		}
		if err := read(key); err != nil {
			return err
		}
	}
	_, err := r.dec.Token()
	return err
}

// open reads the start of a value: it reports false for null, and an error for
// anything but the delimiter.
func (r *stateReader) open(delim json.Delim) (bool, error) {
	tok, err := r.dec.Token()
	if err != nil {
		return false, err
	}
	if tok == nil {
		return false, nil
	}
	if d, ok := tok.(json.Delim); !ok || d != delim {
		return false, errStateShape
	}
	return true, nil
}

func (r *stateReader) expect(delim json.Delim) error {
	ok, err := r.open(delim)
	if err == nil && !ok {
		return errStateShape
	}
	return err
}

// skip reads one value without keeping it. The depth is a counter, not a
// recursion.
func (r *stateReader) skip() error {
	depth := 0
	for {
		tok, err := r.dec.Token()
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok {
			if d == '{' || d == '[' {
				depth++
			} else {
				depth--
			}
		}
		if depth == 0 {
			return nil
		}
	}
}

// values reads the object values; an empty state has none (BR1.3).
func (r *stateReader) values() error {
	ok, err := r.open('{')
	if err != nil || !ok {
		return err
	}
	return r.fields(func(key string) error {
		if key == "root_module" {
			return r.module(0)
		}
		return r.skip()
	})
}

// module reads one module with its child modules.
func (r *stateReader) module(depth int) error {
	if depth > maxStateModuleDepth {
		return errStateShape
	}
	ok, err := r.open('{')
	if err != nil || !ok {
		return err
	}
	return r.fields(func(key string) error {
		switch key {
		case "resources":
			return r.array(r.resource)
		case "child_modules":
			return r.array(func() error { return r.module(depth + 1) })
		}
		return r.skip()
	})
}

// array calls read for each item of an array value; null counts as empty.
func (r *stateReader) array(read func() error) error {
	ok, err := r.open('[')
	if err != nil || !ok {
		return err
	}
	for r.dec.More() {
		if err := read(); err != nil {
			return err
		}
	}
	_, err = r.dec.Token()
	return err
}

// resource reads one resource into the small structure and keeps the address
// of a managed resource with an ARN (BR1.3).
func (r *stateReader) resource() error {
	var res stateResource
	if err := r.dec.Decode(&res); err != nil {
		return errStateShape
	}
	arn, ok := res.Values.ARN.(string)
	if res.Mode == "managed" && ok && arn != "" && res.Address != "" {
		r.index[arn] = append(r.index[arn], res.Address)
	}
	return nil
}
