package integration

import (
	"context"
	"testing"
	"time"

	"noto/internal/cache"
	"noto/internal/memory"
	"noto/internal/profile"
	"noto/internal/store"
)

func TestContextCache_HitAfterPut(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()

	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Cache Hit Test")
	cacheRepo := store.NewContextCacheRepo(db)
	svc := cache.NewService(cacheRepo)

	if err := svc.Put(ctx, p.ID, "key1", "payload-data", "[]", "v1", "s1"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	entry, err := svc.Get(ctx, p.ID, "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry == nil {
		t.Fatal("expected cache hit, got nil")
	}
	if entry.Payload != "payload-data" {
		t.Errorf("payload mismatch: got %q", entry.Payload)
	}
}

func TestContextCache_MissOnUnknownKey(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()

	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Cache Miss Test")
	cacheRepo := store.NewContextCacheRepo(db)
	svc := cache.NewService(cacheRepo)

	entry, err := svc.Get(ctx, p.ID, "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry != nil {
		t.Error("expected cache miss (nil), got entry")
	}
}

func TestContextCache_InvalidateRemovesEntry(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()

	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Cache Invalidate Test")
	cacheRepo := store.NewContextCacheRepo(db)
	svc := cache.NewService(cacheRepo)

	if err := svc.Put(ctx, p.ID, "key2", "data", "[]", "v1", "s1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Invalidate(ctx, p.ID, "key2"); err != nil {
		t.Fatal(err)
	}

	entry, _ := svc.Get(ctx, p.ID, "key2")
	if entry != nil {
		t.Error("expected nil after invalidation")
	}
}

func TestContextCache_ExpiredEntry_ReturnsMiss(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()

	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Cache Expiry Test")
	cacheRepo := store.NewContextCacheRepo(db)

	// Insert an already-expired entry directly.
	past := time.Now().Add(-1 * time.Hour)
	entry := &store.ContextCacheEntry{
		ID:            "cc-expired",
		ProfileID:     p.ID,
		CacheKey:      "expired-key",
		Payload:       "old-data",
		SourceNoteIDs: "[]",
		PromptVersion: "v1",
		StateVersion:  "s1",
		CreatedAt:     past,
		ExpiresAt:     &past,
	}
	if err := cacheRepo.Upsert(ctx, entry); err != nil {
		t.Fatal(err)
	}

	svc := cache.NewService(cacheRepo)
	got, _ := svc.Get(ctx, p.ID, "expired-key")
	if got != nil {
		t.Error("expected nil for expired cache entry")
	}
}

func TestContextCache_InvalidateAll_ClearsProfile(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()

	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Cache ClearAll Test")
	cacheRepo := store.NewContextCacheRepo(db)
	svc := cache.NewService(cacheRepo)

	for _, k := range []string{"k1", "k2", "k3"} {
		if err := svc.Put(ctx, p.ID, k, "data", "[]", "v1", "s1"); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.InvalidateAll(ctx, p.ID); err != nil {
		t.Fatal(err)
	}

	for _, k := range []string{"k1", "k2", "k3"} {
		e, _ := svc.Get(ctx, p.ID, k)
		if e != nil {
			t.Errorf("expected nil for key %q after InvalidateAll", k)
		}
	}
}

func TestContextCache_RelevanceSelection_RespectsTokenBudget(t *testing.T) {
	notes := []*store.MemoryNote{
		{ID: "n1", Content: "alpha beta", Importance: 3, CreatedAt: time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)},
		{ID: "n2", Content: "one two three", Importance: 7, CreatedAt: time.Date(2026, 4, 10, 10, 1, 0, 0, time.UTC)},
		{ID: "n3", Content: "four five six", Importance: 5, CreatedAt: time.Date(2026, 4, 10, 10, 2, 0, 0, time.UTC)},
	}
	selected := memory.SelectNotesForContext(notes, []string{"n2", "n1", "n3"}, 5)
	if len(selected) != 2 {
		t.Fatalf("expected 2 notes within budget, got %d", len(selected))
	}
	if selected[0].ID != "n2" || selected[1].ID != "n1" {
		t.Errorf("unexpected order: %s, %s", selected[0].ID, selected[1].ID)
	}
}

func TestContextCache_RelevanceSelection_FallbackOrdering(t *testing.T) {
	notes := []*store.MemoryNote{
		{ID: "n1", Content: "low priority", Importance: 2, CreatedAt: time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)},
		{ID: "n2", Content: "high priority older", Importance: 9, CreatedAt: time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC)},
		{ID: "n3", Content: "high priority newer", Importance: 9, CreatedAt: time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)},
	}
	selected := memory.SelectNotesForContext(notes, nil, 100)
	if len(selected) != 3 {
		t.Fatalf("expected all notes, got %d", len(selected))
	}
	if selected[0].ID != "n3" || selected[1].ID != "n2" || selected[2].ID != "n1" {
		t.Errorf("unexpected fallback order: %s, %s, %s", selected[0].ID, selected[1].ID, selected[2].ID)
	}
}
