package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"noto/internal/app"
	"noto/internal/profile"
	"noto/internal/store"
)

// tempDB creates a temporary SQLite database and returns its path and a closer.
func tempDB(t *testing.T) (*store.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	return db, func() { db.Close() }
}

func TestStartupFlow_ZeroProfiles_PromptsCreate(t *testing.T) {
	db, close := tempDB(t)
	defer close()

	repo := store.NewProfileRepo(db)
	svc := profile.NewService(repo)
	flow := app.NewStartupFlow(svc)

	ctx := context.Background()
	result, err := flow.Resolve(
		ctx,
		os.Stdout,
		func() (string, error) { return "Test Profile", nil },
		func(_ []*store.Profile) (string, error) { return "", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "prompt_create" {
		t.Errorf("expected action=prompt_create, got %s", result.Action)
	}
	if result.Profile.Name != "Test Profile" {
		t.Errorf("expected name=Test Profile, got %s", result.Profile.Name)
	}
}

func TestStartupFlow_OneProfile_AutoSelect(t *testing.T) {
	db, close := tempDB(t)
	defer close()

	repo := store.NewProfileRepo(db)
	svc := profile.NewService(repo)

	ctx := context.Background()
	_, err := svc.Create(ctx, "Solo Profile")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	flow := app.NewStartupFlow(svc)
	result, err := flow.Resolve(
		ctx, os.Stdout,
		func() (string, error) { return "", nil },
		func(_ []*store.Profile) (string, error) { return "", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "selected_only" {
		t.Errorf("expected action=selected_only, got %s", result.Action)
	}
	if result.Profile.Name != "Solo Profile" {
		t.Errorf("expected name=Solo Profile, got %s", result.Profile.Name)
	}
}

func TestStartupFlow_MultipleProfiles_UsesDefault(t *testing.T) {
	db, close := tempDB(t)
	defer close()

	repo := store.NewProfileRepo(db)
	svc := profile.NewService(repo)

	ctx := context.Background()
	for _, name := range []string{"Alpha", "Beta", "Gamma"} {
		if _, err := svc.Create(ctx, name); err != nil {
			t.Fatalf("create profile %s: %v", name, err)
		}
	}
	if _, err := svc.Select(ctx, "Beta"); err != nil {
		t.Fatalf("select beta: %v", err)
	}

	flow := app.NewStartupFlow(svc)
	result, err := flow.Resolve(
		ctx, os.Stdout,
		func() (string, error) { return "", nil },
		func(_ []*store.Profile) (string, error) { return "", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Profile.Name != "Beta" {
		t.Errorf("expected Beta to be selected, got %s", result.Profile.Name)
	}
}

func TestStartupFlow_MultipleProfiles_NoDefault_PromptsSelect(t *testing.T) {
	db, close := tempDB(t)
	defer close()

	repo := store.NewProfileRepo(db)
	svc := profile.NewService(repo)

	ctx := context.Background()
	for _, name := range []string{"Red", "Blue"} {
		if _, err := svc.Create(ctx, name); err != nil {
			t.Fatalf("create profile: %v", err)
		}
	}
	// Ensure no default by directly checking — after Create, no default is set.

	flow := app.NewStartupFlow(svc)
	result, err := flow.Resolve(
		ctx, os.Stdout,
		func() (string, error) { return "", nil },
		func(_ []*store.Profile) (string, error) { return "Blue", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Profile.Name != "Blue" {
		t.Errorf("expected Blue to be selected, got %s", result.Profile.Name)
	}
}
