package reference

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"noto/internal/startup"
	"noto/internal/store"
)

func TestScanReportsProgressIncludingFailedFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("private text"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := NewService(Options{Root: root, ProfileID: "p", Repo: store.NewReferenceFileRepo(db)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var events []startup.Event
	diagnostics, err := s.Scan(context.Background(), func(e startup.Event) { events = append(events, e) })
	if err != nil || len(diagnostics) != 1 {
		t.Fatalf("Scan = %v, %v; want one warning", diagnostics, err)
	}
	if len(events) < 3 || events[0].State != "start" || events[0].Total != -1 || events[len(events)-1] != (startup.Event{Phase: "files", State: "warning", Done: 1, Total: 1}) {
		t.Fatalf("progress events = %#v", events)
	}
}

func TestScanReportsCancellation(t *testing.T) {
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := NewService(Options{Root: t.TempDir(), ProfileID: "p", Repo: store.NewReferenceFileRepo(db)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var events []startup.Event
	_, err = s.Scan(ctx, func(e startup.Event) { events = append(events, e) })
	if !errors.Is(err, context.Canceled) || len(events) < 2 || events[len(events)-1].State != "failure" {
		t.Fatalf("Scan = %v, events = %#v", err, events)
	}
}

func TestScanReportsEmptyDirectory(t *testing.T) {
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := NewService(Options{Root: t.TempDir(), ProfileID: "p", Repo: store.NewReferenceFileRepo(db)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var events []startup.Event
	if _, err := s.Scan(context.Background(), func(e startup.Event) { events = append(events, e) }); err != nil {
		t.Fatal(err)
	}
	want := startup.Event{Phase: "files", State: "success", Done: 0, Total: 0}
	if len(events) == 0 || events[len(events)-1] != want {
		t.Fatalf("empty scan events = %#v", events)
	}
}
