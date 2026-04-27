package tui

import (
	"testing"

	"noto/internal/tui"
)

func TestFooterFields_AlwaysVisibleSet(t *testing.T) {
	m := newTestModel(nil)
	view := m.View().Content
	if !footerHasAllFields(view) {
		t.Fatalf("expected footer fields (tokens/cost/ctx/help) to be visible")
	}
	_ = tui.UsageUpdatedMain // keep compile reference for message API
}
