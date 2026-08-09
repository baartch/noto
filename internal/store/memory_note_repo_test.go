//go:build !production

package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryNoteRepo_CreateBatchOrdering(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	db, err := OpenForTesting(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewMemoryNoteRepo(db)
	ctx := context.Background()
	profileID := "profile-batch"

	// Simulate a single extraction batch: notes inserted in a tight loop with
	// zero timestamps (the pre-fix behavior used the second-precision DB default,
	// so every note in the batch shared the same created_at).
	for i := range 6 {
		note := &MemoryNote{
			ID:         fmt.Sprintf("mn-batch-%d", i),
			ProfileID:  profileID,
			Category:   CategoryFact,
			Content:    fmt.Sprintf("note %d", i),
			Importance: 5,
		}
		if err := repo.Create(ctx, note); err != nil {
			t.Fatalf("create note %d: %v", i, err)
		}
		// Each note must get a populated, non-zero timestamp.
		if note.CreatedAt.IsZero() || note.UpdatedAt.IsZero() {
			t.Fatalf("note %d: expected populated CreatedAt/UpdatedAt", i)
		}
	}

	notes, hasMore, err := repo.ListByProfilePaginated(ctx, profileID, 20, 0)
	if err != nil {
		t.Fatalf("ListByProfilePaginated: %v", err)
	}
	if hasMore {
		t.Fatal("expected hasMore=false")
	}
	if len(notes) != 6 {
		t.Fatalf("expected 6 notes, got %d", len(notes))
	}
	// Newest-created note must come first (created_at DESC).
	if notes[0].ID != "mn-batch-5" {
		t.Fatalf("expected mn-batch-5 first, got %s", notes[0].ID)
	}
	for i := 1; i < len(notes); i++ {
		if notes[i].ID != fmt.Sprintf("mn-batch-%d", len(notes)-1-i) {
			t.Fatalf("expected descending batch order, got %s at position %d", notes[i].ID, i)
		}
		if !notes[i-1].CreatedAt.After(notes[i].CreatedAt) {
			t.Fatalf("expected strictly decreasing CreatedAt, %v not after %v", notes[i-1].CreatedAt, notes[i].CreatedAt)
		}
	}
}

func TestMemoryNoteRepo_LegacyTiebreakNewestFirst(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	db, err := OpenForTesting(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewMemoryNoteRepo(db)
	ctx := context.Background()
	profileID := "profile-legacy"

	// Legacy rows all share an identical created_at (second-precision default).
	ts := time.Date(2026, 8, 9, 8, 54, 3, 0, time.UTC)
	for i := range 6 {
		note := &MemoryNote{
			ID:         fmt.Sprintf("mn-legacy-%d", i),
			ProfileID:  profileID,
			Category:   CategoryFact,
			Content:    fmt.Sprintf("note %d", i),
			Importance: 5,
			CreatedAt:  ts,
			UpdatedAt:  ts,
		}
		if err := repo.Create(ctx, note); err != nil {
			t.Fatalf("create note %d: %v", i, err)
		}
		if !note.CreatedAt.Equal(ts) {
			t.Fatalf("expected explicit CreatedAt preserved, got %v", note.CreatedAt)
		}
	}

	notes, _, err := repo.ListByProfilePaginated(ctx, profileID, 20, 0)
	if err != nil {
		t.Fatalf("ListByProfilePaginated: %v", err)
	}
	if len(notes) != 6 {
		t.Fatalf("expected 6 notes, got %d", len(notes))
	}
	// Ties must break on rowid DESC: most recently inserted note first.
	if notes[0].ID != "mn-legacy-5" {
		t.Fatalf("expected mn-legacy-5 first, got %s", notes[0].ID)
	}
	for i := 1; i < len(notes); i++ {
		if notes[i].ID != fmt.Sprintf("mn-legacy-%d", len(notes)-1-i) {
			t.Fatalf("expected descending legacy order, got %s at position %d", notes[i].ID, i)
		}
	}
}

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
