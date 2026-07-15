package tui_test

import (
	"context"
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

func newHistoryTestModel() tui.Model {
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

func makeMessages(n int) []*store.Message {
	base := time.Now().Add(-time.Hour)
	out := make([]*store.Message, 0, n)
	for i := 1; i <= n; i++ {
		role := store.RoleUser
		if i%2 == 0 {
			role = store.RoleAssistant
		}
		out = append(out, &store.Message{
			ID:             fmt.Sprintf("m-%02d", i),
			ConversationID: "conv-1",
			Role:           role,
			Content:        fmt.Sprintf("message-%02d", i),
			CreatedAt:      base.Add(time.Duration(i) * time.Minute),
		})
	}
	return out
}

func TestStartupHistory_UsesLatestTenInChronologicalOrder(t *testing.T) {
	m := newHistoryTestModel()
	msgs := makeMessages(12)
	m.SetStartupConversationHistory(msgs, nil)

	view := m.View().Content
	if strings.Contains(view, "message-01") || strings.Contains(view, "message-02") {
		t.Fatalf("expected oldest messages to be excluded from startup window")
	}
	for i := 3; i <= 12; i++ {
		want := fmt.Sprintf("message-%02d", i)
		if !strings.Contains(view, want) {
			t.Fatalf("expected %s in view", want)
		}
	}
}

func TestStartupHistory_AllowsEmptyWithoutError(t *testing.T) {
	m := newHistoryTestModel()
	m.SetStartupConversationHistory(nil, nil)
	view := m.View().Content
	if !strings.Contains(view, "No messages yet") {
		t.Fatalf("expected empty-state message")
	}
}

func TestStartupHistory_NonFatalErrorShown(t *testing.T) {
	m := newHistoryTestModel()
	m.SetStartupConversationHistory(nil, context.DeadlineExceeded)
	view := m.View().Content
	if !strings.Contains(view, "deadline exceeded") {
		t.Fatalf("expected non-fatal history error in view")
	}
}
