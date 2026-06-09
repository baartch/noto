package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"noto/internal/memory"
	"noto/internal/profile"
	"noto/internal/store"
	"noto/tests/integration/testutil"
)

func TestTimelineSettingsEdges_BoundedMonthlyAndZeroValidation(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	ctx := context.Background()

	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Edges")
	settingsRepo := store.NewTimelineSettingsRepo(db)

	if err := settingsRepo.Upsert(ctx, &store.TimelineSettings{ProfileID: p.ID, RawNoteDays: 30, WeeklySummaryWeeks: 0, MonthlySummaryMonths: store.MonthlySummaryAllRemaining}); err == nil {
		t.Fatalf("expected zero weekly setting to be rejected")
	}
	if err := settingsRepo.Upsert(ctx, &store.TimelineSettings{ProfileID: p.ID, RawNoteDays: 30, WeeklySummaryWeeks: 8, MonthlySummaryMonths: 0}); err == nil {
		t.Fatalf("expected zero monthly setting to be rejected")
	}

	valid := &store.TimelineSettings{ProfileID: p.ID, RawNoteDays: 30, WeeklySummaryWeeks: 8, MonthlySummaryMonths: 2}
	if err := settingsRepo.Upsert(ctx, valid); err != nil {
		t.Fatalf("upsert valid settings: %v", err)
	}
	got, err := settingsRepo.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got.MonthlySummaryMonths != 2 {
		t.Fatalf("monthly setting = %d, want 2", got.MonthlySummaryMonths)
	}

	window, err := memory.ComputeTimelineWindow(time.Date(2026, 6, 9, 15, 0, 0, 0, time.UTC), valid)
	if err != nil {
		t.Fatalf("compute window: %v", err)
	}
	if window.MonthlyCutoff == nil {
		t.Fatalf("expected bounded monthly cutoff")
	}
}

func TestTimelineSettingsRepo_NotFoundStillSupported(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	ctx := context.Background()
	repo := store.NewTimelineSettingsRepo(db)
	_, err := repo.Get(ctx, "missing")
	if !errors.Is(err, store.ErrTimelineSettingsNotFound) {
		t.Fatalf("expected ErrTimelineSettingsNotFound, got %v", err)
	}
}
