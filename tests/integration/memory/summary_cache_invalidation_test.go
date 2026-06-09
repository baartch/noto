package integration

import (
	"context"
	"testing"

	"noto/internal/cache"
	"noto/internal/profile"
	"noto/internal/store"
	"noto/tests/integration/testutil"
)

func TestSummaryCacheInvalidation_OnSummaryStateChange(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	ctx := context.Background()
	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Summary Cache")

	cacheRepo := store.NewContextCacheRepo(db)
	service := cache.NewService(cacheRepo)
	if err := service.Put(ctx, p.ID, "cache-key", "payload", "[]", "prompt:v1", "state:v1"); err != nil {
		t.Fatalf("put cache: %v", err)
	}
	triggers := cache.NewInvalidationTriggers(cacheRepo)
	if err := triggers.OnSummaryChange(ctx, p.ID); err != nil {
		t.Fatalf("invalidate on summary change: %v", err)
	}
	entry, err := service.Get(ctx, p.ID, "cache-key")
	if err != nil {
		t.Fatalf("get cache: %v", err)
	}
	if entry != nil {
		t.Fatalf("expected cache entry to be invalidated")
	}
}
