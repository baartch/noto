package integration

import (
	"context"
	"testing"

	"noto/internal/memory"
	"noto/internal/profile"
	"noto/internal/store"
)

func TestMemoryContinuity_ExtractTurn_NoLLM_SavesNothing(t *testing.T) {
	db, close := tempDB(t)
	defer close()
	ctx := context.Background()

	noteRepo := store.NewMemoryNoteRepo(db)
	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Memory Test")

	// With nil adapter, ExtractTurn should return no notes (no LLM = no extraction).
	extractor := memory.NewExtractor(noteRepo, nil, nil)
	result, err := extractor.ExtractTurn(ctx, p.ID, "", "hello", "hi there")
	if err != nil {
		t.Fatalf("ExtractTurn: %v", err)
	}
	if len(result.Notes) != 0 {
		t.Errorf("expected 0 notes with nil adapter, got %d", len(result.Notes))
	}
}

func TestMemoryContinuity_ManualNote_AppearsInRetrieval(t *testing.T) {
	db, close := tempDB(t)
	defer close()
	ctx := context.Background()

	noteRepo := store.NewMemoryNoteRepo(db)
	summaryRepo := store.NewSessionSummaryRepo(db)
	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Retrieval Test")

	// Insert a note directly (simulating a previous session's extraction).
	note := &store.MemoryNote{
		ID:               "mn-manual",
		ProfileID:        p.ID,
		Category:         store.CategoryFact,
		Content:          "User prefers concise answers",
		Importance:       8,
		SourceMessageIDs: "[]",
	}
	if err := noteRepo.Create(ctx, note); err != nil {
		t.Fatal(err)
	}

	// Retrieval should include the note in the assembled prompt.
	retrieval := memory.NewRetrieval(noteRepo, summaryRepo, nil)
	rc, err := retrieval.Assemble(ctx, p.ID, "You are a helpful assistant.")
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if rc.MemoryBlock == "" {
		t.Error("expected non-empty memory block")
	}
	if rc.AssembledPrompt == rc.SystemPrompt {
		t.Error("assembled prompt should differ from base system prompt when notes exist")
	}
}

func TestMemoryContinuity_PersistAcrossSessions(t *testing.T) {
	db, close := tempDB(t)
	defer close()
	ctx := context.Background()

	noteRepo := store.NewMemoryNoteRepo(db)
	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Continuity Test")

	note := &store.MemoryNote{
		ID:               "mn-prev",
		ProfileID:        p.ID,
		Category:         store.CategoryFact,
		Content:          "User prefers concise answers",
		Importance:       8,
		SourceMessageIDs: "[]",
	}
	if err := noteRepo.Create(ctx, note); err != nil {
		t.Fatal(err)
	}

	// New session retrieval should find the note.
	notes, err := noteRepo.ListByProfile(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range notes {
		if n.ID == "mn-prev" {
			found = true
		}
	}
	if !found {
		t.Error("memory note from previous session not found")
	}
}
