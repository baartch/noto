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

func TestRollupRuntime_MarkStaleAfterCoveredNoteChange(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	ctx := context.Background()

	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Rollup Runtime")
	noteRepo := store.NewMemoryNoteRepo(db)
	summaryRepo := store.NewMemorySummaryRepo(db)

	noteTime := time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC)
	mustCreateNoteAt(t, ctx, noteRepo, p.ID, "n1", "Initial covered note", noteTime)
	builder := memory.NewSummaryRollupBuilder(noteRepo, summaryRepo)
	if _, err := builder.CatchUp(ctx, p.ID, time.Date(2026, 6, 9, 15, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("catch up: %v", err)
	}
	if err := builder.MarkCoveredSummariesStale(ctx, p.ID, noteTime); err != nil {
		t.Fatalf("mark stale: %v", err)
	}
	weekly, err := summaryRepo.GetByPeriod(ctx, p.ID, store.SummaryTypeWeekly, memory.WeeklyPeriodKey(noteTime))
	if err != nil {
		t.Fatalf("get weekly summary: %v", err)
	}
	if weekly.FreshnessState != store.SummaryStale {
		t.Fatalf("weekly freshness = %q, want stale", weekly.FreshnessState)
	}
}
