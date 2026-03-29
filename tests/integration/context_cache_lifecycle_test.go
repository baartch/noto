package integration

import (
	"context"
	"testing"
	"time"

	"noto/internal/cache"
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
