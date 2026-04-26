package integration

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/tui"
)

func TestStartupHistoryFailure_DoesNotBlockInputFlow(t *testing.T) {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}
	m := tui.New(
		"Profile", "", "", "", "cache: n/a", "tokens: n/a", false, false,
		dispatcher, execCtx,
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
	m = updated.(tui.Model)
	m.SetStartupConversationHistory(nil, context.DeadlineExceeded)

	view := m.View().Content
	if !strings.Contains(view, "deadline exceeded") {
		t.Fatalf("expected non-fatal history error in view")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Text: "h", Code: 'h'})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Text: "i", Code: 'i'})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = updated.(tui.Model)
}
