package integration

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
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
	updated, _ = m.Update(tea.KeyPressMsg{Code: ',', Mod: tea.ModCtrl})
	m = updated.(tui.Model)

	view := m.View().Content
	if !strings.Contains(view, "Settings") {
		t.Fatalf("expected settings dialog to render")
	}
}

func TestSettingsEntries_AreSortedAlphabetically(t *testing.T) {
	entries := []tui.SettingsEntry{
		{Label: "Token Budget"},
		{Label: "Extractor Model"},
		{Label: "Model"},
	}
	tui.SortSettingsEntries(entries)
	if entries[0].Label != "Extractor Model" || entries[1].Label != "Model" || entries[2].Label != "Token Budget" {
		t.Fatalf("entries not sorted alphabetically")
	}
}
