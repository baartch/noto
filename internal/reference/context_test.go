package reference

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"noto/internal/provider"
	"noto/internal/store"
)

func TestReferencePrepareSelectsAndEvictsTopicContext(t *testing.T) {
	root := t.TempDir()
	body := "private artist identity source"
	if err := os.WriteFile(filepath.Join(root, "identity.txt"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := store.NewReferenceFileRepo(db)
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), rootHandle, "identity.txt")
	_ = rootHandle.Close()
	if err != nil {
		t.Fatal(err)
	}
	file := store.ReferenceFile{ID: "rf-identity", ProfileID: "p", RelativePath: "identity.txt", MediaType: doc.MediaType, Fingerprint: doc.Fingerprint, SummaryFingerprint: doc.Fingerprint, Summary: "Artist identity, voice, and public tone.", Status: store.ReferenceFileReady}
	if err := repo.Upsert(context.Background(), file); err != nil {
		t.Fatal(err)
	}
	adapter := &recordingAdapter{responses: []string{
		`{"topic":"artist identity","topic_changed":false,"file_ids":["rf-identity"]}`,
		`{"topic":"artist identity","topic_changed":false,"file_ids":["rf-identity"]}`,
		`{"topic":"weather","topic_changed":true,"file_ids":[]}`,
	}}
	service, err := NewService(Options{Root: root, ProfileID: "p", Repo: repo, Model: func() Model { return Model{Adapter: adapter, ContextMax: 8192} }})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	input := Input{UserMessage: "Write my public artist bio", RecentUserMessages: []string{"I am an artist"}, BaseMessages: []provider.Message{{Role: "system", Content: "system"}}, ContextMax: 8192}
	first, err := service.Prepare(context.Background(), input)
	if err != nil || first.Degraded || len(first.Attachments) != 1 || first.Attachments[0].Text != body {
		t.Fatalf("first Prepare() = %#v, %v", first, err)
	}
	selector := adapter.requests()[0]
	for _, message := range selector.Messages {
		if strings.Contains(message.Content, body) {
			t.Fatal("selector received raw document text")
		}
	}
	input.UserMessage = "Make that shorter"
	continued, err := service.Prepare(context.Background(), input)
	if err != nil || len(continued.Attachments) != 1 {
		t.Fatalf("continuation dropped active document: %#v, %v", continued, err)
	}
	input.UserMessage = "Will it rain tomorrow?"
	unrelated, err := service.Prepare(context.Background(), input)
	if err != nil || len(unrelated.Attachments) != 0 {
		t.Fatalf("topic change retained document: %#v, %v", unrelated, err)
	}
}

func TestReferencePrepareFailsClosedOnBadSelection(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("secret body"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := store.NewReferenceFileRepo(db)
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), rootHandle, "guide.md")
	_ = rootHandle.Close()
	if err != nil {
		t.Fatal(err)
	}
	file := store.ReferenceFile{ID: "rf-guide", ProfileID: "p", RelativePath: "guide.md", Fingerprint: doc.Fingerprint, SummaryFingerprint: doc.Fingerprint, Summary: "Guide", Status: store.ReferenceFileReady}
	if err := repo.Upsert(context.Background(), file); err != nil {
		t.Fatal(err)
	}
	for _, response := range []string{
		`{"topic":"x","topic_changed":false,"file_ids":["unknown"]}`,
		`not json`,
		`{"topic":"x","topic_changed":false,"file_ids":["rf-guide","rf-guide"]}`,
	} {
		adapter := &recordingAdapter{response: response}
		service, err := NewService(Options{Root: root, ProfileID: "p", Repo: repo, Model: func() Model { return Model{Adapter: adapter} }})
		if err != nil {
			t.Fatal(err)
		}
		turn, err := service.Prepare(context.Background(), Input{UserMessage: "Use that document", ContextMax: 8192})
		_ = service.Close()
		if err != nil || !turn.Degraded || len(turn.Attachments) != 0 || len(turn.Diagnostics) == 0 {
			t.Fatalf("bad output did not fail closed: %#v, %v", turn, err)
		}
	}
}

