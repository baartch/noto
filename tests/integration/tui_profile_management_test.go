package integration

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/profile"
	"noto/internal/store"
	"noto/internal/tui"
)

func asModel(t *testing.T, updated tea.Model) tui.Model {
	t.Helper()
	switch m := updated.(type) {
	case tui.Model:
		return m
	case *tui.Model:
		return *m
	default:
		t.Fatalf("expected tui.Model, got %T", updated)
	}
	return tui.Model{}
}

func openProfilesSubmenu(t *testing.T, m tui.Model) tui.Model {
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	m = asModel(t, updated)
	for range 3 {
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		m = asModel(t, updated)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return asModel(t, updated)
}

func newProfileSettingsModel(t *testing.T) (tui.Model, *commands.ExecContext) {
	_, execCtx := newSettingsModel(t)

	execCtx = &commands.ExecContext{ProfileID: execCtx.ProfileID, ProfileSlug: execCtx.ProfileSlug, DB: execCtx.DB}
	m := tui.New(
		"Profile", "", "", "cache: n/a", "tokens: n/a", false,
		chat.NewDispatcher(commands.NewRegistry()),
		execCtx,
		nil, nil,
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
	)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = asModel(t, updated)
	return m, execCtx
}

func TestSettingsProfiles_ListView(t *testing.T) {
	m, execCtx := newProfileSettingsModel(t)
	if execCtx == nil {
		t.Fatalf("missing exec context")
	}

	svc := profile.NewService(store.NewProfileRepo(execCtx.DB))
	if _, err := svc.Create(context.Background(), "Alt Profile"); err != nil {
		t.Fatalf("create profile: %v", err)
	}

	m = openProfilesSubmenu(t, m)
	view := m.View().Content

	if !strings.Contains(view, "Profiles") {
		t.Fatalf("expected profiles list view")
	}
	if !strings.Contains(view, "Alt Profile") {
		t.Fatalf("expected profile name in list")
	}
}

func TestSettingsProfiles_KeyHints(t *testing.T) {
	m, _ := newProfileSettingsModel(t)
	m = openProfilesSubmenu(t, m)
	view := m.View().Content

	if !strings.Contains(view, "Ctrl+N") || !strings.Contains(view, "Ctrl+R") || !strings.Contains(view, "Ctrl+D") || !strings.Contains(view, "Enter") {
		t.Fatalf("expected keybinding hints in profile list")
	}
}
