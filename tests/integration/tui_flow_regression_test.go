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

func TestTUIModel_HandlesWindowResize(t *testing.T) {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}

	model := tui.New(
		"Profile",
		"",
		"",
		"cache: n/a",
		"tokens: n/a",
		true,
		dispatcher,
		execCtx,
		nil,
		nil,
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
	)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if _, ok := updated.(tui.Model); !ok {
		t.Fatalf("expected Update to return tui.Model")
	}
}

func TestTUIModel_TogglesHelp(t *testing.T) {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}

	model := tui.New(
		"Profile",
		"",
		"",
		"cache: n/a",
		"tokens: n/a",
		false,
		dispatcher,
		execCtx,
		nil,
		nil,
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
	)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl})
	m = updated.(tui.Model)

	view := m.View().Content
	if !strings.Contains(view, "help") {
		t.Fatalf("expected help view to be rendered when toggled")
	}
}

func TestTUIModel_OpenSettingsShortcut(t *testing.T) {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}

	model := tui.New(
		"Profile",
		"",
		"",
		"cache: n/a",
		"tokens: n/a",
		false,
		dispatcher,
		execCtx,
		nil,
		nil,
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
	)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	m = updated.(tui.Model)

	view := m.View().Content
	if !strings.Contains(view, "Settings") {
		t.Fatalf("expected settings dialog to render")
	}
}

func TestTUISettingsEditor_SaveAndCancel(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()

	repo := store.NewProfileRepo(db)
	svc := profile.NewService(repo)
	ctx := context.Background()
	p, err := svc.Create(ctx, "Settings Test")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	execCtx := &commands.ExecContext{ProfileID: p.ID, ProfileSlug: p.Slug, DB: db}
	model := tui.New(
		"Profile",
		"",
		"",
		"cache: n/a",
		"tokens: n/a",
		false,
		chat.NewDispatcher(commands.NewRegistry()),
		execCtx,
		nil,
		nil,
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
	)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	m = updated.(tui.Model)

	// Move to System Prompt (sorted) and open editor
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'N'})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'e'})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'w'})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)

	if !strings.Contains(m.View().Content, "New") {
		t.Fatalf("expected updated system prompt")
	}

	// Re-open editor and cancel
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'X'})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(tui.Model)

	if strings.Contains(m.View().Content, "X") {
		t.Fatalf("expected cancel to keep original value")
	}
}

func TestTUISettingsEditor_InvalidNumber(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()

	repo := store.NewProfileRepo(db)
	svc := profile.NewService(repo)
	ctx := context.Background()
	p, err := svc.Create(ctx, "Settings Test")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	execCtx := &commands.ExecContext{ProfileID: p.ID, ProfileSlug: p.Slug, DB: db}
	model := tui.New(
		"Profile",
		"",
		"",
		"cache: n/a",
		"tokens: n/a",
		false,
		chat.NewDispatcher(commands.NewRegistry()),
		execCtx,
		nil,
		nil,
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
	)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	m = updated.(tui.Model)

	// Memory Token Budget should be last; move down to it
	for i := 0; i < 4; i++ {
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		m = updated.(tui.Model)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a'})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)

	if !strings.Contains(m.View().Content, "positive number") {
		t.Fatalf("expected validation error")
	}
}

func TestSettingsSubmenuNavigation_EscBehavior(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()

	repo := store.NewProfileRepo(db)
	svc := profile.NewService(repo)
	ctx := context.Background()
	p, err := svc.Create(ctx, "Settings Test")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	execCtx := &commands.ExecContext{ProfileID: p.ID, ProfileSlug: p.Slug, DB: db}
	model := tui.New(
		"Profile",
		"",
		"",
		"cache: n/a",
		"tokens: n/a",
		false,
		chat.NewDispatcher(commands.NewRegistry()),
		execCtx,
		nil,
		nil,
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
	)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	m = updated.(tui.Model)

	// Move to Providers and enter submenu
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)

	if !strings.Contains(m.View().Content, "Providers") {
		t.Fatalf("expected providers submenu")
	}

	// Esc returns to root
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(tui.Model)
	if !strings.Contains(m.View().Content, "Settings") {
		t.Fatalf("expected root settings after esc")
	}

	// Esc closes dialog
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(tui.Model)
	if strings.Contains(m.View().Content, "Settings") {
		t.Fatalf("expected settings dialog closed")
	}
}

func TestSettingsEntries_AreSortedAlphabetically(t *testing.T) {
	entries := []tui.SettingsEntry{
		{Label: "Token Budget"},
		{Label: "Model Extractor"},
		{Label: "Model"},
	}
	tui.SortSettingsEntries(entries)
	if entries[0].Label != "Model Extractor" || entries[1].Label != "Model" || entries[2].Label != "Token Budget" {
		t.Fatalf("entries not sorted alphabetically")
	}
}
