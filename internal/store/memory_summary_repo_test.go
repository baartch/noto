//go:build !production

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestMemorySummaryRepo_ListByProfileAndTypePaginated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	db, err := OpenForTesting(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewMemorySummaryRepo(db)
	ctx := context.Background()
	profileID := "profile-1"
	now := time.Now().UTC()

	// Insert 25 weekly summaries.
	for i := range 25 {
		s := MemorySummary{
			ID:             string(rune('w' + i)),
			ProfileID:      profileID,
			SummaryType:    SummaryTypeWeekly,
			PeriodKey:      "2026-W" + string(rune('0'+i)),
			PeriodStart:    now.Add(-time.Duration(i) * 7 * 24 * time.Hour),
			PeriodEnd:      now.Add(-time.Duration(i) * 7 * 24 * time.Hour).Add(6 * 24 * time.Hour),
			Content:        "weekly summary " + string(rune('a'+i)),
			FreshnessState: SummaryFresh,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := repo.Upsert(ctx, &s); err != nil {
			t.Fatalf("upsert summary %d: %v", i, err)
		}
	}

	t.Run("first page returns 20 and hasMore=true", func(t *testing.T) {
		summaries, hasMore, err := repo.ListByProfileAndTypePaginated(ctx, profileID, SummaryTypeWeekly, 20, 0)
		if err != nil {
			t.Fatalf("ListByProfileAndTypePaginated: %v", err)
		}
		if len(summaries) != 20 {
			t.Fatalf("expected 20 summaries, got %d", len(summaries))
		}
		if !hasMore {
			t.Fatal("expected hasMore=true")
		}
	})

	t.Run("second page returns 5 and hasMore=false", func(t *testing.T) {
		summaries, hasMore, err := repo.ListByProfileAndTypePaginated(ctx, profileID, SummaryTypeWeekly, 20, 20)
		if err != nil {
			t.Fatalf("ListByProfileAndTypePaginated: %v", err)
		}
		if len(summaries) != 5 {
			t.Fatalf("expected 5 summaries, got %d", len(summaries))
		}
		if hasMore {
			t.Fatal("expected hasMore=false")
		}
	})

	t.Run("empty type returns empty", func(t *testing.T) {
		summaries, hasMore, err := repo.ListByProfileAndTypePaginated(ctx, profileID, SummaryTypeMonthly, 20, 0)
		if err != nil {
			t.Fatalf("ListByProfileAndTypePaginated: %v", err)
		}
		if len(summaries) != 0 {
			t.Fatalf("expected 0 summaries, got %d", len(summaries))
		}
		if hasMore {
			t.Fatal("expected hasMore=false")
		}
	})
}
