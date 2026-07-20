package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/store"
	"noto/internal/tui"
)

func TestConversationAnchor_RemainsStableAfterPrepend(t *testing.T) {
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
		nil, false,
	)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)

	base := time.Now().Add(-time.Hour)
	msgs := make([]*store.Message, 0, 20)
	for i := 1; i <= 20; i++ {
		msgs = append(msgs, &store.Message{ID: fmt.Sprintf("a-%d", i), ConversationID: "a", Role: store.RoleAssistant, Content: fmt.Sprintf("anchor-%02d", i), CreatedAt: base.Add(time.Duration(i) * time.Minute)})
	}
	m.SetStartupConversationHistory(msgs, nil)
	before := m.View().Content

	for range 220 {
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
		m = updated.(tui.Model)
	}
	after := m.View().Content

	if !strings.Contains(before, "anchor-20") {
		t.Fatalf("expected baseline view to include latest message")
	}
	if !strings.Contains(after, "anchor-01") {
		t.Fatalf("expected older content to be prepended while preserving readable viewport")
	}
}
