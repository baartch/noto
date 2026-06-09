package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/provider"
	"noto/internal/tui"
)

func TestFooterContextCapacity_UpdatesAfterUsageChange(t *testing.T) {
	m := newTestModel(nil)
	updated, _ := m.Update(tui.StatsUpdated(provider.Stats{TokensIn: 1000, TokensOut: 100, CostUSD: 0.1, ContextUsed: 1000, ContextMax: 2000}.Format()))
	m2 := updated.(tui.Model)
	view := m2.View().Content
	if !strings.Contains(view, "50%") {
		t.Fatalf("expected context percentage in footer, got: %s", view)
	}
	updated, _ = m2.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(tui.Model)
}
