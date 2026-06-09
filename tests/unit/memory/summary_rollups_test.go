package memory_test

import (
	"testing"
	"time"

	"noto/internal/memory"
	"noto/internal/store"
)

func TestWeeklyAndMonthlyPeriodKeys(t *testing.T) {
	ts := time.Date(2026, 6, 9, 15, 0, 0, 0, time.UTC)
	if got, want := memory.WeeklyPeriodKey(ts), "2026-W24"; got != want {
		t.Fatalf("weekly key = %q, want %q", got, want)
	}
	if got, want := memory.MonthlyPeriodKey(ts), "2026-06"; got != want {
		t.Fatalf("monthly key = %q, want %q", got, want)
	}
}

func TestIsSummaryFresh(t *testing.T) {
	state := memory.SummaryStateVersion([]string{"n1", "n2"})
	s := &store.MemorySummary{FreshnessState: store.SummaryFresh, SourceStateVersion: state}
	if !memory.IsSummaryFresh(s, state) {
		t.Fatalf("expected summary to be fresh")
	}
	if memory.IsSummaryFresh(&store.MemorySummary{FreshnessState: store.SummaryStale, SourceStateVersion: state}, state) {
		t.Fatalf("expected stale summary not to be fresh")
	}
}

func TestSummaryStateVersion_ChangesWithInput(t *testing.T) {
	a := memory.SummaryStateVersion([]string{"n1", "n2"})
	b := memory.SummaryStateVersion([]string{"n1", "n3"})
	if a == b {
		t.Fatalf("expected different state versions")
	}
}
