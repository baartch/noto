package integration

import (
	"context"
	"testing"
	"time"

	"noto/internal/memory"
	"noto/internal/profile"
	"noto/internal/store"
	"noto/tests/integration/testutil"
)

func TestRollupCatchup_CreatesMissingPeriodSummariesOnNextProcessing(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	ctx := context.Background()

	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Rollup Catchup")
	noteRepo := store.NewMemoryNoteRepo(db)
	summaryRepo := store.NewMemorySummaryRepo(db)

	mustCreateNoteAt(t, ctx, noteRepo, p.ID, "week-note", "Work completed in May week", time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC))
	mustCreateNoteAt(t, ctx, noteRepo, p.ID, "month-note", "Milestone achieved in April", time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC))

	builder := memory.NewSummaryRollupBuilder(noteRepo, summaryRepo)
	created, err := builder.CatchUp(ctx, p.ID, time.Date(2026, 6, 9, 15, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("catch up rollups: %v", err)
	}
	if created.Weekly == 0 {
		t.Fatalf("expected weekly catch-up summary creation")
	}
	if created.Monthly == 0 {
		t.Fatalf("expected monthly catch-up summary creation")
	}
}
