package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/provider"
	nototui "noto/internal/tui"
)

func newTestModel(listModels func(context.Context) ([]provider.ModelInfo, error)) nototui.Model {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}

	m := nototui.New(
		"Profile",
		"main-model",
		"extractor-model",
		"embeddings-model",
		"ctx:l1-hit",
		"↑0 ↓0 cr:0 cw:0 $0.000",
		false,
		false,
		dispatcher,
		execCtx,
		nil,
		func(ctx context.Context) ([]provider.ModelInfo, error) {
			if listModels != nil {
				return listModels(ctx)
			}
			return []provider.ModelInfo{{ID: "main-model"}, {ID: "other-model"}}, nil
		},
		func(context.Context) ([]provider.ModelInfo, error) {
			return []provider.ModelInfo{{ID: "embeddings-model"}}, nil
		},
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
		false,
	)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(nototui.Model)
}

func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func newTestModelWithSetup(needsSetup bool) nototui.Model {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}

	m := nototui.New(
		"Profile",
		"",
		"",
		"",
		"ctx:n/a",
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
		nil,
		needsSetup,
	)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(nototui.Model)
}
