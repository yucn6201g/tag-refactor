package planner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/inventory"
)

// The saved plan (contract C2). plan --out writes it with WritePlan; U6's
// apply and U8's iac read it with LoadPlan.

// SupportedVersion is the version of the plan file this tool writes and
// reads.
const SupportedVersion = 1

// MaxPlanBytes is the largest plan file that is written or read (256 MiB,
// NFR8.18): ten times a plan of 10,000 resources with 20 tags each.
const MaxPlanBytes = 256 << 20

// timeLayout writes the creation time in UTC; fractions of a second appear
// only when the time has them.
const timeLayout = time.RFC3339Nano

// The JSON types of contract C2. Their fields are the only ones written, so
// nothing else (credentials, profile settings, environment variables) can
// reach the file (NFR8.19).
type jsonResource struct {
	ARN          string            `json:"arn"`
	Region       string            `json:"region"`
	Global       bool              `json:"global"`
	ResourceType string            `json:"resourceType"`
	BeforeTags   map[string]string `json:"beforeTags"`
	IaCManagedBy *string           `json:"iacManagedBy"`
	Changes      []jsonChange      `json:"changes"`
}

type jsonChange struct {
	ChangeID           string     `json:"changeId"`
	Kind               ChangeKind `json:"kind"`
	FromKey            *string    `json:"fromKey,omitempty"`
	ToKey              *string    `json:"toKey,omitempty"`
	FromValue          *string    `json:"fromValue,omitempty"`
	ToValue            *string    `json:"toValue,omitempty"`
	MergesIntoExisting *bool      `json:"mergesIntoExisting,omitempty"`
	Impacted           bool       `json:"impacted"`
}

type jsonConflict struct {
	ARN          string            `json:"arn"`
	CanonicalKey string            `json:"canonicalKey"`
	PresentTags  map[string]string `json:"presentTags"`
}

// jsonFinding is one impact finding of contract C2. U5 added
// conditionOperator, written as is, and empty for Resource and NotResource.
type jsonFinding struct {
	Verdict           Verdict  `json:"verdict"`
	PolicyRef         string   `json:"policyRef"`
	SourceKind        string   `json:"sourceKind"`
	StatementID       string   `json:"statementId"`
	ConditionOperator string   `json:"conditionOperator"`
	ConditionKey      string   `json:"conditionKey"`
	ConditionValue    string   `json:"conditionValue"`
	ChangeIDs         []string `json:"changeIds"`
}

// jsonUnread is one unread policy of contract C2. U5 added policyRef and
// region, left out when empty (a whole list; IAM and the bucket list).
type jsonUnread struct {
	SourceKind string `json:"sourceKind"`
	PolicyRef  string `json:"policyRef,omitempty"`
	Region     string `json:"region,omitempty"`
	Reason     string `json:"reason"`
}

type jsonUnset struct {
	Key  string   `json:"key"`
	ARNs []string `json:"arns"`
}

type jsonTypeCount struct {
	ResourceType string `json:"resourceType"`
	Count        int    `json:"count"`
}

func ptr[T any](v T) *T { return &v }

func toJSONChange(c Change) jsonChange {
	j := jsonChange{ChangeID: c.ChangeID, Kind: c.Kind, Impacted: c.Impacted}
	switch c.Kind {
	case KindRenameKey:
		j.FromKey, j.ToKey, j.FromValue = ptr(c.FromKey), ptr(c.ToKey), ptr(c.FromValue)
		j.MergesIntoExisting = ptr(c.MergesIntoExisting)
	case KindChangeValue:
		j.FromKey, j.FromValue, j.ToValue = ptr(c.FromKey), ptr(c.FromValue), ptr(c.ToValue)
	case KindAddRequired:
		j.ToKey, j.ToValue = ptr(c.ToKey), ptr(c.ToValue)
	case KindRemoveKey:
		j.FromKey, j.FromValue = ptr(c.FromKey), ptr(c.FromValue)
	}
	return j
}

