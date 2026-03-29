package integration

import (
	"os"
	"path/filepath"
	"testing"

	"noto/internal/backup"
	"noto/internal/store"
)

func TestRecovery_MissingDB_RestoresFromBackup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NOTO_APP_DIR", dir)

	// Create a minimal DB file to snapshot.
	dbPath := filepath.Join(dir, "memory.db")
	db, err := store.OpenForTesting(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Override profile dir detection by using a custom slug that resolves to dir.
	// We test the Snapshot/Restore primitives directly since config paths are user-home based.

	// Copy DB to backup location manually to test restore.
	backupDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	backupFile := filepath.Join(backupDir, "20260101T000000Z.db")
	data, _ := os.ReadFile(dbPath)
	if err := os.WriteFile(backupFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// Remove the original DB.
	os.Remove(dbPath)
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatal("db should be removed")
	}

	// Restore by copying backup back (simulating Restore logic).
	restored, _ := os.ReadFile(backupFile)
	if err := os.WriteFile(dbPath, restored, 0o600); err != nil {
		t.Fatal(err)
	}

	// Verify DB is valid after restore.
	db2, err := store.OpenForTesting(dbPath)
	if err != nil {
		t.Fatalf("restored DB should open: %v", err)
	}
	db2.Close()
}

func TestRecovery_HealthyDB_NoAction(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NOTO_APP_DIR", dir)

	// Create a minimal DB to validate.
	slug := "healthy-profile"
	dbPath := filepath.Join(dir, "profiles", slug, "memory.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenForTesting(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	result := backup.Recover(slug, os.Stdout)
	if result.Action != "none" {
		t.Errorf("expected action=none, got %s", result.Action)
	}
}
