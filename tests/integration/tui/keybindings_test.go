package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"noto/internal/tui"
)

func TestKeybindings_CtrlJOpensSettings(t *testing.T) {
	m := newTestModel(nil)
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	view := updated.(tui.Model).View().Content
	if !strings.Contains(view, "Settings") {
		t.Fatalf("expected settings dialog for Ctrl+J")
	}
}

func TestKeybindings_CtrlLOpensModelPicker(t *testing.T) {
	m := newTestModel(nil)
	updatedModel, cmd := m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	updated := updatedModel.(tui.Model)
	if msg := runCmd(cmd); msg != nil {
		updatedModel, _ = updated.Update(msg)
		updated = updatedModel.(tui.Model)
	}
	view := updated.View().Content
	if !strings.Contains(view, "Select model") {
		t.Fatalf("expected model picker for Ctrl+L")
	}
}

func TestKeybindings_CtrlDQuits(t *testing.T) {
	m := newTestModel(nil)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatalf("expected quit command for Ctrl+D")
	}
}
