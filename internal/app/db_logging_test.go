package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileDatabaseLogsAppliedMigrationsOnce(t *testing.T) {
	appDir := t.TempDir()
	t.Setenv("NOTO_APP_DIR", appDir)

	for range 2 {
		db, err := openProfileDB("logging-test")
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}

	log, err := os.ReadFile(filepath.Join(appDir, "noto.log"))
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(log), "migration applied: version=0005_restore_context_cache"); count != 1 {
		t.Fatalf("expected one successful cache migration log, got %d: %s", count, log)
	}
	if !strings.Contains(string(log), "logging-test") {
		t.Fatalf("migration log must identify the profile database: %s", log)
	}
}

func TestProfileDatabaseDoesNotLogFailedMigrationAsSuccessful(t *testing.T) {
	appDir := t.TempDir()
	t.Setenv("NOTO_APP_DIR", appDir)
	dir := filepath.Join(appDir, "profiles", "logging-test")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE schema_migrations (version TEXT PRIMARY KEY);
		INSERT INTO schema_migrations (version) VALUES
			('0001_init'), ('0002_extractor_model'), ('0002_system_prompts'),
			('0003_memory_timeline'), ('0004_reference_files');
		CREATE VIEW context_cache AS SELECT 1 AS profile_id;
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	opened, err := openProfileDB("logging-test")
	if err == nil {
		_ = opened.Close()
		t.Fatal("expected cache migration to fail")
	}
	log, err := os.ReadFile(filepath.Join(appDir, "noto.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(log), "migration applied: version=0005_restore_context_cache") {
		t.Fatalf("failed migration logged as successful: %s", log)
	}
}
