package integration

import (
	"context"
	"testing"

	"noto/internal/vector"
)

type stubDeduper struct {
	result vector.DedupResult
	calls  int
}

func (s *stubDeduper) CheckDuplicate(_ context.Context, _ string, _ string) (vector.DedupResult, error) {
	s.calls++
	return s.result, nil
}

func TestVectorDedupResult(t *testing.T) {
	deduper := &stubDeduper{result: vector.DedupResult{IsDuplicate: true, MatchID: "note-1", Score: 0.92}}
	res, err := deduper.CheckDuplicate(context.Background(), "profile", "note content")
	if err != nil {
		t.Fatalf("CheckDuplicate error: %v", err)
	}
	if !res.IsDuplicate || res.MatchID != "note-1" {
		t.Fatalf("unexpected dedup result: %+v", res)
	}
}
