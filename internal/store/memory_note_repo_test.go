//go:build !production

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryNoteRepo_ListByProfilePaginated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	db, err := OpenForTesting(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewMemoryNoteRepo(db)
	ctx := context.Background()
	profileID := "profile-1"
	now := time.Now().UTC()

	// Insert 25 notes with staggered timestamps.
	for i := range 25 {
		id := string(rune('a' + i))
		note := &MemoryNote{
			ID:         string(rune('a' + i)),
			ProfileID:  profileID,
			Category:   CategoryFact,
			Content:    "note " + id,
			Importance: 5,
			CreatedAt:  now.Add(-time.Duration(i) * time.Hour),
			UpdatedAt:  now.Add(-time.Duration(i) * time.Hour),
		}
		if err := repo.Create(ctx, note); err != nil {
			t.Fatalf("create note %d: %v", i, err)
		}
	}

	t.Run("first page returns 20 and hasMore=true", func(t *testing.T) {
		notes, hasMore, err := repo.ListByProfilePaginated(ctx, profileID, 20, 0)
		if err != nil {
			t.Fatalf("ListByProfilePaginated: %v", err)
		}
		if len(notes) != 20 {
			t.Fatalf("expected 20 notes, got %d", len(notes))
		}
		if !hasMore {
			t.Fatal("expected hasMore=true")
		}
	})

	t.Run("second page returns 5 and hasMore=false", func(t *testing.T) {
		notes, hasMore, err := repo.ListByProfilePaginated(ctx, profileID, 20, 20)
		if err != nil {
			t.Fatalf("ListByProfilePaginated: %v", err)
		}
		if len(notes) != 5 {
			t.Fatalf("expected 5 notes, got %d", len(notes))
		}
		if hasMore {
			t.Fatal("expected hasMore=false")
		}
	})

	t.Run("empty profile returns empty", func(t *testing.T) {
		notes, hasMore, err := repo.ListByProfilePaginated(ctx, "nonexistent", 20, 0)
		if err != nil {
			t.Fatalf("ListByProfilePaginated: %v", err)
		}
		if len(notes) != 0 {
			t.Fatalf("expected 0 notes, got %d", len(notes))
		}
		if hasMore {
			t.Fatal("expected hasMore=false")
		}
	})
}
