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
	"noto/tests/integration/testutil"
)

func TestTUIModel_HandlesWindowResize(t *testing.T) {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}

	model := tui.New(
		"Profile",
		"",
		"",
		"",
		"cache: n/a",
		"tokens: n/a",
		true,
		false,
		dispatcher,
		execCtx,
		nil,
		nil,
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil, false,
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
		"",
		"cache: n/a",
		"tokens: n/a",
		false,
		false,
		dispatcher,
		execCtx,
		nil,
		nil,
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil, false,
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
		"",
		"cache: n/a",
		"tokens: n/a",
		false,
		false,
		dispatcher,
		execCtx,
		nil,
		nil,
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil, false,
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

func newSettingsModel(t *testing.T) (tui.Model, *commands.ExecContext) {
	t.Helper()
	db, closeDB := testutil.TempDB(t)
	t.Cleanup(closeDB)

	repo := store.NewProfileRepo(db)
	svc := profile.NewService(repo)
	p, err := svc.Create(context.Background(), "Settings Test")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	registry := commands.NewRegistry()
	if err := commands.RegisterPromptCommands(registry); err != nil {
		t.Fatalf("register prompt commands: %v", err)
	}
	if err := commands.RegisterMemoryCommands(registry); err != nil {
		t.Fatalf("register memory commands: %v", err)
	}
	if err := commands.RegisterModelCommand(registry); err != nil {
		t.Fatalf("register model commands: %v", err)
	}
	if err := commands.RegisterProviderCommands(registry); err != nil {
		t.Fatalf("register provider commands: %v", err)
	}
	if err := commands.RegisterModelExtractorCommand(registry); err != nil {
		t.Fatalf("register extractor commands: %v", err)
	}
	if err := commands.RegisterBackupCommands(registry); err != nil {
		t.Fatalf("register backup commands: %v", err)
	}

	execCtx := &commands.ExecContext{ProfileID: p.ID, ProfileSlug: p.Slug, DB: db}
	model := tui.New(
		"Profile", "", "", "", "cache: n/a", "tokens: n/a", false, false,
		chat.NewDispatcher(registry),
		execCtx,
		nil, nil, nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil, false,
	)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := updated.(tui.Model)
	return m, execCtx
}

func TestTUISettingsEditor_SaveAndCancel(t *testing.T) {
	m, _ := newSettingsModel(t)

	// Open settings
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	m = updated.(tui.Model)

	// Navigate down until we land on a value entry and enter it
	// Sorted order: Model, Model Embeddings, Model Extractor, Profiles, Provider,
	// System Prompt, Themes, Timeline
	// System Prompt is index 5 (0-based) — navigate down 5 times from top
	for range 5 {
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		m = updated.(tui.Model)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)

	// Should now be in editor — Esc cancels, settings still open
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(tui.Model)
	if !strings.Contains(m.View().Content, "Settings") {
		t.Fatalf("expected settings dialog still open after cancel")
	}
}

func TestTUISettingsEditor_InvalidNumber(t *testing.T) {
	m, _ := newSettingsModel(t)

	// Open settings
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	m = updated.(tui.Model)

	// Move to the Timeline submenu, then enter the first numeric timeline setting.
	// Timeline is index 7 in the sorted root settings list.
	for range 7 {
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		m = updated.(tui.Model)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)

	// Send a no-op update to let the textarea initialize
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)

	// Type invalid value then press Enter to save
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)

	if !strings.Contains(m.View().Content, "positive number") {
		t.Fatalf("expected validation error")
	}
}

func TestSettingsSubmenuNavigation_EscBehavior(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
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
		"",
		"cache: n/a",
		"tokens: n/a",
		false,
		false,
		chat.NewDispatcher(commands.NewRegistry()),
		execCtx,
		nil,
		nil,
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil, false,
	)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	m = updated.(tui.Model)

	// Sorted: Model(0), Model Embeddings(1), Model Extractor(2), Profiles(3),
	// Provider(4), System Prompt(5), Themes(6), Timeline(7)
	// Navigate to Provider (index 4)
	for range 4 {
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		m = updated.(tui.Model)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)

	// Inside provider submenu — should show its entries
	view := m.View().Content
	if !strings.Contains(view, "Endpoint") && !strings.Contains(view, "Key") {
		t.Fatalf("expected provider submenu entries, got: %s", view)
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
		{Label: "Model Embeddings"},
	}
	tui.SortSettingsEntries(entries)
	if entries[0].Label != "Model" || entries[1].Label != "Model Embeddings" || entries[2].Label != "Model Extractor" || entries[3].Label != "Token Budget" {
		t.Fatalf("entries not sorted alphabetically, got: %s, %s, %s, %s", entries[0].Label, entries[1].Label, entries[2].Label, entries[3].Label)
	}
}
