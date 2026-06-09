package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"noto/internal/memory"
	"noto/internal/profile"
	"noto/internal/store"
	"noto/tests/integration/testutil"
)

func TestTimelineContextAssembly_MultiMonthHistory(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	ctx := context.Background()

	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Timeline")
	noteRepo := store.NewMemoryNoteRepo(db)
	summaryRepo := store.NewMemorySummaryRepo(db)
	settingsRepo := store.NewTimelineSettingsRepo(db)

	now := time.Date(2026, 6, 9, 15, 0, 0, 0, time.UTC)
	mustTimelineSettings(t, ctx, settingsRepo, &store.TimelineSettings{ProfileID: p.ID, RawNoteDays: 30, WeeklySummaryWeeks: 8, MonthlySummaryMonths: store.MonthlySummaryAllRemaining})

	mustCreateNoteAt(t, ctx, noteRepo, p.ID, "recent-1", "Recent note A", time.Date(2026, 6, 8, 9, 0, 0, 0, time.UTC))
	mustCreateNoteAt(t, ctx, noteRepo, p.ID, "recent-2", "Recent note B", time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC))
	mustCreateSummary(t, ctx, summaryRepo, &store.MemorySummary{ID: "w-2026-W18", ProfileID: p.ID, SummaryType: store.SummaryTypeWeekly, PeriodKey: "2026-W18", PeriodStart: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC), Content: "Weekly summary in range", SourceStateVersion: "v1", FreshnessState: store.SummaryFresh, CreatedAt: now, UpdatedAt: now})
	mustCreateSummary(t, ctx, summaryRepo, &store.MemorySummary{ID: "m-2026-03", ProfileID: p.ID, SummaryType: store.SummaryTypeMonthly, PeriodKey: "2026-03", PeriodStart: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Content: "Monthly summary in range", SourceStateVersion: "v1", FreshnessState: store.SummaryFresh, CreatedAt: now, UpdatedAt: now})

	window, err := memory.ComputeTimelineWindow(now, &store.TimelineSettings{ProfileID: p.ID, RawNoteDays: 30, WeeklySummaryWeeks: 8, MonthlySummaryMonths: store.MonthlySummaryAllRemaining})
	if err != nil {
		t.Fatalf("compute timeline window: %v", err)
	}
	if !window.RawStart.Equal(time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("raw start = %v, want 2026-05-04", window.RawStart)
	}

	block := memory.BuildTimelineMemoryBlock(
		[]*store.MemoryNote{{ID: "recent-1", Category: store.CategoryFact, Content: "Recent note A"}, {ID: "recent-2", Category: store.CategoryFact, Content: "Recent note B"}},
		[]*store.MemorySummary{{ID: "w-2026-W18", SummaryType: store.SummaryTypeWeekly, PeriodKey: "2026-W18", Content: "Weekly summary in range"}},
		[]*store.MemorySummary{{ID: "m-2026-03", SummaryType: store.SummaryTypeMonthly, PeriodKey: "2026-03", Content: "Monthly summary in range"}},
	)

	if !strings.Contains(block, "Recent note A") || !strings.Contains(block, "Weekly summary in range") || !strings.Contains(block, "Monthly summary in range") {
		t.Fatalf("expected all timeline layers in block, got %q", block)
	}
}

func mustTimelineSettings(t *testing.T, ctx context.Context, repo *store.TimelineSettingsRepo, s *store.TimelineSettings) {
	t.Helper()
	if err := repo.Upsert(ctx, s); err != nil {
		t.Fatalf("upsert settings: %v", err)
	}
}

func mustCreateSummary(t *testing.T, ctx context.Context, repo *store.MemorySummaryRepo, s *store.MemorySummary) {
	t.Helper()
	if err := repo.Upsert(ctx, s); err != nil {
		t.Fatalf("upsert summary: %v", err)
	}
}

func mustCreateNoteAt(t *testing.T, ctx context.Context, repo *store.MemoryNoteRepo, profileID, id, content string, createdAt time.Time) {
	t.Helper()
	if err := repo.Create(ctx, &store.MemoryNote{ID: id, ProfileID: profileID, Category: store.CategoryFact, Content: content, Importance: 5, SourceMessageIDs: "[]"}); err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, err := repo.DB().ExecContext(ctx, `UPDATE memory_notes SET created_at = ?, updated_at = ? WHERE id = ?`, createdAt, createdAt, id); err != nil {
		t.Fatalf("set note timestamps: %v", err)
	}
}
