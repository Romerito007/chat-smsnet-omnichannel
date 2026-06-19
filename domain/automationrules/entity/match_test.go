package entity

import "testing"

func rule(conds ...Condition) *AutomationRule {
	return &AutomationRule{Event: EventConversationCreated, Conditions: conds}
}

func baseCtx() EvalContext {
	return EvalContext{
		Status: "open", Channel: "whatsapp", AssignedAgentID: "u1",
		SectorID: "s1", QueueID: "q1", Priority: "high",
		Tags: []string{"vip", "lead"}, ContactPhone: "+5511999998888",
	}
}

func TestMatch_ScalarEqualAndNotEqual(t *testing.T) {
	ec := baseCtx()
	if !rule(Condition{FieldStatus, OpEqualTo, "open"}).Matches(ec) {
		t.Error("status equal_to open should match")
	}
	if rule(Condition{FieldStatus, OpEqualTo, "closed"}).Matches(ec) {
		t.Error("status equal_to closed should not match")
	}
	if !rule(Condition{FieldChannel, OpNotEqualTo, "telegram"}).Matches(ec) {
		t.Error("channel not_equal_to telegram should match")
	}
	if rule(Condition{FieldPriority, OpNotEqualTo, "high"}).Matches(ec) {
		t.Error("priority not_equal_to high should not match (it is high)")
	}
}

func TestMatch_InteractiveReplyID(t *testing.T) {
	ec := baseCtx()
	ec.InteractiveReplyID = "intent_500mb"

	if !rule(Condition{FieldInteractiveReplyID, OpEqualTo, "intent_500mb"}).Matches(ec) {
		t.Error("equal_to the chosen id should match")
	}
	if rule(Condition{FieldInteractiveReplyID, OpEqualTo, "intent_1gb"}).Matches(ec) {
		t.Error("equal_to a different id should not match")
	}
	if !rule(Condition{FieldInteractiveReplyID, OpNotEqualTo, "intent_1gb"}).Matches(ec) {
		t.Error("not_equal_to a different id should match")
	}
	// contains = membership in a comma-separated allowlist.
	if !rule(Condition{FieldInteractiveReplyID, OpContains, "intent_1gb, intent_500mb ,intent_2gb"}).Matches(ec) {
		t.Error("contains should match when the id is in the list")
	}
	if rule(Condition{FieldInteractiveReplyID, OpContains, "intent_1gb,intent_2gb"}).Matches(ec) {
		t.Error("contains should not match when the id is absent from the list")
	}
	if !rule(Condition{FieldInteractiveReplyID, OpDoesNotContain, "intent_1gb,intent_2gb"}).Matches(ec) {
		t.Error("does_not_contain should match when the id is absent")
	}
	// an empty reply id never matches a positive membership test.
	ec.InteractiveReplyID = ""
	if rule(Condition{FieldInteractiveReplyID, OpContains, "intent_500mb"}).Matches(ec) {
		t.Error("an empty reply id must not match contains")
	}
}

func TestMatch_TagsContains(t *testing.T) {
	ec := baseCtx()
	if !rule(Condition{FieldTags, OpContains, "vip"}).Matches(ec) {
		t.Error("tags contains vip should match")
	}
	if rule(Condition{FieldTags, OpContains, "churn"}).Matches(ec) {
		t.Error("tags contains churn should not match")
	}
	if !rule(Condition{FieldTags, OpDoesNotContain, "churn"}).Matches(ec) {
		t.Error("tags does_not_contain churn should match")
	}
	if rule(Condition{FieldTags, OpDoesNotContain, "vip"}).Matches(ec) {
		t.Error("tags does_not_contain vip should not match")
	}
}

func TestMatch_PhoneEqualAndContains(t *testing.T) {
	ec := baseCtx()
	if !rule(Condition{FieldContactPhone, OpContains, "5511"}).Matches(ec) {
		t.Error("phone contains 5511 should match")
	}
	if !rule(Condition{FieldContactPhone, OpEqualTo, "+5511999998888"}).Matches(ec) {
		t.Error("phone equal_to full number should match")
	}
	if rule(Condition{FieldContactPhone, OpContains, "0000"}).Matches(ec) {
		t.Error("phone contains 0000 should not match")
	}
}

func TestMatch_AndAcrossConditions(t *testing.T) {
	ec := baseCtx()
	all := rule(
		Condition{FieldStatus, OpEqualTo, "open"},
		Condition{FieldTags, OpContains, "vip"},
		Condition{FieldSectorID, OpEqualTo, "s1"},
	)
	if !all.Matches(ec) {
		t.Error("all conditions true → match")
	}
	one := rule(
		Condition{FieldStatus, OpEqualTo, "open"},
		Condition{FieldSectorID, OpEqualTo, "OTHER"},
	)
	if one.Matches(ec) {
		t.Error("one condition false → no match (AND)")
	}
}

func TestMatch_EmptyConditionsMatchAll(t *testing.T) {
	if !rule().Matches(baseCtx()) {
		t.Error("empty conditions must match every occurrence")
	}
}
