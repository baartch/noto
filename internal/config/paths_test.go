package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileFilesDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NOTO_APP_DIR", root)

	dir, err := ProfileFilesDir("artist")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ProfilesDirName, "artist", FilesDirName)
	if dir != want {
		t.Fatalf("ProfileFilesDir() = %q, want %q", dir, want)
	}
	if err := EnsureAppDirs("artist"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("files directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("files path is not a directory: %s", dir)
	}
}
