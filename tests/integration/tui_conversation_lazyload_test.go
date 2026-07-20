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

func newLazyloadModel() tui.Model {
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

func makeLongConversation(n int) []*store.Message {
	base := time.Now().Add(-2 * time.Hour)
	msgs := make([]*store.Message, 0, n)
	for i := 1; i <= n; i++ {
		role := store.RoleUser
		if i%2 == 0 {
			role = store.RoleAssistant
		}
		msgs = append(msgs, &store.Message{
			ID:             fmt.Sprintf("l-%02d", i),
			ConversationID: "long",
			Role:           role,
			Content:        fmt.Sprintf("long-%02d", i),
			CreatedAt:      base.Add(time.Duration(i) * time.Minute),
		})
	}
	return msgs
}

func TestConversationLazyLoad_PrependsOlderMessages(t *testing.T) {
	m := newLazyloadModel()
	m.SetStartupConversationHistory(makeLongConversation(25), nil)

	before := m.View().Content
	if strings.Contains(before, "long-01") {
		t.Fatalf("expected long-01 not visible initially")
	}

	for range 220 {
		updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
		m = updated.(tui.Model)
	}
	after := m.View().Content
	if !strings.Contains(after, "long-01") {
		t.Fatalf("expected long-01 after lazy loading older batches")
	}
}

func TestConversationLazyLoad_NoFurtherLoadsAtHistoryTop(t *testing.T) {
	m := newLazyloadModel()
	m.SetStartupConversationHistory(makeLongConversation(12), nil)
	for range 220 {
		updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
		m = updated.(tui.Model)
	}
	view := m.View().Content
	if !strings.Contains(view, "long-01") {
		t.Fatalf("expected to remain at top without crashing/no-op")
	}
}