func TestReferencePrepareCatalogOverflowDisablesSelection(t *testing.T) {
	root := t.TempDir()
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := store.NewReferenceFileRepo(db)
	file := store.ReferenceFile{ID: "rf-big", ProfileID: "p", RelativePath: "guide.md", Fingerprint: "f", SummaryFingerprint: "f", Summary: strings.Repeat("x", maxCatalogBytes+1), Status: store.ReferenceFileReady}
	if err := repo.Upsert(context.Background(), file); err != nil {
		t.Fatal(err)
	}
	adapter := &recordingAdapter{response: `{}`}
	service, err := NewService(Options{Root: root, ProfileID: "p", Repo: repo, Model: func() Model { return Model{Adapter: adapter} }})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	turn, err := service.Prepare(context.Background(), Input{UserMessage: "hello"})
	if err != nil || !turn.Degraded || len(turn.Diagnostics) == 0 || adapter.callCount() != 0 {
		t.Fatalf("oversized catalog did not disable selection: %#v calls=%d err=%v", turn, adapter.callCount(), err)
	}
}

func TestFormatCatalogWithNoFilesIsEmpty(t *testing.T) {
	catalog, encoded, err := formatCatalog(nil)
	if err != nil || catalog != "" || len(encoded) != 0 {
		t.Fatalf("empty catalog = %q (%q), %v", catalog, encoded, err)
	}
}

func TestReferencePrepareRefreshesChangedFileBeforeUse(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "guide.txt")
	if err := os.WriteFile(filename, []byte("old body"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := store.NewReferenceFileRepo(db)
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), rootHandle, "guide.txt")
	_ = rootHandle.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Upsert(context.Background(), store.ReferenceFile{ID: "rf-guide", ProfileID: "p", RelativePath: "guide.txt", Fingerprint: doc.Fingerprint, SummaryFingerprint: doc.Fingerprint, Summary: "Old description", Status: store.ReferenceFileReady}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte("new body"), 0o600); err != nil {
		t.Fatal(err)
	}
	decision := `{"topic":"guide","topic_changed":false,"file_ids":["rf-guide"]}`
	adapter := &recordingAdapter{responses: []string{decision, "Fresh description", decision}}
	service, err := NewService(Options{Root: root, ProfileID: "p", Repo: repo, Model: func() Model { return Model{Adapter: adapter} }})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	turn, err := service.Prepare(context.Background(), Input{UserMessage: "Use the guide", ContextMax: 8192})
	if err != nil || turn.Degraded || len(turn.Attachments) != 1 || turn.Attachments[0].Text != "new body" {
		t.Fatalf("Prepare did not use refreshed source: %#v, %v", turn, err)
	}
	files, err := repo.List(context.Background(), "p")
	if err != nil || files[0].Summary != "Fresh description" || files[0].Fingerprint != files[0].SummaryFingerprint {
		t.Fatalf("catalog was not refreshed: %#v, %v", files, err)
	}
	requests := adapter.requests()
	if len(requests) != 3 || strings.Contains(requests[0].Messages[1].Content, "new body") || strings.Contains(requests[2].Messages[1].Content, "new body") {
		t.Fatalf("selector did not re-run without raw text after refresh: %#v", requests)
	}
}

func TestReferencePrepareRemovesConfirmedDeletedFile(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "gone.md")
	if err := os.WriteFile(filename, []byte("body"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := store.NewReferenceFileRepo(db)
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), rootHandle, "gone.md")
	_ = rootHandle.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Upsert(context.Background(), store.ReferenceFile{ID: "rf-gone", ProfileID: "p", RelativePath: "gone.md", Fingerprint: doc.Fingerprint, SummaryFingerprint: doc.Fingerprint, Summary: "Guide", Status: store.ReferenceFileReady}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	adapter := &recordingAdapter{response: `{"topic":"guide","topic_changed":false,"file_ids":["rf-gone"]}`}
	service, err := NewService(Options{Root: root, ProfileID: "p", Repo: repo, Model: func() Model { return Model{Adapter: adapter} }})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	turn, err := service.Prepare(context.Background(), Input{UserMessage: "Use the deleted guide", ContextMax: 8192})
	if err != nil || !turn.Degraded || len(turn.Attachments) != 0 || len(turn.Diagnostics) == 0 {
		t.Fatalf("deleted file was not reported unavailable: %#v, %v", turn, err)
	}
	if strings.Contains(turn.Catalog, "rf-gone") || strings.Contains(turn.Catalog, "Guide") {
		t.Fatalf("turn retained stale description for a deleted file: %s", turn.Catalog)
	}
	files, err := repo.List(context.Background(), "p")
	if err != nil || len(files) != 0 {
		t.Fatalf("confirmed missing file remains cataloged: %#v, %v", files, err)
	}
}

