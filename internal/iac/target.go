package iac

import (
	"strings"

	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// The tags AWS puts on what CloudFormation makes (BR2.2).
const (
	stackNameTag = "aws:cloudformation:stack-name"
	logicalIDTag = "aws:cloudformation:logical-id"
)

// target is a resource with changes on its way through iac (IacTarget).
type target struct {
	arn          string
	resourceType string
	before       map[string]string
	changes      []planner.Change
	tool         Tool
	// stackName and logicalID are set for CloudFormation.
	stackName, logicalID string
	// address is set for Terraform.
	address string
}

// changeKey is the key a manual item names: the key the change starts from, or
// the key it adds.
func changeKey(c planner.Change) string {
	if c.FromKey != "" {
		return c.FromKey
	}
	return c.ToKey
}

// manualAll puts every change of the target on the list to fix by hand.
func manualAll(res *Result, t *target, reason ManualReason, kind DetailKind, detail string) {
	for _, c := range t.changes {
		res.Manual = append(res.Manual, ManualItem{ARN: t.arn, ChangeID: c.ChangeID, Key: changeKey(c),
			Reason: reason, Detail: detail, DetailKind: kind})
	}
}

// buildTargets decides, for each resource with changes, which IaC it belongs
// to (BR2.1 to BR2.4). The targets it returns still have to find their place;
// the rest are already counted in the result.
func buildTargets(plan *planner.Plan, state stateIndex) ([]*target, *Result) {
	res := &Result{}
	var targets []*target
	for i := range plan.Resources {
		r := &plan.Resources[i]
		if len(r.Changes) == 0 {
			continue
		}
		res.Resources++
		t := &target{arn: r.ARN, resourceType: r.ResourceType, before: r.BeforeTags, changes: r.Changes}
		if decideTool(res, t, state) {
			targets = append(targets, t)
		}
	}
	return targets, res
}

// decideTool sets the tool of the target and reports whether the target goes
// on to look for its place.
func decideTool(res *Result, t *target, state stateIndex) bool {
	stack, hasStack := t.before[stackNameTag]
	logical, hasLogical := t.before[logicalIDTag]
	switch {
	case hasStack && hasLogical:
		t.tool, t.stackName, t.logicalID = ToolCloudFormation, stack, logical
		return true
	case hasStack:
		t.tool = ToolCloudFormation
		manualAll(res, t, ReasonTemplateNotFound, DetailNone, "")
		return false
	}
	addresses := state[t.arn]
	switch {
	case len(addresses) == 0:
		t.tool = ToolNone
		res.Unmanaged++
		return false
	case len(addresses) > 1:
		t.tool = ToolTerraform
		manualAll(res, t, ReasonAmbiguous, DetailAddresses, strings.Join(addresses, ", "))
		return false
	}
	t.tool, t.address = ToolTerraform, addresses[0]
	switch {
	case strings.HasPrefix(t.address, "module."):
		manualAll(res, t, ReasonInModule, DetailAddress, t.address)
		return false
	case strings.Contains(t.address, "["):
		manualAll(res, t, ReasonMultipleInstances, DetailAddress, t.address)
		return false
	}
	return true
}
