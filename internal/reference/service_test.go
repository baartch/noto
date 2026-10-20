package reference

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenValidatedRootRejectsSymlinkSwap(t *testing.T) {
	parent := t.TempDir()
	rootPath := filepath.Join(parent, "files")
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	original := filepath.Join(parent, "files-original")
	inspect := func(name string) (os.FileInfo, error) {
		info, err := os.Lstat(name)
		if err != nil {
			return nil, err
		}
		if err := os.Rename(name, original); err != nil {
			return nil, err
		}
		if err := os.Symlink(outside, name); err != nil {
			return nil, err
		}
		return info, nil
	}
	root, err := openValidatedRoot(rootPath, inspect, os.OpenRoot)
	if err == nil {
		_ = root.Close()
		t.Fatal("accepted a root swapped for a symlink after validation")
	}
}
