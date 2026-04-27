package integration

import (
	"context"
	"testing"

	chatpkg "noto/internal/chat"
	"noto/internal/observe"
	"noto/internal/profile"
	"noto/internal/store"
	"noto/tests/integration/testutil"
)

func TestConversationRepairGuard_ArchivesLingeringActiveConversationsOnSessionStart(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	ctx := context.Background()

	profSvc := profile.NewService(store.NewProfileRepo(db))
	p, err := profSvc.Create(ctx, "Repair Guard")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	convRepo := store.NewConversationRepo(db)
	msgRepo := store.NewMessageRepo(db)
	noteRepo := store.NewMemoryNoteRepo(db)
	summaryRepo := store.NewSessionSummaryRepo(db)

	// Seed two lingering active conversations for the same profile.
	if err := convRepo.Create(ctx, &store.Conversation{ID: "conv-old-1", ProfileID: p.ID, Status: store.ConversationActive}); err != nil {
		t.Fatalf("seed conv 1: %v", err)
	}
	if err := convRepo.Create(ctx, &store.Conversation{ID: "conv-old-2", ProfileID: p.ID, Status: store.ConversationActive}); err != nil {
		t.Fatalf("seed conv 2: %v", err)
	}

	s, err := chatpkg.NewSession(
		ctx,
		p.ID,
		p.Slug,
		"You are Noto.",
		db,
		convRepo,
		msgRepo,
		noteRepo,
		summaryRepo,
		nil,
		nil,
		observe.NewNoopLogger(),
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close(ctx)

	all, err := convRepo.ListByProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListByProfile: %v", err)
	}

	activeCount := 0
	for _, c := range all {
		if c.Status == store.ConversationActive {
			activeCount++
		}
		if c.ID == "conv-old-1" || c.ID == "conv-old-2" {
			if c.Status != store.ConversationArchived {
				t.Fatalf("expected seeded conversation %s archived, got %s", c.ID, c.Status)
			}
			if c.EndedAt == nil {
				t.Fatalf("expected ended_at set for archived conversation %s", c.ID)
			}
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active conversation after repair guard, got %d", activeCount)
	}
}
