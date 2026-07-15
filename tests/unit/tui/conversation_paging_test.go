package tui_test

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

func newPagingModel() tui.Model {
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
	return updated.(tui.Model)
}

func makePagedMessages(n int) []*store.Message {
	base := time.Now().Add(-time.Hour)
	out := make([]*store.Message, 0, n)
	for i := 1; i <= n; i++ {
		role := store.RoleUser
		if i%2 == 0 {
			role = store.RoleAssistant
		}
		out = append(out, &store.Message{
			ID:             fmt.Sprintf("p-%02d", i),
			ConversationID: "conv",
			Role:           role,
			Content:        fmt.Sprintf("pmsg-%02d", i),
			CreatedAt:      base.Add(time.Duration(i) * time.Minute),
		})
	}
	return out
}

func TestConversationPaging_LazyLoadsOlderAtTop(t *testing.T) {
	m := newPagingModel()
	m.SetStartupConversationHistory(makePagedMessages(25), nil)
	before := m.View().Content
	if strings.Contains(before, "pmsg-01") {
		t.Fatalf("expected only newest slice initially")
	}

	for range 200 {
		updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
		m = updated.(tui.Model)
	}
	after := m.View().Content
	if !strings.Contains(after, "pmsg-01") {
		t.Fatalf("expected oldest message after repeated paging up")
	}
}
