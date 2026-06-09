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

func TestRollupRegeneration_MarksStaleAndRebuildsSummary(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	ctx := context.Background()

	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Rollup Regen")
	noteRepo := store.NewMemoryNoteRepo(db)
	summaryRepo := store.NewMemorySummaryRepo(db)

	mustCreateNoteAt(t, ctx, noteRepo, p.ID, "n1", "Initial weekly content", time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC))
	builder := memory.NewSummaryRollupBuilder(noteRepo, summaryRepo)
	created, err := builder.CatchUp(ctx, p.ID, time.Date(2026, 6, 9, 15, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("initial catch up: %v", err)
	}
	if created.Weekly == 0 {
		t.Fatalf("expected initial weekly summary")
	}

	weekly, err := summaryRepo.ListByProfileAndType(ctx, p.ID, store.SummaryTypeWeekly)
	if err != nil || len(weekly) == 0 {
		t.Fatalf("load weekly summaries: %v len=%d", err, len(weekly))
	}
	if err := noteRepo.Update(ctx, &store.MemoryNote{ID: "n1", ProfileID: p.ID, Category: store.CategoryFact, Content: "Updated weekly content", Importance: 5, SourceMessageIDs: "[]"}); err != nil {
		t.Fatalf("update note: %v", err)
	}
	if err := builder.MarkCoveredSummariesStale(ctx, p.ID, time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("mark stale: %v", err)
	}
	weeklyAfter, err := summaryRepo.GetByPeriod(ctx, p.ID, store.SummaryTypeWeekly, weekly[0].PeriodKey)
	if err != nil {
		t.Fatalf("reload weekly summary: %v", err)
	}
	if weeklyAfter.FreshnessState != store.SummaryStale {
		t.Fatalf("freshness = %q, want stale", weeklyAfter.FreshnessState)
	}

	if err := builder.RegenerateStale(ctx, p.ID, time.Date(2026, 6, 9, 15, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("regenerate stale summaries: %v", err)
	}
	weeklyFresh, err := summaryRepo.GetByPeriod(ctx, p.ID, store.SummaryTypeWeekly, weekly[0].PeriodKey)
	if err != nil {
		t.Fatalf("reload regenerated summary: %v", err)
	}
	if weeklyFresh.FreshnessState != store.SummaryFresh {
		t.Fatalf("freshness = %q, want fresh", weeklyFresh.FreshnessState)
	}
}
