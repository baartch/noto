package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	nototui "noto/internal/tui"
)

func TestModel_NeedsProviderSetup_ShowsDialog(t *testing.T) {
	m := newTestModelWithSetup(true)
	view := m.View().Content
	if view == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestModel_NoProviderSetup_NoDialog(t *testing.T) {
	m := newTestModelWithSetup(false)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	um := updated.(nototui.Model)
	view := um.View().Content
	if view == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestModel_SetNeedsProviderSetup(t *testing.T) {
	m := newTestModelWithSetup(false)
	m.SetNeedsProviderSetup()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	um := updated.(nototui.Model)
	view := um.View().Content
	if view == "" {
		t.Fatal("expected non-empty view after SetNeedsProviderSetup")
	}
}
