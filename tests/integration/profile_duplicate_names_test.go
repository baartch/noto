package integration

import (
	"context"
	"testing"

	"noto/internal/profile"
	"noto/internal/store"
)

func TestProfileDiscovery_DuplicateNames(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()

	ctx := context.Background()
	svc := profile.NewService(store.NewProfileRepo(db))

	p1, err := svc.Create(ctx, "Same Name")
	if err != nil {
		t.Fatalf("create p1: %v", err)
	}
	p2, err := svc.Create(ctx, "Same Name")
	if err != nil {
		t.Fatalf("create p2: %v", err)
	}

	if p1.Slug == p2.Slug {
		t.Fatalf("expected different slugs for duplicate names")
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
