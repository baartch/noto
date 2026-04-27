package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"noto/internal/provider"
	"noto/internal/tui"
)

func TestPickerFilter_SlashUpdatesListInPlace(t *testing.T) {
	m := newTestModel(func(context.Context) ([]provider.ModelInfo, error) {
		return []provider.ModelInfo{{ID: "alpha"}, {ID: "beta"}, {ID: "gamma"}}, nil
	})

	updatedModel, cmd := m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	updated := updatedModel.(tui.Model)
	if msg := runCmd(cmd); msg != nil {
		updatedModel, _ = updated.Update(msg)
		updated = updatedModel.(tui.Model)
	}

	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: '/'})
	updated = updatedModel.(tui.Model)
	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	updated = updatedModel.(tui.Model)

	view := updated.View().Content
	if !strings.Contains(view, "alpha") {
		t.Fatalf("expected filtered list to keep matching items visible")
	}
}
