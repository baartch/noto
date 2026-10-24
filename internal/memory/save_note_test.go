package memory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"noto/internal/store"
	"noto/internal/vector"
)

type saveNoteStubDeduper struct {
	result vector.DedupResult
	calls  int
}

func (d *saveNoteStubDeduper) CheckDuplicate(_ context.Context, _ string, _ string) (vector.DedupResult, error) {
	d.calls++
	return d.result, nil
}

func openSaveNoteTestDB(t *testing.T) (*store.DB, *store.MemoryNoteRepo) {
	t.Helper()
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "notes.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, store.NewMemoryNoteRepo(db)
}

func TestSaveNoteExplicitStoresLowScoreContent(t *testing.T) {
	_, repo := openSaveNoteTestDB(t)
	ctx := context.Background()

	content := "User keeps spare keys under the flowerpot"
	candidate := EvaluateCandidate(content, 3, []string{content})
	if candidate.ValueScore.Total >= MinValueScore {
		t.Fatalf("precondition failed: content scores %d, want < %d; pick weaker content", candidate.ValueScore.Total, MinValueScore)
	}

	res, err := SaveNoteExplicit(ctx, repo, nil, nil, "p1", "conv-1", SaveNoteInput{Content: content, Importance: 3}, []string{"msg-u1"})
	if err != nil {
		t.Fatalf("SaveNoteExplicit() error: %v", err)
	}
	if res == nil || res.Note == nil {
		t.Fatalf("SaveNoteExplicit() = %#v, want a stored note", res)
	}
	if res.Updated {
		t.Fatalf("SaveNoteExplicit() Updated = true, want false")
	}
	if res.Note.ProfileID != "p1" || res.Note.ConversationID != "conv-1" {
		t.Fatalf("note bound to wrong profile/conversation: %#v", res.Note)
	}
	notes, err := repo.ListByProfile(ctx, "p1")
	if err != nil || len(notes) != 1 {
		t.Fatalf("ListByProfile() = %d notes, %v; want 1", len(notes), err)
	}
}

func TestSaveNoteExplicitDuplicateUpdatesExisting(t *testing.T) {
	_, repo := openSaveNoteTestDB(t)
	ctx := context.Background()

	seed := &store.MemoryNote{
		ID: "mn-seed", ProfileID: "p1", ConversationID: "conv-1",
		Category: store.CategoryFact, Content: "User lives in Lisbon", Importance: 5,
		SourceMessageIDs: `["msg-old"]`,
	}
	if err := repo.Create(ctx, seed); err != nil {
		t.Fatal(err)
	}

	deduper := &saveNoteStubDeduper{result: vector.DedupResult{IsDuplicate: true, MatchID: "mn-seed"}}
	res, err := SaveNoteExplicit(ctx, repo, deduper, nil, "p1", "conv-1",
		SaveNoteInput{Content: "User lives in Lisbon, Portugal", Category: "fact", Importance: 8},
		[]string{"msg-new"})
	if err != nil {
		t.Fatalf("SaveNoteExplicit() error: %v", err)
	}
	if res == nil || !res.Updated {
		t.Fatalf("SaveNoteExplicit() = %#v, want Updated=true", res)
	}
	notes, err := repo.ListByProfile(ctx, "p1")
	if err != nil || len(notes) != 1 {
		t.Fatalf("ListByProfile() = %d notes, %v; want 1 (merged, not duplicated)", len(notes), err)
	}
	if notes[0].Content != "User lives in Lisbon, Portugal" {
		t.Fatalf("duplicate note content not replaced: %q", notes[0].Content)
	}
	if !strings.Contains(notes[0].SourceMessageIDs, `"msg-old"`) || !strings.Contains(notes[0].SourceMessageIDs, `"msg-new"`) {
		t.Fatalf("source IDs not merged: %s", notes[0].SourceMessageIDs)
	}
	if notes[0].Importance != 8 {
		t.Fatalf("duplicate note importance = %d, want 8", notes[0].Importance)
	}
	if deduper.calls != 1 {
		t.Fatalf("deduper called %d times, want 1", deduper.calls)
	}
}

func TestSaveNoteExplicitTextDedupFallbackWithoutDeduper(t *testing.T) {
	_, repo := openSaveNoteTestDB(t)
	ctx := context.Background()

	seed := &store.MemoryNote{
		ID: "mn-seed", ProfileID: "p1", Category: store.CategoryFact, Importance: 5,
		Content: "User prefers tea over coffee", SourceMessageIDs: `["msg-old"]`,
	}
	if err := repo.Create(ctx, seed); err != nil {
		t.Fatal(err)
	}

	res, err := SaveNoteExplicit(ctx, repo, nil, nil, "p1", "conv-1",
		SaveNoteInput{Content: "User prefers tea over coffee", Category: "fact", Importance: 7},
		[]string{"msg-new"})
	if err != nil {
		t.Fatalf("SaveNoteExplicit() error: %v", err)
	}
	if res == nil || !res.Updated {
		t.Fatalf("SaveNoteExplicit() = %#v, want Updated=true via text dedup fallback", res)
	}
	notes, err := repo.ListByProfile(ctx, "p1")
	if err != nil || len(notes) != 1 {
		t.Fatalf("ListByProfile() = %d notes, %v; want 1", len(notes), err)
	}
}

