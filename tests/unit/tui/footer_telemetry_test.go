package tui

import (
	"strings"
	"testing"

	"noto/internal/provider"
)

func TestProviderStatsFormat_KnownContextCapacity(t *testing.T) {
	stats := provider.Stats{TokensIn: 1200, TokensOut: 300, CostUSD: 0.123, ContextUsed: 1200, ContextMax: 4000}
	line := stats.Format()
	if !strings.Contains(line, "30%") || !strings.Contains(line, "4.0k") {
		t.Fatalf("expected percent and max context in %q", line)
	}
}

func TestProviderStatsFormat_UnknownContextCapacity(t *testing.T) {
	stats := provider.Stats{TokensIn: 1200, TokensOut: 300, CostUSD: 0.123, ContextUsed: 1200, ContextMax: 0}
	line := stats.Format()
	if strings.Contains(line, "%/") {
		t.Fatalf("did not expect percentage when context max unknown: %q", line)
	}
}
