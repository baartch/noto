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
	if got := acc.formatTokenStatus(); got != "↑10 ↓3 R2 W1 $0.120" {
		t.Fatalf("unexpected status format: %s", got)
	}
}

func TestUsageAccumulator_SkipsMissingUsage(t *testing.T) {
	var acc usageAccumulator
	acc.addFromUsage(provider.Usage{HasUsage: false})
	if acc.up != 0 || acc.down != 0 || acc.cacheRead != 0 || acc.cacheWrite != 0 || acc.cost != 0 {
		t.Fatalf("expected unchanged accumulator, got %+v", acc)
	}
	if got := acc.formatTokenStatus(); got != "↑0 ↓0 $0.000" {
		t.Fatalf("unexpected status format: %s", got)
	}
}

func TestFormatCompactTokens_RoundsKAndM(t *testing.T) {
	cases := map[int]string{
		175000:   "175k",
		51000:    "51k",
		16000000: "16M",
		999:      "999",
	}
	for n, want := range cases {
		if got := formatCompactTokens(n); got != want {
			t.Fatalf("formatCompactTokens(%d) = %s, want %s", n, got, want)
		}
	}
}
