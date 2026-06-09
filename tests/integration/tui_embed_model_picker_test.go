package integration

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/provider"
	"noto/internal/tui"
)

func TestEmbeddingsModelPicker_SelectsModel(t *testing.T) {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}

	selected := ""
	listEmbeddings := func(context.Context) ([]provider.ModelInfo, error) {
		return []provider.ModelInfo{{ID: "embed-1"}}, nil
	}
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
		listEmbeddings,
		func(string) error { return nil },
		func(modelID string) error { selected = modelID; return nil },
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
	for range 1 {
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		m = updated.(tui.Model)
	}
	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatalf("expected picker command")
	}
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if selected != "embed-1" {
		t.Fatalf("expected selected model, got %q", selected)
	}
}