func TestReferencePrepareInvalidatesCatalogOnReadFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "unreadable.md"), []byte("body"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := store.NewReferenceFileRepo(db)
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), rootHandle, "unreadable.md")
	_ = rootHandle.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Upsert(context.Background(), store.ReferenceFile{ID: "rf-unreadable", ProfileID: "p", RelativePath: "unreadable.md", Fingerprint: doc.Fingerprint, SummaryFingerprint: doc.Fingerprint, Summary: "Stale voice description", Status: store.ReferenceFileReady}); err != nil {
		t.Fatal(err)
	}
	adapter := &recordingAdapter{response: `{"topic":"guide","topic_changed":false,"file_ids":["rf-unreadable"]}`}
	service, err := NewService(Options{
		Root: root, ProfileID: "p", Repo: repo,
		Reader: func(context.Context, *os.Root, string) (Document, error) { return Document{}, ErrUnreadable },
		Model:  func() Model { return Model{Adapter: adapter} },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	turn, err := service.Prepare(context.Background(), Input{UserMessage: "Use the guide", ContextMax: 8192})
	if err != nil || !turn.Degraded || strings.Contains(turn.Catalog, "Stale voice description") {
		t.Fatalf("read failure left stale catalog in turn: %#v, %v", turn, err)
	}
	files, err := repo.List(context.Background(), "p")
	if err != nil || len(files) != 1 || files[0].Status != store.ReferenceFileFailed || files[0].Summary != "" {
		t.Fatalf("read-failed catalog row = %#v, %v", files, err)
	}
}

func TestReferencePrepareRejectsSelectedSetOverContextBudget(t *testing.T) {
	root := t.TempDir()
	body := strings.Repeat("long reference ", 400)
	if err := os.WriteFile(filepath.Join(root, "large.txt"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := store.NewReferenceFileRepo(db)
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), rootHandle, "large.txt")
	_ = rootHandle.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Upsert(context.Background(), store.ReferenceFile{ID: "rf-large", ProfileID: "p", RelativePath: "large.txt", Fingerprint: doc.Fingerprint, SummaryFingerprint: doc.Fingerprint, Summary: "Large guide", Status: store.ReferenceFileReady}); err != nil {
		t.Fatal(err)
	}
	adapter := &recordingAdapter{response: `{"topic":"guide","topic_changed":false,"file_ids":["rf-large"]}`}
	service, err := NewService(Options{Root: root, ProfileID: "p", Repo: repo, Model: func() Model { return Model{Adapter: adapter} }})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	turn, err := service.Prepare(context.Background(), Input{UserMessage: "Use all of the guide", ContextMax: 8192})
	if err != nil || !turn.Degraded || len(turn.Attachments) != 0 || len(turn.Diagnostics) == 0 {
		t.Fatalf("oversized selection was truncated or accepted: %#v, %v", turn, err)
	}
}

func TestReferencePrepareBudgetsJSONEscapedAttachmentBytes(t *testing.T) {
	root := t.TempDir()
	body := strings.Repeat("\x00", 1800)
	if err := os.WriteFile(filepath.Join(root, "control.txt"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := store.NewReferenceFileRepo(db)
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Read(context.Background(), rootHandle, "control.txt")
	_ = rootHandle.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Upsert(context.Background(), store.ReferenceFile{ID: "rf-control", ProfileID: "p", RelativePath: "control.txt", Fingerprint: doc.Fingerprint, SummaryFingerprint: doc.Fingerprint, Summary: "Control text", Status: store.ReferenceFileReady}); err != nil {
		t.Fatal(err)
	}
	adapter := &recordingAdapter{response: `{"topic":"controls","topic_changed":false,"file_ids":["rf-control"]}`}
	service, err := NewService(Options{Root: root, ProfileID: "p", Repo: repo, Model: func() Model { return Model{Adapter: adapter} }})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	turn, err := service.Prepare(context.Background(), Input{UserMessage: "Use the control file", ContextMax: 8192})
	if err != nil || !turn.Degraded || len(turn.Attachments) != 0 || len(turn.Diagnostics) == 0 {
		t.Fatalf("JSON-escaped attachment exceeded budget but was sent: %#v, %v", turn, err)
	}
}
