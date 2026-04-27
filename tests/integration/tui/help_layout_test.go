package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/tui"
)

func TestHelpLayout_ExpandedHelpRenders(t *testing.T) {
	m := newTestModel(nil)
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl})
	view := updated.(tui.Model).View().Content
	if !strings.Contains(strings.ToLower(view), "help") {
		t.Fatalf("expected expanded help content")
	}
}
