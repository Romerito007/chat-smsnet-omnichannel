package entity

import "strings"

// EvalContext is the hydrated conversation+contact state a rule matches against.
// It is built by the evaluator from the live conversation (and its contact) — for
// message_created too, the conditions resolve against the message's CONVERSATION,
// never the message itself.
type EvalContext struct {
	Status          string
	Channel         string
	AssignedAgentID string
	SectorID        string
	QueueID         string
	Priority        string
	Tags            []string
	ContactPhone    string
}

// Matches reports whether every condition holds (AND). An empty condition list
// matches every occurrence of the event (match-all).
func (r *AutomationRule) Matches(ec EvalContext) bool {
	for _, c := range r.Conditions {
		if !c.match(ec) {
			return false
		}
	}
	return true
}

// match evaluates one condition against the context per its field type.
func (c Condition) match(ec EvalContext) bool {
	switch c.Field {
	case FieldTags:
		has := containsString(ec.Tags, c.Value)
		if c.Operator == OpDoesNotContain {
			return !has
		}
		return has // OpContains
	case FieldContactPhone:
		if c.Operator == OpContains {
			return strings.Contains(ec.ContactPhone, c.Value)
		}
		return ec.ContactPhone == c.Value // OpEqualTo
	default:
		v := scalarField(ec, c.Field)
		if c.Operator == OpNotEqualTo {
			return v != c.Value
		}
		return v == c.Value // OpEqualTo
	}
}

func scalarField(ec EvalContext, field ConditionField) string {
	switch field {
	case FieldStatus:
		return ec.Status
	case FieldChannel:
		return ec.Channel
	case FieldAssignedAgentID:
		return ec.AssignedAgentID
	case FieldSectorID:
		return ec.SectorID
	case FieldQueueID:
		return ec.QueueID
	case FieldPriority:
		return ec.Priority
	default:
		return ""
	}
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
