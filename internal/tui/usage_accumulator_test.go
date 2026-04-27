package tui

import (
	"testing"

	"noto/internal/provider"
)

func TestUsageAccumulator_AddFromUsage(t *testing.T) {
	var acc usageAccumulator
	acc.addFromUsage(provider.Usage{
		PromptTokens:     10,
		CompletionTokens: 3,
		CachedTokens:     2,
		CacheWriteTokens: 1,
		Cost:             0.12,
		HasUsage:         true,
	})
	if acc.up != 10 || acc.down != 3 || acc.cacheRead != 2 || acc.cacheWrite != 1 {
		t.Fatalf("unexpected accumulator values: %+v", acc)
	}
}

func TestUsageAccumulator_SkipsMissingUsage(t *testing.T) {
	var acc usageAccumulator
	acc.addFromUsage(provider.Usage{HasUsage: false})
	if acc.up != 0 || acc.down != 0 || acc.cacheRead != 0 || acc.cacheWrite != 0 || acc.cost != 0 {
		t.Fatalf("expected unchanged accumulator, got %+v", acc)
	}
}
