package tui

import (
	"testing"
)

func TestFooterFields_AlwaysVisibleSet(t *testing.T) {
	m := newTestModel(nil)
	view := m.View().Content
	if !footerHasAllFields(view) {
		t.Fatalf("expected footer fields (tokens/cost/ctx/help) to be visible")
	}
	if !(containsAny(view, "ctx:l1-hit", "ctx:l2-hit", "ctx:swr", "ctx:rebuild", "ctx:miss")) {
		t.Fatalf("expected one of the ctx states in footer")
	}
}
