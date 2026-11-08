package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/provider"
	"noto/internal/tui"
)

// openExtractorPicker opens the extractor-model picker the same way a user
// does: Ctrl+J opens settings, two Downs reach the "Model Extractor" entry
// (sorted root order: Model, Model Embeddings, Model Extractor, ...), Enter
// opens the picker, and the returned load command runs to completion.
func openExtractorPicker(t *testing.T, m tui.Model) tui.Model {
	t.Helper()
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	m = updated.(tui.Model)
	for range 2 {
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		m = updated.(tui.Model)
	}
	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(tui.Model)
	if msg := runCmd(cmd); msg != nil {
		updated, _ = m.Update(msg)
		m = updated.(tui.Model)
	}
	return m
}

func TestExtractorPicker_ListsOnlyStructuredOutputsModels(t *testing.T) {
	m := newTestModel(func(context.Context) ([]provider.ModelInfo, error) {
		return []provider.ModelInfo{
			{ID: "good-model", SupportsStructuredOutputs: true},
			{ID: "bad-model"},
			{ID: "also-bad"},
		}, nil
	})
	m = openExtractorPicker(t, m)
	view := m.View().Content
	if !strings.Contains(view, "good-model") {
		t.Fatalf("expected eligible model in extractor picker, got: %s", view)
	}
	if strings.Contains(view, "bad-model") || strings.Contains(view, "also-bad") {
		t.Fatal("extractor picker must list only structured-outputs models")
	}
	if strings.Contains(view, "no models support structured outputs") {
		t.Fatal("expected no picker error while eligible models exist")
	}
}

func TestExtractorPicker_NoEligibleModels_ShowsError(t *testing.T) {
	m := newTestModel(func(context.Context) ([]provider.ModelInfo, error) {
		return []provider.ModelInfo{{ID: "bad-model"}}, nil
	})
	m = openExtractorPicker(t, m)
	view := m.View().Content
	if !strings.Contains(view, "no models support structured outputs on this endpoint") {
		t.Fatalf("expected picker error for ineligible models, got: %s", view)
	}
	if strings.Contains(view, "bad-model") {
		t.Fatal("picker item list must be empty when no models are eligible")
	}
}

func TestFooter_ExtractorIneligibleWarning(t *testing.T) {
	m := newTestModel(nil)
	// Widen the window so the footer's left side is not width-truncated
	// (the full warning needs more than the default 120 columns).
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = updated.(tui.Model)
	m.SetExtractorIneligible(true)
	if !strings.Contains(m.View().Content, "Extractor model lacks structured outputs") {
		t.Fatal("footer missing extractor warning")
	}
	m.SetExtractorIneligible(false)
	if strings.Contains(m.View().Content, "Extractor model lacks structured outputs") {
		t.Fatal("footer must not show warning when eligible")
	}
}

func TestExtractorEligibilityUpdatedMsg_TogglesFooter(t *testing.T) {
	m := newTestModel(nil)
	// Widen the window so the footer's left side is not width-truncated.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.ExtractorEligibilityUpdated(false))
	if !strings.Contains(updated.(tui.Model).View().Content, "Extractor model lacks structured outputs") {
		t.Fatal("expected ineligible warning after ExtractorEligibilityUpdated(false)")
	}
	updated, _ = updated.Update(tui.ExtractorEligibilityUpdated(true))
	if strings.Contains(updated.(tui.Model).View().Content, "Extractor model lacks structured outputs") {
		t.Fatal("expected warning gone after ExtractorEligibilityUpdated(true)")
	}
}
