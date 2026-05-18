package cache_test

import (
	"context"
	"testing"

	"noto/internal/cache"
	"noto/internal/store"
	"noto/tests/integration/testutil"
)

func TestInvalidationTriggers_AdditionalEvents(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	repo := store.NewContextCacheRepo(db)
	tr := cache.NewInvalidationTriggers(repo)
	ctx := context.Background()
	if err := tr.OnTokenBudgetChange(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if err := tr.OnEmbeddingModelChange(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if err := tr.OnNoteCreated(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if err := tr.OnNoteUpdated(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if err := tr.OnNoteDeleted(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
}