// resourceJSON returns resource i of the plan as contract C2 writes it.
func (p *Plan) resourceJSON(i int) any {
	r := p.Resources[i]
	j := jsonResource{ARN: r.ARN, Region: r.Region, Global: r.Global, ResourceType: r.ResourceType,
		BeforeTags: r.BeforeTags, Changes: make([]jsonChange, 0, len(r.Changes))}
	if j.BeforeTags == nil {
		j.BeforeTags = map[string]string{}
	}
	if r.IaCManagedBy != "" {
		j.IaCManagedBy = ptr(r.IaCManagedBy)
	}
	for _, c := range r.Changes {
		j.Changes = append(j.Changes, toJSONChange(c))
	}
	return j
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// FormatTime writes a creation time as the plan file and the JSON output
// do: RFC 3339 in UTC.
func FormatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

// WriteDetails writes the fields of contract C2 that the details of plan
// --format json share with the plan file (contract C3): resources,
// keyConflicts, impactFindings, unreadPolicies, unsetRequiredTags and
// removeCandidates, one array item at a time.
func WriteDetails(o *JSONObject, p *Plan) {
	o.Array("resources", len(p.Resources), p.resourceJSON)
	o.Array("keyConflicts", len(p.Conflicts), func(i int) any {
		c := p.Conflicts[i]
		tags := c.PresentTags
		if tags == nil {
			tags = map[string]string{}
		}
		return jsonConflict{ARN: c.ResourceARN, CanonicalKey: c.CanonicalKey, PresentTags: tags}
	})
	o.Array("impactFindings", len(p.ImpactFindings), func(i int) any {
		f := p.ImpactFindings[i]
		return jsonFinding{Verdict: f.Verdict, PolicyRef: f.PolicyRef, SourceKind: f.SourceKind, StatementID: f.StatementID,
			ConditionOperator: f.ConditionOperator, ConditionKey: f.ConditionKey, ConditionValue: f.ConditionValue,
			ChangeIDs: nonNil(f.ChangeIDs)}
	})
	o.Array("unreadPolicies", len(p.UnreadPolicies), func(i int) any {
		u := p.UnreadPolicies[i]
		return jsonUnread(u)
	})
	o.Array("unsetRequiredTags", len(p.UnsetRequiredTags), func(i int) any {
		u := p.UnsetRequiredTags[i]
		return jsonUnset{Key: u.Key, ARNs: nonNil(u.ARNs)}
	})
	keys := make([]string, 0, len(p.RemoveCandidates))
	for _, c := range p.RemoveCandidates {
		keys = append(keys, c.Key)
	}
	o.Field("removeCandidates", keys)
}

// TypeCountsJSON returns the unsupported resource types as contract C2 and
// C3 write them.
func TypeCountsJSON(types []inventory.TypeCount) []any {
	out := make([]any, 0, len(types))
	for _, t := range types {
		out = append(out, jsonTypeCount{ResourceType: t.ResourceType, Count: t.Count})
	}
	return out
}

// WritePlan writes the plan as contract C2 to w, one array item at a time
// (U4 NFR design §3). Pass a SafeJSONWriter as w, with a size-counting writer
// below it; WritePlan stops at the first error of w.
func WritePlan(w io.Writer, p *Plan) error {
	o := NewJSONObject(w, "")
	o.Field("version", SupportedVersion)
	o.Field("toolVersion", p.ToolVersion)
	o.Field("planId", p.PlanID)
	o.Field("accountId", p.AccountID)
	o.Field("createdAt", FormatTime(p.CreatedAt))
	o.Field("regions", nonNil(p.Regions))
	o.Field("inventoryComplete", p.InventoryComplete)
	o.Field("impactAnalysisComplete", p.ImpactAnalysisComplete)
	o.Field("ruleFile", p.RuleFile)
	WriteDetails(o, p)
	types := TypeCountsJSON(p.UnsupportedResourceTypes)
	o.Array("unsupportedResourceTypes", len(types), func(i int) any { return types[i] })
	if err := o.Close(); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// The JSON types LoadPlan decodes into: pointers tell a missing field from
// an empty one.
type readPlan struct {
	Version                  *int            `json:"version"`
	ToolVersion              *string         `json:"toolVersion"`
	PlanID                   *string         `json:"planId"`
	AccountID                *string         `json:"accountId"`
	CreatedAt                *string         `json:"createdAt"`
	Regions                  *[]string       `json:"regions"`
	InventoryComplete        *bool           `json:"inventoryComplete"`
	ImpactAnalysisComplete   *bool           `json:"impactAnalysisComplete"`
	RuleFile                 *string         `json:"ruleFile"`
	Resources                *[]readResource `json:"resources"`
	KeyConflicts             []readConflict  `json:"keyConflicts"`
	ImpactFindings           []readFinding   `json:"impactFindings"`
	UnreadPolicies           []jsonUnread    `json:"unreadPolicies"`
	UnsetRequiredTags        []readUnset     `json:"unsetRequiredTags"`
	RemoveCandidates         []string        `json:"removeCandidates"`
	UnsupportedResourceTypes []jsonTypeCount `json:"unsupportedResourceTypes"`
}

type readResource struct {
	ARN          *string            `json:"arn"`
	Region       *string            `json:"region"`
	Global       *bool              `json:"global"`
	ResourceType *string            `json:"resourceType"`
	BeforeTags   *map[string]string `json:"beforeTags"`
	IaCManagedBy *string            `json:"iacManagedBy"`
	Changes      *[]readChange      `json:"changes"`
}

type readChange struct {
	ChangeID           *string `json:"changeId"`
	Kind               *string `json:"kind"`
	FromKey            *string `json:"fromKey"`
	ToKey              *string `json:"toKey"`
	FromValue          *string `json:"fromValue"`
	ToValue            *string `json:"toValue"`
	MergesIntoExisting *bool   `json:"mergesIntoExisting"`
}

type readConflict struct {
	ARN          *string            `json:"arn"`
	CanonicalKey *string            `json:"canonicalKey"`
	PresentTags  *map[string]string `json:"presentTags"`
}

type readFinding struct {
	Verdict     *string `json:"verdict"`
	PolicyRef   *string `json:"policyRef"`
	SourceKind  *string `json:"sourceKind"`
	StatementID string  `json:"statementId"`
	// ConditionOperator is optional: plans saved before U5 have none.
	ConditionOperator string    `json:"conditionOperator"`
	ConditionKey      string    `json:"conditionKey"`
	ConditionValue    string    `json:"conditionValue"`
	ChangeIDs         *[]string `json:"changeIds"`
}

type readUnset struct {
	Key  *string   `json:"key"`
	ARNs *[]string `json:"arns"`
}

// LoadPlan reads the plan file at path (BR4.4, NFR8.18). Only a regular file
// of at most MaxPlanBytes is read; a larger one is refused without reading
// it. ParsePlan does every other check. It never calls AWS.
func LoadPlan(path string) (*Plan, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fileError(errkind.NotFound, path, msg("planfile.notExist"), msg("planfile.hintPath"), err)
		}
		return nil, readFailed(path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, planError(path, msg("planfile.notRegular"), msg("planfile.hintRegular"))
	}
	if info.Size() > MaxPlanBytes {
		return nil, tooLarge(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, readFailed(path, err)
	}
	defer func() { _ = f.Close() }()
	// The buffer is sized from the file once (NFR1.15), and the read is
	// bounded again, for a file that grows while it is read.
	var buf bytes.Buffer
	buf.Grow(int(info.Size()) + bytes.MinRead)
	if _, err := buf.ReadFrom(io.LimitReader(f, MaxPlanBytes+1)); err != nil {
		return nil, readFailed(path, err)
	}
	return ParsePlan(path, buf.Bytes())
}

func readFailed(path string, err error) error {
	return fileError(errkind.StorageFailed, path, msg("planfile.unreadable"), msg("planfile.hintReadable"), err)
}

func tooLarge(path string) error {
	return planError(path, msg("planfile.tooLarge"), msg("planfile.hintRegion"))
}

// quote shows a value from the file in a message: quoted and at most 64
// characters. The display sanitizes it before it reaches the terminal.
func quote(s string) string {
	if r := []rune(s); len(r) > 64 {
		s = string(r[:64]) + "..."
	}
	return strconv.Quote(s)
}

// ParsePlan checks and decodes the content of a plan file (BR4.4). path is
// used in messages. A UTF-8 byte order mark is skipped; a UTF-16 one is an
// error that asks for UTF-8. Unknown fields are ignored; unknown values of
// kind, verdict and sourceKind are errors. Every mark and impacted flag is
// worked out again from the impact findings: the saved ones are not used.
func ParsePlan(path string, data []byte) (*Plan, error) {
	if len(data) > MaxPlanBytes {
		return nil, tooLarge(path)
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if bytes.HasPrefix(data, []byte("\xff\xfe")) || bytes.HasPrefix(data, []byte("\xfe\xff")) {
		return nil, planError(path, msg("planfile.utf16"), msg("planfile.hintUTF8"))
	}
	var head struct {
		Version     json.RawMessage `json:"version"`
		ToolVersion json.RawMessage `json:"toolVersion"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, decodeError(path, err)
	}
	if head.Version == nil {
		return nil, planError(path, msg("planfile.missing", "version"), msg("planfile.hintSave"))
	}
	var version int
	if err := json.Unmarshal(head.Version, &version); err != nil {
		return nil, planError(path, msg("planfile.versionNumber"), msg("planfile.hintSave"))
	}
	if version != SupportedVersion {
		var tool string
		_ = json.Unmarshal(head.ToolVersion, &tool)
		return nil, fileError(errkind.UnsupportedVersion, path, msg("planfile.unsupported", version, quote(tool)),
			msg("planfile.hintVersion", SupportedVersion), nil)
	}
	var rp readPlan
	if err := json.Unmarshal(data, &rp); err != nil {
		return nil, decodeError(path, err)
	}
	return (&planReader{path: path}).build(&rp)
}

// decodeError turns an error of encoding/json into InvalidInput with the
// place in the file.
func decodeError(path string, err error) error {
	var syntax *json.SyntaxError
	var typ *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return planError(path, msg("planfile.syntax", syntax.Offset), msg("planfile.hintSave"))
	case errors.As(err, &typ):
		field := typ.Field
		if field == "" {
			field = "(the plan)"
		}
		return planError(path, typeMessage(field, typ.Type.Kind().String(), typ.Value), msg("planfile.hintSave"))
	}
	return planError(path, msg("planfile.notJSON"), msg("planfile.hintSave"))
}

// typeMessage says which type a field must have; value is the JSON type it
// had ("string", "number"...), shown as it is.
func typeMessage(field, kind, value string) text {
	switch kind {
	case "struct", "map":
		return msg("planfile.typeObject", field, value)
	case "slice":
		return msg("planfile.typeArray", field, value)
	case "bool":
		return msg("planfile.typeBool", field, value)
	case "int":
		return msg("planfile.typeNumber", field, value)
	case "string":
		return msg("planfile.typeString", field, value)
	}
	return msg("planfile.typeOther", field, kind, value)
}

// planReader checks a decoded plan field by field and stops at the first
// error.
type planReader struct {
	path string
	err  error
}

func (r *planReader) fail(detail text) {
	if r.err == nil {
		r.err = planError(r.path, detail, msg("planfile.hintSave"))
	}
}

func (r *planReader) str(field string, v *string) string {
	if v == nil {
		r.fail(msg("planfile.missing", field))
		return ""
	}
	return *v
}

// key reads a tag key that must not be empty and must not start with aws:
// (BR1.6).
func (r *planReader) key(field string, v *string) string {
	k := r.str(field, v)
	if r.err == nil && k == "" {
		r.fail(msg("planfile.empty", field))
	}
	if r.err == nil && inventory.IsAWSReservedKey(k) {
		r.fail(msg("planfile.awsKey", field, quote(k)))
	}
	return k
}

func (r *planReader) boolean(field string, v *bool) bool {
	if v == nil {
		r.fail(msg("planfile.missing", field))
		return false
	}
	return *v
}

func (r *planReader) build(rp *readPlan) (*Plan, error) {
	p := &Plan{
		ToolVersion:              r.str("toolVersion", rp.ToolVersion),
		PlanID:                   r.str("planId", rp.PlanID),
		AccountID:                r.str("accountId", rp.AccountID),
		InventoryComplete:        r.boolean("inventoryComplete", rp.InventoryComplete),
		ImpactAnalysisComplete:   r.boolean("impactAnalysisComplete", rp.ImpactAnalysisComplete),
		Resources:                []PlannedResource{},
		Conflicts:                []KeyConflict{},
		UnsetRequiredTags:        []UnsetRequiredTag{},
		RemoveCandidates:         []RemoveCandidate{},
		UnsupportedResourceTypes: []inventory.TypeCount{},
		ImpactFindings:           []ImpactFinding{},
		UnreadPolicies:           []UnreadPolicy{},
		FailedRegions:            []inventory.FailedRegion{},
	}
	created := r.str("createdAt", rp.CreatedAt)
	if r.err == nil {
		t, err := time.Parse(time.RFC3339, created)
		if err != nil {
			r.fail(msg("planfile.badTime", "createdAt", quote(created)))
		} else if t = t.UTC(); t.Year() < 0 || t.Year() > 9999 {
			r.fail(msg("planfile.timeRange", "createdAt", quote(created)))
		} else {
			p.CreatedAt = t
		}
	}
	if rp.Regions == nil && r.err == nil {
		r.fail(msg("planfile.missing", "regions"))
	} else if rp.Regions != nil {
		p.Regions = append([]string{}, *rp.Regions...)
	}
	if rp.RuleFile != nil {
		p.RuleFile = *rp.RuleFile
	}
	if rp.Resources == nil {
		r.fail(msg("planfile.missing", "resources"))
	} else {
		r.resources(p, *rp.Resources)
	}
	r.conflicts(p, rp.KeyConflicts)
	r.findings(p, rp.ImpactFindings)
	for _, u := range rp.UnreadPolicies {
		p.UnreadPolicies = append(p.UnreadPolicies, UnreadPolicy(u))
	}
	for i, u := range rp.UnsetRequiredTags {
		at := fmt.Sprintf("unsetRequiredTags[%d]", i)
		key := r.str(at+".key", u.Key)
		if u.ARNs == nil {
			r.fail(msg("planfile.missing", at+".arns"))
			continue
		}
		p.UnsetRequiredTags = append(p.UnsetRequiredTags, UnsetRequiredTag{Key: key, ARNs: append([]string{}, *u.ARNs...)})
	}
	for _, k := range rp.RemoveCandidates {
		p.RemoveCandidates = append(p.RemoveCandidates, RemoveCandidate{Key: k})
	}
	for _, t := range rp.UnsupportedResourceTypes {
		p.UnsupportedResourceTypes = append(p.UnsupportedResourceTypes, inventory.TypeCount{ResourceType: t.ResourceType, Count: t.Count})
	}
	if r.err != nil {
		return nil, r.err
	}
	p.markChanges()
	return p, nil
}

func (r *planReader) resources(p *Plan, resources []readResource) {
	arns := make(map[string]bool, len(resources))
	ids := map[string]bool{}
	for i, res := range resources {
		if r.err != nil {
			return
		}
		at := fmt.Sprintf("resources[%d]", i)
		pr := PlannedResource{
			ARN:          r.str(at+".arn", res.ARN),
			Region:       r.str(at+".region", res.Region),
			Global:       r.boolean(at+".global", res.Global),
			ResourceType: r.str(at+".resourceType", res.ResourceType),
		}
		if r.err == nil && pr.ARN == "" {
			r.fail(msg("planfile.empty", at+".arn"))
		}
		if r.err == nil && arns[pr.ARN] {
			r.fail(msg("planfile.twice", at+".arn", quote(pr.ARN)))
		}
		arns[pr.ARN] = true
		if res.BeforeTags == nil || *res.BeforeTags == nil {
			r.fail(msg("planfile.missing", at+".beforeTags"))
			return
		}
		pr.BeforeTags = make(map[string]string, len(*res.BeforeTags))
		for k, v := range *res.BeforeTags {
			pr.BeforeTags[k] = v
		}
		if res.IaCManagedBy != nil {
			pr.IaCManagedBy = *res.IaCManagedBy
		}
		if res.Changes == nil {
			r.fail(msg("planfile.missing", at+".changes"))
			return
		}
		for j, c := range *res.Changes {
			change := r.change(fmt.Sprintf("%s.changes[%d]", at, j), c)
			if r.err != nil {
				return
			}
			if ids[change.ChangeID] {
				r.fail(msg("planfile.twice", fmt.Sprintf("%s.changes[%d].changeId", at, j), quote(change.ChangeID)))
				return
			}
			ids[change.ChangeID] = true
			change.ResourceARN = pr.ARN
			pr.Changes = append(pr.Changes, change)
		}
		if len(pr.Changes) == 0 {
			r.fail(msg("planfile.empty", at+".changes"))
			return
		}
		p.Resources = append(p.Resources, pr)
	}
}

// change reads one change with the fields its kind needs (contract C2). The
// fields its kind does not use are dropped.
func (r *planReader) change(at string, c readChange) Change {
	id := r.str(at+".changeId", c.ChangeID)
	if r.err == nil && id == "" {
		r.fail(msg("planfile.empty", at+".changeId"))
	}
	kind := ChangeKind(r.str(at+".kind", c.Kind))
	if r.err != nil {
		return Change{}
	}
	out := Change{ChangeID: id, Kind: kind, Mark: MarkNone}
	switch kind {
	case KindRenameKey:
		out.FromKey = r.key(at+".fromKey", c.FromKey)
		out.ToKey = r.key(at+".toKey", c.ToKey)
		out.MergesIntoExisting = r.boolean(at+".mergesIntoExisting", c.MergesIntoExisting)
		if c.FromValue != nil {
			out.FromValue = *c.FromValue
		}
	case KindChangeValue:
		out.FromKey = r.key(at+".fromKey", c.FromKey)
		out.FromValue = r.str(at+".fromValue", c.FromValue)
		out.ToValue = r.str(at+".toValue", c.ToValue)
	case KindAddRequired:
		out.ToKey = r.key(at+".toKey", c.ToKey)
		out.ToValue = r.str(at+".toValue", c.ToValue)
	case KindRemoveKey:
		out.FromKey = r.key(at+".fromKey", c.FromKey)
		if c.FromValue != nil {
			out.FromValue = *c.FromValue
		}
	default:
		r.fail(msg("planfile.kind", at+".kind", quote(string(kind))))
	}
	return out
}

func (r *planReader) conflicts(p *Plan, conflicts []readConflict) {
	for i, c := range conflicts {
		at := fmt.Sprintf("keyConflicts[%d]", i)
		arn := r.str(at+".arn", c.ARN)
		canonical := r.str(at+".canonicalKey", c.CanonicalKey)
		if c.PresentTags == nil || *c.PresentTags == nil {
			r.fail(msg("planfile.missing", at+".presentTags"))
		}
		if r.err != nil {
			return
		}
		kc := KeyConflict{ResourceARN: arn, CanonicalKey: canonical, PresentTags: map[string]string{}}
		for k, v := range *c.PresentTags {
			kc.PresentTags[k] = v
			if k != canonical {
				kc.KeyOrder = append(kc.KeyOrder, k)
			}
		}
		// The file does not keep the order of the aliases; key order, then
		// the canonical key, is used instead.
		slices.Sort(kc.KeyOrder)
		if _, ok := kc.PresentTags[canonical]; ok {
			kc.KeyOrder = append(kc.KeyOrder, canonical)
		}
		p.Conflicts = append(p.Conflicts, kc)
	}
}

func (r *planReader) findings(p *Plan, findings []readFinding) {
	ids := map[string]bool{}
	for _, res := range p.Resources {
		for _, c := range res.Changes {
			ids[c.ChangeID] = true
		}
	}
	for i, f := range findings {
		if r.err != nil {
			return
		}
		at := fmt.Sprintf("impactFindings[%d]", i)
		verdict := Verdict(r.str(at+".verdict", f.Verdict))
		ref := r.str(at+".policyRef", f.PolicyRef)
		source := r.str(at+".sourceKind", f.SourceKind)
		if f.ChangeIDs == nil {
			r.fail(msg("planfile.missing", at+".changeIds"))
		}
		if r.err != nil {
			return
		}
		if verdict != VerdictAffected && verdict != VerdictUndecided {
			r.fail(msg("planfile.verdict", at+".verdict", quote(string(verdict))))
			return
		}
		if !slices.Contains(SourceKinds, source) {
			r.fail(msg("planfile.source", at+".sourceKind", quote(source), strings.Join(SourceKinds, ", ")))
			return
		}
		for j, id := range *f.ChangeIDs {
			if !ids[id] {
				r.fail(msg("planfile.unknownChange", fmt.Sprintf("%s.changeIds[%d]", at, j), quote(id)))
				return
			}
		}
		p.ImpactFindings = append(p.ImpactFindings, ImpactFinding{Verdict: verdict, PolicyRef: ref, SourceKind: source,
			StatementID: f.StatementID, ConditionOperator: f.ConditionOperator, ConditionKey: f.ConditionKey, ConditionValue: f.ConditionValue,
			ChangeIDs: append([]string{}, *f.ChangeIDs...)})
	}
}
