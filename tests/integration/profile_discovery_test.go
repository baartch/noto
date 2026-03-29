package integration

import (
	"context"
	"testing"

	"noto/internal/profile"
	"noto/internal/store"
)

func TestProfileDiscovery_ListViaFilesystem(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()

	ctx := context.Background()
	svc := profile.NewService(store.NewProfileRepo(db))

	if _, err := svc.Create(ctx, "Alpha"); err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	if _, err := svc.Create(ctx, "Beta"); err != nil {
		t.Fatalf("create beta: %v", err)
	}

	profiles, warnings, err := profile.DiscoverProfiles()
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
}
