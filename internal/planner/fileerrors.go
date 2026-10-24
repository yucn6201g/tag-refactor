package planner

import (
	"fmt"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// The errors of a plan file (BR4.4) name their messages by identifier, with
// the values to put in, so the command line shows them in the user's
// language (the display catalog has the same identifiers in English and
// Japanese; a test of the display keeps the two English texts equal).
// Detail and Hint keep the English text for Error() and the logs.
var fileMessages = map[string]string{
	"planfile.notExist":      "the plan file does not exist",
	"planfile.unreadable":    "the plan file could not be read",
	"planfile.notRegular":    "the plan file is not a regular file",
	"planfile.tooLarge":      "the plan file is larger than 256 MiB",
	"planfile.utf16":         "the plan file is UTF-16",
	"planfile.versionNumber": "version must be a whole number",
	"planfile.unsupported":   "plan file version %d (written by tag-refactor %s) is not supported",
	"planfile.syntax":        "the plan file is not valid JSON (at byte %d)",
	"planfile.notJSON":       "the plan file is not valid JSON",
	"planfile.typeObject":    "%s must be an object, not a JSON %s",
	"planfile.typeArray":     "%s must be an array, not a JSON %s",
	"planfile.typeBool":      "%s must be true or false, not a JSON %s",
	"planfile.typeNumber":    "%s must be a whole number, not a JSON %s",
	"planfile.typeString":    "%s must be a string, not a JSON %s",
	"planfile.typeOther":     "%s must be %s, not a JSON %s",
	"planfile.missing":       "%s is missing",
	"planfile.empty":         "%s is empty",
	"planfile.awsKey":        "%s %s starts with aws:; keys of AWS are never changed",
	"planfile.badTime":       "%s %s is not a date and time such as 2026-10-19T12:04:00Z",
	"planfile.timeRange":     "%s %s is out of range",
	"planfile.twice":         "%s %s appears twice",
	"planfile.kind":          "%s %s is not renameKey, changeValue, addRequired or removeKey",
	"planfile.verdict":       "%s %s is not affected or undecided",
	"planfile.source":        "%s %s is not one of %s",
	"planfile.unknownChange": "%s %s names no change of the plan",

	"planfile.hintPath":     "check the path of the plan file",
	"planfile.hintReadable": "check that the file is readable",
	"planfile.hintRegular":  "pass the path of a plan file saved with plan --out",
	"planfile.hintRegion":   "create smaller plans with --region, for example one region at a time",
	"planfile.hintUTF8":     "save it again as UTF-8; on Windows PowerShell 5.1, save plans with --out instead of a redirection",
	"planfile.hintSave":     "save the plan again with plan --out",
	"planfile.hintVersion":  "this version of tag-refactor reads plan file version %d; create the plan again with it",
}

// FileMessages returns a copy of the English texts of the plan file errors
// by identifier, for the test that keeps the display catalog in step.
func FileMessages() map[string]string {
	out := make(map[string]string, len(fileMessages))
	for id, m := range fileMessages {
		out[id] = m
	}
	return out
}

// text is a message of a plan file error: its identifier and values.
type text struct {
	id   string
	args []any
}

func msg(id string, args ...any) text { return text{id: id, args: args} }

func (t text) english() string {
	if len(t.args) == 0 {
		return fileMessages[t.id]
	}
	return fmt.Sprintf(fileMessages[t.id], t.args...)
}

// fileError is an error of the plan file at path.
func fileError(kind errkind.Kind, path string, detail, hint text, cause error) error {
	return &errkind.Error{Kind: kind, Reason: errkind.ReasonPlanFile, Subject: path,
		Detail: detail.english(), DetailID: detail.id, DetailArgs: detail.args,
		Hint: hint.english(), HintID: hint.id, HintArgs: hint.args, Cause: cause}
}

// planError is an InvalidInput error of the plan file.
func planError(path string, detail, hint text) error {
	return fileError(errkind.InvalidInput, path, detail, hint, nil)
}
