package contract

import (
	"context"
	"path/filepath"
	"testing"

	"noto/internal/profile"
	"noto/internal/store"
)

// openTestDB creates an in-memory (temp file) test database.
func openTestDB(t *testing.T) (*store.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("NOTO_APP_DIR", dir)
	db, err := store.OpenForTesting(filepath.Join(dir, "contract_test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db, func() { db.Close() }
}

// TestProfileCreate_Contract verifies that Create returns a profile with correct fields.
func TestProfileCreate_Contract(t *testing.T) {
	db, closeDB := openTestDB(t)
	defer closeDB()

	svc := profile.NewService(store.NewProfileRepo(db))
	ctx := context.Background()

	p, err := svc.Create(ctx, "Contract Profile")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.Name != "Contract Profile" {
		t.Errorf("Name mismatch: got %q", p.Name)
	}
	if p.Slug == "" {
		t.Error("Slug must not be empty")
	}
	if p.ID == "" {
		t.Error("ID must not be empty")
	}
}

// TestProfileList_Contract verifies that List returns all created profiles.
func TestProfileList_Contract(t *testing.T) {
	db, closeDB := openTestDB(t)
	defer closeDB()

	svc := profile.NewService(store.NewProfileRepo(db))
	ctx := context.Background()

	for _, n := range []string{"A", "B", "C"} {
		if _, err := svc.Create(ctx, n); err != nil {
			t.Fatalf("create %s: %v", n, err)
		}
	}

	profiles, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(profiles) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(profiles))
	}
}

// TestProfileSelect_Contract verifies that Select marks the profile as default.
func TestProfileSelect_Contract(t *testing.T) {
	db, closeDB := openTestDB(t)
	defer closeDB()

	svc := profile.NewService(store.NewProfileRepo(db))
	ctx := context.Background()

	if _, err := svc.Create(ctx, "Primary"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "Secondary"); err != nil {
		t.Fatal(err)
	}

	p, err := svc.Select(ctx, "Secondary")
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if !p.IsDefault {
		t.Error("selected profile should be marked as default")
	}
}

// TestProfileRename_Contract verifies that Rename updates name and slug.
func TestProfileRename_Contract(t *testing.T) {
	db, closeDB := openTestDB(t)
	defer closeDB()

	svc := profile.NewService(store.NewProfileRepo(db))
	ctx := context.Background()

	if _, err := svc.Create(ctx, "Old Name"); err != nil {
		t.Fatal(err)
	}

	p, err := svc.Rename(ctx, "Old Name", "New Name")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if p.Name != "New Name" {
		t.Errorf("expected name=New Name, got %q", p.Name)
	}
	if p.Slug != "new-name" {
		t.Errorf("expected slug=new-name, got %q", p.Slug)
	}
}

// TestProfileDelete_Contract verifies deletion with confirmation.
func TestProfileDelete_Contract(t *testing.T) {
	db, closeDB := openTestDB(t)
	defer closeDB()

	svc := profile.NewService(store.NewProfileRepo(db))
	ctx := context.Background()

	if _, err := svc.Create(ctx, "ToDelete"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "Keeper"); err != nil {
		t.Fatal(err)
	}

	err := svc.Delete(ctx, "ToDelete", func(_ string) bool { return true })
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	profiles, _ := svc.List(ctx)
	for _, p := range profiles {
		if p.Name == "ToDelete" {
			t.Error("deleted profile still present")
		}
	}
}

// TestProfileDelete_OnlyProfile_Contract verifies that deleting the last profile fails.
func TestProfileDelete_OnlyProfile_Contract(t *testing.T) {
	db, closeDB := openTestDB(t)
	defer closeDB()

	svc := profile.NewService(store.NewProfileRepo(db))
	ctx := context.Background()

	if _, err := svc.Create(ctx, "Only"); err != nil {
		t.Fatal(err)
	}

	err := svc.Delete(ctx, "Only", func(_ string) bool { return true })
	if err == nil {
		t.Error("expected error when deleting only profile, got nil")
	}
}
