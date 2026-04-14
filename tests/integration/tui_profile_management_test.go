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

func newProfileSettingsModel(t *testing.T) (tui.Model, *commands.ExecContext, *profile.Service) {
	t.Helper()
	db, closeDB := tempDB(t)
	t.Cleanup(closeDB)

	repo := store.NewProfileRepo(db)
	svc := profile.NewService(repo)
	ctx := context.Background()
	p, err := svc.Create(ctx, "Profile")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if _, err := svc.Select(ctx, p.Name); err != nil {
		t.Fatalf("select profile: %v", err)
	}

	execCtx := &commands.ExecContext{ProfileID: p.ID, ProfileSlug: p.Slug, DB: db}
	m := tui.New(
		p.Name, "", "", "cache: n/a", "tokens: n/a", false, false,
		chat.NewDispatcher(commands.NewRegistry()),
		execCtx,
		nil, nil,
		func(string) error { return nil },
		svc,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
	)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = asModel(t, updated)
	return m, execCtx, svc
}

func TestSettingsProfiles_ListView(t *testing.T) {
	m, execCtx, svc := newProfileSettingsModel(t)
	if execCtx == nil {
		t.Fatalf("missing exec context")
	}

	if _, err := svc.Create(context.Background(), "Alt Profile"); err != nil {
		t.Fatalf("create profile: %v", err)
	}

	m = openProfilesSubmenu(t, m)
	view := m.View().Content
	t.Logf("profiles view: %s", view)

	if !strings.Contains(view, "Profile") {
		t.Fatalf("expected profiles list view")
	}
	if !strings.Contains(view, "Alt Profile") {
		t.Fatalf("expected profile name in list")
	}
}

func TestSettingsProfiles_KeyHints(t *testing.T) {
	m, _, _ := newProfileSettingsModel(t)
	m = openProfilesSubmenu(t, m)
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl})
	m = asModel(t, updated)
	view := m.View().Content

	if !strings.Contains(view, "ctrl+n") || !strings.Contains(view, "ctrl+r") || !strings.Contains(view, "ctrl+d") || !strings.Contains(view, "enter") {
		t.Fatalf("expected keybinding hints in profile list")
	}
}
