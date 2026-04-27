package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/tui"
)

func TestHelpComponentGrouping_FooterShortHelpIsPrimaryOnly(t *testing.T) {
	m := newTestModel(nil)
	view := m.View().Content
	if !strings.Contains(strings.ToLower(view), "ctrl+h") {
		t.Fatalf("expected primary help keybinding in footer")
	}

	// Expanded help should include secondary bindings as well.
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl})
	expanded := updated.(tui.Model).View().Content
	if !strings.Contains(strings.ToLower(expanded), "ctrl+l") {
		t.Fatalf("expected secondary bindings in expanded help")
	}
}
