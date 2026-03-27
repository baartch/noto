package integration

import (
	"context"
	"testing"

	"noto/internal/vector"
)

// stubManifestStatusSetter records the last status set.
type stubManifestStatusSetter struct {
	lastStatus string
}

func (s *stubManifestStatusSetter) SetManifestStatusStr(_ context.Context, _ string, status string) error {
	s.lastStatus = status
	return nil
}

func TestVectorRebuild_SetsStatusReady(t *testing.T) {
	ctx := context.Background()
	setter := &stubManifestStatusSetter{}
	index := vector.NoopIndex{}
	profileID := "rebuild-profile"

	rebuilder := vector.NewRebuilder(setter, index, profileID)
	notes := []vector.MemoryNoteRecord{
		{ID: "n1", Content: "Note one"},
		{ID: "n2", Content: "Note two"},
	}

	if err := rebuilder.Rebuild(ctx, notes); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if setter.lastStatus != string(vector.ManifestReady) {
		t.Errorf("expected status=%q after rebuild, got %q", vector.ManifestReady, setter.lastStatus)
	}
}

func TestVectorRebuild_EmptyNotes_StillSucceeds(t *testing.T) {
	ctx := context.Background()
	setter := &stubManifestStatusSetter{}
	index := vector.NoopIndex{}

	rebuilder := vector.NewRebuilder(setter, index, "profile-empty")
	if err := rebuilder.Rebuild(ctx, nil); err != nil {
		t.Fatalf("Rebuild with no notes: %v", err)
	}
	if setter.lastStatus != string(vector.ManifestReady) {
		t.Errorf("expected ready, got %q", setter.lastStatus)
	}
}

func TestVectorFallback_NoIndex_ReturnsNotesFromList(t *testing.T) {
	ctx := context.Background()
	notes := []vector.MemoryNoteRecord{
		{ID: "a", Content: "A"},
		{ID: "b", Content: "B"},
	}
	lister := &stubNoteLister{notes: notes}
	index := vector.NoopIndex{}
	retrieval := vector.NewHybridRetrieval(index, lister, "fallback-profile")

	results, err := retrieval.Retrieve(ctx, nil, 10)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 fallback results, got %d", len(results))
	}
}

func TestVectorInvalidation_OnPromptChange_MarksStale(t *testing.T) {
	ctx := context.Background()
	setter := &stubManifestStatusSetter{}
	triggers := vector.NewInvalidationTriggers(setter, "inv-profile")

	if err := triggers.OnPromptChange(ctx); err != nil {
		t.Fatalf("OnPromptChange: %v", err)
	}
	if setter.lastStatus != string(vector.ManifestStale) {
		t.Errorf("expected stale, got %q", setter.lastStatus)
	}
}
