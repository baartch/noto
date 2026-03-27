package integration

import (
	"context"
	"testing"

	"noto/internal/memory"
	"noto/internal/profile"
	"noto/internal/store"
)

func TestMemoryContinuity_ExtractAndRetrieve(t *testing.T) {
	db, close := tempDB(t)
	defer close()

	ctx := context.Background()
	noteRepo := store.NewMemoryNoteRepo(db)
	summaryRepo := store.NewSessionSummaryRepo(db)

	// Create a profile.
	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Memory Test")

	// Create a conversation.
	convRepo := store.NewConversationRepo(db)
	conv := &store.Conversation{
		ID:        "conv-1",
		ProfileID: p.ID,
		Status:    store.ConversationActive,
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}

	// Simulate a long assistant message.
	msgRepo := store.NewMessageRepo(db)
	longContent := "This is a detailed assistant response that provides comprehensive information about the topic at hand, exceeding the threshold for memory extraction so it gets captured as a fact."
	msg := &store.Message{
		ID:             "msg-1",
		ConversationID: "conv-1",
		Role:           store.RoleAssistant,
		Content:        longContent,
	}
	if err := msgRepo.Create(ctx, msg); err != nil {
		t.Fatal(err)
	}

	// Extract memory.
	extractor := memory.NewExtractor(noteRepo)
	result, err := extractor.Extract(ctx, p.ID, "conv-1", []*store.Message{msg})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(result.Notes) == 0 {
		t.Error("expected at least one memory note to be extracted")
	}

	// Retrieve context.
	retrieval := memory.NewRetrieval(noteRepo, summaryRepo)
	rc, err := retrieval.Assemble(ctx, p.ID, "You are a helpful assistant.")
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if rc.MemoryBlock == "" {
		t.Error("expected non-empty memory block after extraction")
	}
}

func TestMemoryContinuity_PersistAcrossSessions(t *testing.T) {
	db, close := tempDB(t)
	defer close()

	ctx := context.Background()
	noteRepo := store.NewMemoryNoteRepo(db)

	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Continuity Test")

	// Insert a note directly simulating a previous session.
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
