package memory_test

import (
	"testing"

	"noto/internal/memory"
	"noto/internal/store"
)

func TestCacheKey_ChangesWithTimelineState(t *testing.T) {
	settingsA := &store.TimelineSettings{ProfileID: "p", RawNoteDays: 30, WeeklySummaryWeeks: 8, MonthlySummaryMonths: store.MonthlySummaryAllRemaining}
	settingsB := &store.TimelineSettings{ProfileID: "p", RawNoteDays: 14, WeeklySummaryWeeks: 8, MonthlySummaryMonths: store.MonthlySummaryAllRemaining}
	stateA := memory.BuildTimelineStateHashForTest([]string{"n1"}, nil, nil, settingsA)
	stateB := memory.BuildTimelineStateHashForTest([]string{"n1"}, nil, nil, settingsB)
	if stateA == stateB {
		t.Fatalf("expected different timeline state hashes")
	}
}

func TestCacheKey_ChangesWithSummaryState(t *testing.T) {
	settings := &store.TimelineSettings{ProfileID: "p", RawNoteDays: 30, WeeklySummaryWeeks: 8, MonthlySummaryMonths: store.MonthlySummaryAllRemaining}
	stateA := memory.BuildTimelineStateHashForTest([]string{"n1"}, []string{"w1:fresh"}, nil, settings)
	stateB := memory.BuildTimelineStateHashForTest([]string{"n1"}, []string{"w1:stale"}, nil, settings)
	if stateA == stateB {
		t.Fatalf("expected summary freshness to affect state hash")
	}
}
