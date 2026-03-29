package integration

import (
	"context"
	"errors"
	"testing"

	"noto/internal/profile"
	"noto/internal/store"
)

func TestProfileLifecycle_Create(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()
	svc := profile.NewService(store.NewProfileRepo(db))

	p, err := svc.Create(ctx, "Lifecycle Profile")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.Slug != "lifecycle-profile" {
		t.Errorf("expected slug=lifecycle-profile, got %s", p.Slug)
	}
}

func TestProfileLifecycle_List(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()
	svc := profile.NewService(store.NewProfileRepo(db))

	for _, n := range []string{"P1", "P2", "P3"} {
		svc.Create(ctx, n)
	}
	profiles, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 3 {
		t.Errorf("expected 3, got %d", len(profiles))
	}
}

func TestProfileLifecycle_Select(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()
	svc := profile.NewService(store.NewProfileRepo(db))

	svc.Create(ctx, "Alpha")
	svc.Create(ctx, "Beta")

	p, err := svc.Select(ctx, "Alpha")
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsDefault {
		t.Error("selected profile should be default")
	}

	// Switch to Beta.
	p2, err := svc.Select(ctx, "Beta")
	if err != nil {
		t.Fatal(err)
	}
	if !p2.IsDefault {
		t.Error("Beta should now be default")
	}

	// Alpha should no longer be default.
	active, err := svc.GetActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if active.Name != "Beta" {
		t.Errorf("expected active=Beta, got %s", active.Name)
	}
}

func TestProfileLifecycle_Rename(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()
	svc := profile.NewService(store.NewProfileRepo(db))

	svc.Create(ctx, "Original")
	p, err := svc.Rename(ctx, "Original", "Updated")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Updated" {
		t.Errorf("expected Updated, got %s", p.Name)
	}
	if p.Slug != "updated" {
		t.Errorf("expected slug=updated, got %s", p.Slug)
	}
}

func TestProfileLifecycle_Delete(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()
	svc := profile.NewService(store.NewProfileRepo(db))

	svc.Create(ctx, "Keep")
	svc.Create(ctx, "Delete Me")

	err := svc.Delete(ctx, "Delete Me", func(_ string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}

	profiles, _ := svc.List(ctx)
	for _, p := range profiles {
		if p.Name == "Delete Me" {
			t.Error("deleted profile still present")
		}
	}
}

func TestProfileLifecycle_Delete_LastProfile_Fails(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()
	svc := profile.NewService(store.NewProfileRepo(db))

	svc.Create(ctx, "Only")

	err := svc.Delete(ctx, "Only", func(_ string) bool { return true })
	if !errors.Is(err, profile.ErrProfileInUse) {
		t.Errorf("expected ErrProfileInUse, got %v", err)
	}
}

func TestProfileLifecycle_Delete_ConfirmationDenied_Fails(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()
	svc := profile.NewService(store.NewProfileRepo(db))

	svc.Create(ctx, "Spare")
	svc.Create(ctx, "Target")

	err := svc.Delete(ctx, "Target", func(_ string) bool { return false })
	if !errors.Is(err, profile.ErrConfirmationRequired) {
		t.Errorf("expected ErrConfirmationRequired, got %v", err)
	}
}
