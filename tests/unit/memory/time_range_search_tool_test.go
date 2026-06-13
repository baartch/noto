package memory_test

import (
	"context"
	"testing"
	"time"

	"noto/internal/memory"
	"noto/internal/store"
)

func TestTimeRangeSearchTool_InvalidRange(t *testing.T) {
	exec := memory.NewTimeRangeSearchTool(nil)
	_, err := exec.Execute(context.Background(), memory.TimeRangeSearchInput{
		StartTime: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatalf("expected invalid range error")
	}
}

func TestTimeRangeSearchTool_ReturnsRawNoteResults(t *testing.T) {
	notes := []*store.MemoryNote{{ID: "n1", Category: store.CategoryFact, Content: "June raw note", CreatedAt: time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)}}
	exec := memory.NewTimeRangeSearchTool(
		func(context.Context) ([]*store.MemoryNote, error) { return notes, nil },
	)
	results, err := exec.Execute(context.Background(), memory.TimeRangeSearchInput{
		StartTime: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC),
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 raw-note result, got %d", len(results))
	}
	if results[0].Content != "June raw note" {
		t.Fatalf("unexpected content: %q", results[0].Content)
	}
	if results[0].Importance != 0 {
		t.Fatalf("expected default importance 0 when note importance is unset, got %d", results[0].Importance)
	}
}
