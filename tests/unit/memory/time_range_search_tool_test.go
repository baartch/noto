package memory_test

import (
	"context"
	"testing"
	"time"

	"noto/internal/memory"
	"noto/internal/store"
)

func TestTimeRangeSearchTool_InvalidRange(t *testing.T) {
	exec := memory.NewTimeRangeSearchTool(nil, nil)
	_, err := exec.Execute(context.Background(), memory.TimeRangeSearchInput{
		StartTime: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatalf("expected invalid range error")
	}
}

func TestTimeRangeSearchTool_ReturnsMixedRawAndSummaryResults(t *testing.T) {
	notes := []*store.MemoryNote{{ID: "n1", Category: store.CategoryFact, Content: "June raw note", CreatedAt: time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)}}
	summaries := []*store.MemorySummary{{ID: "w1", SummaryType: store.SummaryTypeWeekly, PeriodKey: "2026-W23", Content: "Week summary", PeriodStart: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)}}
	exec := memory.NewTimeRangeSearchTool(
		func(context.Context) ([]*store.MemoryNote, error) { return notes, nil },
		func(context.Context) ([]*store.MemorySummary, error) { return summaries, nil },
	)
	results, err := exec.Execute(context.Background(), memory.TimeRangeSearchInput{
		StartTime: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC),
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 mixed results, got %d", len(results))
	}
}
