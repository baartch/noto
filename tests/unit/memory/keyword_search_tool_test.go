package memory_test

import (
	"context"
	"testing"
	"time"

	"noto/internal/memory"
	"noto/internal/store"
)

func TestKeywordSearchTool_EmptyQueryReturnsNoResults(t *testing.T) {
	exec := memory.NewKeywordSearchTool(nil)
	results, err := exec.Execute(context.Background(), memory.KeywordSearchInput{Query: ""})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results, got %d", len(results))
	}
}

func TestKeywordSearchTool_FallbackReturnsDeterministicResults(t *testing.T) {
	notes := []*store.MemoryNote{
		{ID: "n1", Category: store.CategoryFact, Content: "launch checklist", Importance: 9, CreatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "n2", Category: store.CategoryFact, Content: "launch retrospective", Importance: 5, CreatedAt: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)},
	}
	exec := memory.NewKeywordSearchTool(func(context.Context) ([]*store.MemoryNote, error) { return notes, nil })
	results, err := exec.Execute(context.Background(), memory.KeywordSearchInput{Query: "launch", Limit: 5})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].RecordID != "n1" {
		t.Fatalf("expected highest-importance result first, got %q", results[0].RecordID)
	}
}