func TestSaveNoteExplicitTextFallbackRequiresNearIdentical(t *testing.T) {
	_, repo := openSaveNoteTestDB(t)
	ctx := context.Background()

	seed := &store.MemoryNote{
		ID: "mn-seed", ProfileID: "p1", Category: store.CategoryFact, Importance: 5,
		Content: "User's sister is named Anna", SourceMessageIDs: `["msg-old"]`,
	}
	if err := repo.Create(ctx, seed); err != nil {
		t.Fatal(err)
	}

	// Word overlap is ~0.67 (jaccard), above the extractor's 0.45 threshold but
	// far from near-identical: an explicit save must not overwrite a different fact.
	res, err := SaveNoteExplicit(ctx, repo, nil, nil, "p1", "conv-1",
		SaveNoteInput{Content: "User's daughter is named Anna", Category: "fact", Importance: 8},
		[]string{"msg-new"})
	if err != nil {
		t.Fatalf("SaveNoteExplicit() error: %v", err)
	}
	if res.Updated {
		t.Fatalf("SaveNoteExplicit() merged %q into %q; want a separate note", "User's daughter is named Anna", "User's sister is named Anna")
	}
	notes, err := repo.ListByProfile(ctx, "p1")
	if err != nil || len(notes) != 2 {
		t.Fatalf("ListByProfile() = %d notes, %v; want 2 (both facts kept)", len(notes), err)
	}
	contents := make([]string, 0, len(notes))
	for _, n := range notes {
		contents = append(contents, n.Content)
	}
	joined := strings.Join(contents, "\n")
	if !strings.Contains(joined, "sister") || !strings.Contains(joined, "daughter") {
		t.Fatalf("a fact was overwritten: %s", joined)
	}
}

func TestSaveNoteExplicitTextBackstopAfterVectorMiss(t *testing.T) {
	_, repo := openSaveNoteTestDB(t)
	ctx := context.Background()

	seed := &store.MemoryNote{
		ID: "mn-seed", ProfileID: "p1", Category: store.CategoryFact, Importance: 5,
		Content: "Ship date is Nov 5", SourceMessageIDs: `["msg-old"]`,
	}
	if err := repo.Create(ctx, seed); err != nil {
		t.Fatal(err)
	}

	deduper := &saveNoteStubDeduper{result: vector.DedupResult{IsDuplicate: false}}
	res, err := SaveNoteExplicit(ctx, repo, deduper, nil, "p1", "conv-1",
		SaveNoteInput{Content: "Ship date is Nov 5", Category: "fact", Importance: 7},
		[]string{"msg-new"})
	if err != nil {
		t.Fatalf("SaveNoteExplicit() error: %v", err)
	}
	if res == nil || !res.Updated {
		t.Fatalf("SaveNoteExplicit() = %#v, want Updated=true via text backstop after vector miss", res)
	}
	notes, err := repo.ListByProfile(ctx, "p1")
	if err != nil || len(notes) != 1 {
		t.Fatalf("ListByProfile() = %d notes, %v; want 1", len(notes), err)
	}
}

func TestSaveNoteExplicitValidatesInput(t *testing.T) {
	_, repo := openSaveNoteTestDB(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		input   SaveNoteInput
		wantErr bool
	}{
		{name: "empty content", input: SaveNoteInput{Content: "", Category: "fact"}, wantErr: true},
		{name: "whitespace content", input: SaveNoteInput{Content: "   ", Category: "fact"}, wantErr: true},
		{name: "invalid category", input: SaveNoteInput{Content: "Valid content", Category: "bogus"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := SaveNoteExplicit(ctx, repo, nil, nil, "p1", "conv-1", tc.input, nil); (err != nil) != tc.wantErr {
				t.Fatalf("SaveNoteExplicit(%#v) error = %v, wantErr %t", tc.input, err, tc.wantErr)
			}
		})
	}

	t.Run("empty category defaults to fact", func(t *testing.T) {
		res, err := SaveNoteExplicit(ctx, repo, nil, nil, "p2", "conv-1", SaveNoteInput{Content: "User owns a cat"}, nil)
		if err != nil {
			t.Fatalf("SaveNoteExplicit() error: %v", err)
		}
		if res.Note.Category != store.CategoryFact {
			t.Fatalf("category = %q, want fact", res.Note.Category)
		}
	})

	t.Run("importance clamped to 10", func(t *testing.T) {
		res, err := SaveNoteExplicit(ctx, repo, nil, nil, "p2", "conv-1", SaveNoteInput{Content: "User owns a dog", Importance: 99}, nil)
		if err != nil {
			t.Fatalf("SaveNoteExplicit() error: %v", err)
		}
		if res.Note.Importance != 10 {
			t.Fatalf("importance = %d, want 10", res.Note.Importance)
		}
	})
}

func TestSaveNoteExplicitRecordsSourceMessageIDs(t *testing.T) {
	_, repo := openSaveNoteTestDB(t)
	ctx := context.Background()

	res, err := SaveNoteExplicit(ctx, repo, nil, nil, "p1", "conv-1",
		SaveNoteInput{Content: "User's favorite editor is helix", Category: "fact"}, []string{"msg-u1"})
	if err != nil {
		t.Fatalf("SaveNoteExplicit() error: %v", err)
	}
	if got, want := res.Note.SourceMessageIDs, `["msg-u1"]`; got != want {
		t.Fatalf("SourceMessageIDs = %s, want %s", got, want)
	}
}
