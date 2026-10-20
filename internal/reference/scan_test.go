package reference

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"noto/internal/provider"
	"noto/internal/store"
)

type recordingAdapter struct {
	mu        sync.Mutex
	calls     []provider.CompletionRequest
	response  string
	responses []string
	err       error
}

func (a *recordingAdapter) Complete(_ context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, req)
	if a.err != nil {
		return nil, a.err
	}
	response := a.response
	if len(a.responses) > 0 {
		idx := len(a.calls) - 1
		if idx >= len(a.responses) {
			idx = len(a.responses) - 1
		}
		response = a.responses[idx]
	}
	return &provider.CompletionResponse{Content: response, Usage: provider.Usage{PromptTokens: 1, HasUsage: true}}, nil
}
func (*recordingAdapter) Embed(context.Context, provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	return nil, errors.New("not used")
}
func (*recordingAdapter) ProviderType() string { return "test" }
func (a *recordingAdapter) callCount() int     { a.mu.Lock(); defer a.mu.Unlock(); return len(a.calls) }
func (a *recordingAdapter) requests() []provider.CompletionRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]provider.CompletionRequest(nil), a.calls...)
}

func TestReferenceScanSummarizesChangesAndDeletesOrphans(t *testing.T) {
	root := t.TempDir()
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := store.NewReferenceFileRepo(db)
	adapter := &recordingAdapter{response: "Artist identity and tone"}
	service, err := NewService(Options{Root: root, ProfileID: "p1", Repo: repo, Model: func() Model { return Model{Adapter: adapter} }})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	filename := filepath.Join(root, "artist.txt")
	if err := os.WriteFile(filename, []byte("first text"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	if adapter.callCount() != 1 {
		t.Fatalf("summary calls = %d, want 1", adapter.callCount())
	}
	files, err := repo.List(context.Background(), "p1")
	if err != nil || len(files) != 1 || files[0].Status != store.ReferenceFileReady || files[0].Summary != adapter.response || files[0].Fingerprint != files[0].SummaryFingerprint {
		t.Fatalf("indexed row = %#v, %v", files, err)
	}
	oldInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	if adapter.callCount() != 1 {
		t.Fatalf("unchanged file triggered summary call; calls=%d", adapter.callCount())
	}
	if err := os.WriteFile(filename, []byte("second text"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filename, oldInfo.ModTime(), oldInfo.ModTime()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	if adapter.callCount() != 2 {
		t.Fatalf("same-size changed file was not summarized; calls=%d", adapter.callCount())
	}
	files, err = repo.List(context.Background(), "p1")
	if err != nil || files[0].Fingerprint == "" || files[0].Fingerprint != files[0].SummaryFingerprint {
		t.Fatalf("changed row is stale: %#v, %v", files, err)
	}
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	deleted, err := service.Scan(context.Background())
	if err != nil || len(deleted) != 0 {
		t.Fatalf("deleted scan diagnostics = %#v, %v", deleted, err)
	}
	files, err = repo.List(context.Background(), "p1")
	if err != nil || len(files) != 0 {
		t.Fatalf("orphan row not removed: %#v, %v", files, err)
	}
}

func TestReferenceScanFailureDoesNotPublishSummary(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("guide body"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	adapter := &recordingAdapter{err: errors.New("provider unavailable")}
	service, err := NewService(Options{Root: root, ProfileID: "p", Repo: store.NewReferenceFileRepo(db), Model: func() Model { return Model{Adapter: adapter} }})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	diagnostics, err := service.Scan(context.Background())
	if err != nil || len(diagnostics) != 1 {
		t.Fatalf("Scan() = %#v, %v", diagnostics, err)
	}
	files, err := store.NewReferenceFileRepo(db).List(context.Background(), "p")
	if err != nil || len(files) != 1 || files[0].Status != store.ReferenceFileFailed || files[0].Summary != "" || files[0].SummaryFingerprint != "" {
		t.Fatalf("failed file row = %#v, %v", files, err)
	}
}

func TestReferenceSummaryProcessesEveryBoundedSourcePart(t *testing.T) {
	root := t.TempDir()
	body := "start-marker" + strings.Repeat("x", 9000) + "end-marker"
	if err := os.WriteFile(filepath.Join(root, "large.txt"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	adapter := &recordingAdapter{response: "The document describes a creative identity."}
	usageCalls := 0
	service, err := NewService(Options{
		Root: root, ProfileID: "p", Repo: store.NewReferenceFileRepo(db),
		Model:   func() Model { return Model{Adapter: adapter, ContextMax: 8192} },
		OnUsage: func(provider.Usage) { usageCalls++ },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	requests := adapter.requests()
	if len(requests) < 3 || usageCalls != len(requests) {
		t.Fatalf("summary calls=%d usage callbacks=%d, want chunk+reduction calls and usage for each", len(requests), usageCalls)
	}
	var transmitted strings.Builder
	for _, req := range requests {
		for _, message := range req.Messages {
			transmitted.WriteString(message.Content)
		}
	}
	if !strings.Contains(transmitted.String(), "start-marker") || !strings.Contains(transmitted.String(), "end-marker") {
		t.Fatal("not all source parts participated in summary requests")
	}
}

func TestReferenceServiceRejectsSymlinkRoot(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "files")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = NewService(Options{Root: link, ProfileID: "p", Repo: store.NewReferenceFileRepo(db)})
	if err == nil || !strings.Contains(err.Error(), "real directory") {
		t.Fatalf("NewService error = %v, want symlink root rejection", err)
	}
}

func TestReferenceScanHonorsCanceledContext(t *testing.T) {
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err := NewService(Options{Root: t.TempDir(), ProfileID: "p", Repo: store.NewReferenceFileRepo(db)})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Scan(ctx); err != context.Canceled {
		t.Fatalf("Scan error = %v, want canceled", err)
	}
}
