package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteActiveProfileReplacesMarkerSecurely(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NOTO_APP_DIR", root)
	selectedAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	if err := WriteActiveProfile("alpha", selectedAt); err != nil {
		t.Fatal(err)
	}
	if err := WriteActiveProfile("beta", selectedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	active, err := ReadActiveProfile()
	if err != nil || active.Slug != "beta" || !active.LastSelected.Equal(selectedAt.Add(time.Minute)) {
		t.Fatalf("active profile = %+v, err=%v", active, err)
	}
	info, err := os.Stat(filepath.Join(root, ActiveProfileFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("active profile permissions = %o, want 600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ActiveProfileFile {
		t.Fatalf("active profile temp files were not cleaned up: %v", entries)
	}
}
