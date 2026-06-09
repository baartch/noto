package memory_test

import (
	"context"
	"path/filepath"
	"testing"

	"noto/internal/store"
)

func TestTimelineSettingsRepo_UpsertAndGet(t *testing.T) {
	db := openTimelineTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repo := store.NewTimelineSettingsRepo(db)
	in := &store.TimelineSettings{ProfileID: "profile-1", RawNoteDays: 45, WeeklySummaryWeeks: 12, MonthlySummaryMonths: 6}
	if err := repo.Upsert(context.Background(), in); err != nil {
		t.Fatalf("upsert settings: %v", err)
	}
	got, err := repo.Get(context.Background(), "profile-1")
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got.RawNoteDays != 45 || got.WeeklySummaryWeeks != 12 || got.MonthlySummaryMonths != 6 {
		t.Fatalf("unexpected settings: %#v", got)
	}
}

func TestTimelineSettingsRepo_GetOrDefault(t *testing.T) {
	db := openTimelineTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repo := store.NewTimelineSettingsRepo(db)
	got, err := repo.GetOrDefault(context.Background(), "profile-2")
	if err != nil {
		t.Fatalf("get default settings: %v", err)
	}
	if got.RawNoteDays != 30 || got.WeeklySummaryWeeks != 8 || got.MonthlySummaryMonths != store.MonthlySummaryAllRemaining {
		t.Fatalf("unexpected defaults: %#v", got)
	}
}

func TestTimelineSettings_ValidateRejectsZeroWeeklyOrMonthly(t *testing.T) {
	cases := []*store.TimelineSettings{
		{ProfileID: "p", RawNoteDays: 30, WeeklySummaryWeeks: 0, MonthlySummaryMonths: 1},
		{ProfileID: "p", RawNoteDays: 30, WeeklySummaryWeeks: 8, MonthlySummaryMonths: 0},
	}
	for _, tc := range cases {
		if err := tc.Validate(); err == nil {
			t.Fatalf("expected validation error for %#v", tc)
		}
	}
}

func TestTimelineSettings_ValidateAcceptsAllRemainingSentinel(t *testing.T) {
	s := &store.TimelineSettings{ProfileID: "p", RawNoteDays: 30, WeeklySummaryWeeks: 8, MonthlySummaryMonths: store.MonthlySummaryAllRemaining}
	if err := s.Validate(); err != nil {
		t.Fatalf("expected valid sentinel config, got %v", err)
	}
}

func openTimelineTestDB(t *testing.T) *store.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "timeline.db")
	db, err := store.OpenForTesting(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	return db
}
