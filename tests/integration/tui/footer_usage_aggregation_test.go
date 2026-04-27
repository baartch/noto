package tui

import (
	"strings"
	"testing"

	"noto/internal/provider"
	"noto/internal/tui"
)

func TestFooterUsageAggregation_AcrossSources(t *testing.T) {
	m := newTestModel(nil)

	updated, _ := m.Update(tui.UsageUpdatedMain(provider.Usage{
		PromptTokens: 10, CompletionTokens: 2, CachedTokens: 1, CacheWriteTokens: 3, Cost: 0.10, HasUsage: true,
	}))
	m2 := updated.(tui.Model)
	updated, _ = m2.Update(tui.UsageUpdatedExtractor(provider.Usage{
		PromptTokens: 4, CompletionTokens: 1, CachedTokens: 0, CacheWriteTokens: 2, Cost: 0.05, HasUsage: true,
	}))
	m2 = updated.(tui.Model)
	updated, _ = m2.Update(tui.UsageUpdatedEmbeddings(provider.Usage{
		PromptTokens: 6, CompletionTokens: 0, CachedTokens: 2, CacheWriteTokens: 1, Cost: 0.03, HasUsage: true,
	}))
	m2 = updated.(tui.Model)

	view := m2.View().Content
	if !strings.Contains(view, "↑20") || !strings.Contains(view, "↓3") || !strings.Contains(view, "R3") || !strings.Contains(view, "W6") {
		t.Fatalf("expected aggregated usage values in footer, got: %s", view)
	}
}
