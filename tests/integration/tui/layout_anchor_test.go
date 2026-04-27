package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"noto/internal/tui"
)

func TestLayoutAnchor_PickerKeepsFooterVisible(t *testing.T) {
	m := newTestModel(nil)
	updatedModel, cmd := m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	updated := updatedModel.(tui.Model)
	if msg := runCmd(cmd); msg != nil {
		updatedModel, _ = updated.Update(msg)
		updated = updatedModel.(tui.Model)
	}
	view := updated.View().Content
	if !strings.Contains(view, "ctx:") || !strings.Contains(view, "main-model") {
		t.Fatalf("expected footer to remain visible in picker view")
	}
}
