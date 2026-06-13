package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/tui"
	"noto/internal/version"
)

func TestFooterIncludesVersion(t *testing.T) {
	old := version.Version
	version.Version = "v9.9.9"
	t.Cleanup(func() { version.Version = old })

	dispatcher := chat.NewDispatcher(commands.NewRegistry())
	m := tui.New(
		"Profile", "", "", "", "cache: n/a", "tokens: n/a", false, false,
		dispatcher, &commands.ExecContext{},
		nil, nil, nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
	)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := updated.(tui.Model)
	view := model.View().Content
	if !strings.Contains(view, "v9.9.9") {
		t.Fatalf("expected footer to contain version")
	}
}
