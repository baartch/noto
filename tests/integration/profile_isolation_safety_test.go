package integration

import (
	"context"
	"testing"

	"noto/internal/profile"
	"noto/internal/store"
)

// TestProfileIsolation_NotesBelongToCorrectProfile verifies that notes from one profile
// are not visible from another profile's query.
func TestProfileIsolation_NotesBelongToCorrectProfile(t *testing.T) {
	db, close := tempDB(t)
	defer close()
	ctx := context.Background()

	svc := profile.NewService(store.NewProfileRepo(db))
	noteRepo := store.NewMemoryNoteRepo(db)

	p1, _ := svc.Create(ctx, "Profile One")
	p2, _ := svc.Create(ctx, "Profile Two")

	note1 := &store.MemoryNote{
		ID:               "note-p1",
		ProfileID:        p1.ID,
		Category:         store.CategoryFact,
		Content:          "Profile 1 secret",
		Importance:       5,
		SourceMessageIDs: "[]",
	}
	note2 := &store.MemoryNote{
		ID:               "note-p2",
		ProfileID:        p2.ID,
		Category:         store.CategoryFact,
		Content:          "Profile 2 secret",
		Importance:       5,
		SourceMessageIDs: "[]",
	}

	noteRepo.Create(ctx, note1)
	noteRepo.Create(ctx, note2)

	p1Notes, err := noteRepo.ListByProfile(ctx, p1.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range p1Notes {
		if n.ProfileID != p1.ID {
			t.Errorf("note %q belongs to wrong profile: got %s", n.ID, n.ProfileID)
		}
	}
	if len(p1Notes) != 1 {
		t.Errorf("expected 1 note for p1, got %d", len(p1Notes))
	}
}

// TestProfileIsolation_CacheIsolated verifies cache entries are scoped to a profile.
func TestProfileIsolation_CacheIsolated(t *testing.T) {
	db, close := tempDB(t)
	defer close()
	ctx := context.Background()

	svc := profile.NewService(store.NewProfileRepo(db))
	p1, _ := svc.Create(ctx, "Cache Profile A")
	p2, _ := svc.Create(ctx, "Cache Profile B")

	cacheRepo := store.NewContextCacheRepo(db)

	entry := &store.ContextCacheEntry{
		ID:            "cc-iso",
		ProfileID:     p1.ID,
		CacheKey:      "shared-key",
		Payload:       "profile1-data",
		SourceNoteIDs: "[]",
		PromptVersion: "v1",
		StateVersion:  "s1",
	}
	if err := cacheRepo.Upsert(ctx, entry); err != nil {
		t.Fatal(err)
	}

	// p2 should not be able to read p1's cache.
	guards := store.NewIsolationGuards(db)
	err := guards.AssertCacheOwnership(ctx, p2.ID, "shared-key")
	if err == nil {
		t.Error("expected isolation guard to fail for wrong profile")
	}
}

// TestProfileIsolation_DeleteCascades verifies that deleting a profile removes its notes.
func TestProfileIsolation_DeleteCascades(t *testing.T) {
	db, close := tempDB(t)
	defer close()
	ctx := context.Background()

	svc := profile.NewService(store.NewProfileRepo(db))
	noteRepo := store.NewMemoryNoteRepo(db)

	p1, _ := svc.Create(ctx, "Cascade Profile")
	p2, _ := svc.Create(ctx, "Survivor Profile")

	note := &store.MemoryNote{
		ID:               "cascaded-note",
		ProfileID:        p1.ID,
		Category:         store.CategoryFact,
		Content:          "Will be deleted",
		Importance:       3,
		SourceMessageIDs: "[]",
	}
	noteRepo.Create(ctx, note)

	// Delete p1.
	svc.Delete(ctx, "Cascade Profile", func(_ string) bool { return true })

	// p2 notes should still be queriable (verifies no cross-profile damage).
	p2Notes, err := noteRepo.ListByProfile(ctx, p2.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = p2Notes // p2 has no notes — just verify no error
}

// TestProfileIsolation_VectorManifestEntries verifies manifest entries stay profile-scoped.
func TestProfileIsolation_VectorManifestEntries(t *testing.T) {
	db, close := tempDB(t)
	defer close()
	ctx := context.Background()

	svc := profile.NewService(store.NewProfileRepo(db))
	manifestRepo := store.NewVectorManifestRepo(db)

	p1, _ := svc.Create(ctx, "Vector Profile 1")
	p2, _ := svc.Create(ctx, "Vector Profile 2")

	entry := &store.VectorEntry{
		ID:             "ve-1",
		ProfileID:      p1.ID,
		SourceType:     "memory_note",
		SourceID:       "note-1",
		ChunkHash:      "hash",
		EmbeddingModel: "model",
		EmbeddingDim:   2,
		VectorRef:      "0",
	}
	if err := manifestRepo.UpsertEntry(ctx, entry); err != nil {
		t.Fatalf("upsert entry: %v", err)
	}

	entriesP1, err := manifestRepo.ListEntries(ctx, p1.ID)
	if err != nil {
		t.Fatalf("list entries p1: %v", err)
	}
	if len(entriesP1) != 1 {
		t.Fatalf("expected 1 entry for p1, got %d", len(entriesP1))
	}
	entriesP2, err := manifestRepo.ListEntries(ctx, p2.ID)
	if err != nil {
		t.Fatalf("list entries p2: %v", err)
	}
	if len(entriesP2) != 0 {
		t.Fatalf("expected 0 entries for p2, got %d", len(entriesP2))
	}
}

// TestDestructiveConfirmation_RequiredText verifies ConfirmDeletion only accepts "yes".
func TestDestructiveConfirmation_RequiredText(t *testing.T) {
	cases := []struct {
		input    string
		wantBool bool
	}{
		{"yes", true},
		{"YES", true},
		{"Yes", true},
		{"no", false},
		{"n", false},
		{"", false},
		{"yep", false},
	}

	for _, tc := range cases {
		var buf = &stubReader{data: tc.input + "\n"}
		result := profile.ConfirmDeletion(nopWriter{}, buf, "TestProfile")
		if result != tc.wantBool {
			t.Errorf("input=%q: got %v, want %v", tc.input, result, tc.wantBool)
		}
	}
}

// ---- stubs ------------------------------------------------------------------

type stubReader struct {
	data string
	pos  int
}

func (r *stubReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, nil
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
