package integration

import (
	"context"
	"testing"

	"noto/internal/memory"
	"noto/internal/profile"
	"noto/internal/provider"
	"noto/internal/store"
	"noto/tests/integration/testutil"
)

type extractorStubAdapter struct {
	content string
}

func (s extractorStubAdapter) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return &provider.CompletionResponse{Content: s.content}, nil
}
func (s extractorStubAdapter) Embed(_ context.Context, _ provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	return &provider.EmbeddingResponse{Embedding: []float32{0.1}, Model: "stub"}, nil
}
func (s extractorStubAdapter) ProviderType() string { return "stub" }

type captureRejectHook struct {
	reasons []string
}

func (h *captureRejectHook) CandidateScored(memory.NoteCandidate)           {}
func (h *captureRejectHook) DuplicateDetected(memory.NoteCandidate, string) {}
func (h *captureRejectHook) NoteStored(memory.NoteCandidate, string)        {}
func (h *captureRejectHook) NoteStorageFailed(memory.NoteCandidate, error)  {}
func (h *captureRejectHook) ExtractionPayloadRejected(reason string) {
	h.reasons = append(h.reasons, reason)
}

func newExtractorHarness(t *testing.T) (context.Context, *store.MemoryNoteRepo, *store.Profile, *memory.Extractor) {
	t.Helper()
	db, closeDB := testutil.TempDB(t)
	t.Cleanup(closeDB)
	ctx := context.Background()
	p, err := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Extractor Contract")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	noteRepo := store.NewMemoryNoteRepo(db)
	extractor := memory.NewExtractor(noteRepo, nil, nil)
	return ctx, noteRepo, p, extractor
}

func TestExtractorJSONContract_ValidPerNoteActionsAccepted(t *testing.T) {
	ctx, noteRepo, p, _ := newExtractorHarness(t)

	ex := memory.NewExtractor(noteRepo, extractorStubAdapter{content: `{"has_new_info":true,"confidence":0.9,"notes":[{"action":"add","target_id":"","category":"fact","content":"User prefers concise responses.","importance":7}]}`}, nil)
	res, err := ex.ExtractTurn(ctx, p.ID, p.Slug, "", nil, "I prefer concise responses", "Got it")
	if err != nil {
		t.Fatalf("ExtractTurn: %v", err)
	}
	if len(res.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(res.Notes))
	}
}

func TestExtractorJSONContract_UpdateRequiresTargetID(t *testing.T) {
	ctx, noteRepo, p, _ := newExtractorHarness(t)
	hook := &captureRejectHook{}
	ex := memory.NewExtractor(noteRepo, extractorStubAdapter{content: `{"has_new_info":true,"confidence":0.8,"notes":[{"action":"update","category":"progress","content":"Status changed","importance":6}]}`}, nil).WithLogHook(hook)
	res, err := ex.ExtractTurn(ctx, p.ID, p.Slug, "", nil, "update this", "ok")
	if err != nil {
		t.Fatalf("ExtractTurn: %v", err)
	}
	if len(res.Notes) != 0 || res.Updated != 0 {
		t.Fatalf("expected rejected payload, got notes=%d updated=%d", len(res.Notes), res.Updated)
	}
	if len(hook.reasons) == 0 {
		t.Fatal("expected rejection reason to be logged")
	}
}

func TestExtractorJSONContract_CategoryEnforcement(t *testing.T) {
	ctx, noteRepo, p, _ := newExtractorHarness(t)
	hook := &captureRejectHook{}
	ex := memory.NewExtractor(noteRepo, extractorStubAdapter{content: `{"has_new_info":true,"confidence":0.8,"notes":[{"action":"add","category":"invalid","content":"X","importance":6}]}`}, nil).WithLogHook(hook)
	res, err := ex.ExtractTurn(ctx, p.ID, p.Slug, "", nil, "foo", "bar")
	if err != nil {
		t.Fatalf("ExtractTurn: %v", err)
	}
	if len(res.Notes) != 0 {
		t.Fatalf("expected 0 notes for invalid category, got %d", len(res.Notes))
	}
	if len(hook.reasons) == 0 {
		t.Fatal("expected rejection reason")
	}
}

func TestExtractorJSONContract_MalformedPayloadRejected(t *testing.T) {
	ctx, noteRepo, p, _ := newExtractorHarness(t)
	hook := &captureRejectHook{}
	ex := memory.NewExtractor(noteRepo, extractorStubAdapter{content: `not-json`}, nil).WithLogHook(hook)
	res, err := ex.ExtractTurn(ctx, p.ID, p.Slug, "", nil, "foo", "bar")
	if err != nil {
		t.Fatalf("ExtractTurn: %v", err)
	}
	if len(res.Notes) != 0 {
		t.Fatalf("expected 0 notes, got %d", len(res.Notes))
	}
	if len(hook.reasons) == 0 {
		t.Fatal("expected malformed rejection reason")
	}
}

func TestExtractorJSONContract_RequiresTopLevelMetadata(t *testing.T) {
	ctx, noteRepo, p, _ := newExtractorHarness(t)
	hook := &captureRejectHook{}
	// confidence out of range should be rejected.
	ex := memory.NewExtractor(noteRepo, extractorStubAdapter{content: `{"has_new_info":true,"confidence":1.2,"notes":[{"action":"add","target_id":"","category":"fact","content":"X","importance":3}]}`}, nil).WithLogHook(hook)
	res, err := ex.ExtractTurn(ctx, p.ID, p.Slug, "", nil, "foo", "bar")
	if err != nil {
		t.Fatalf("ExtractTurn: %v", err)
	}
	if len(res.Notes) != 0 {
		t.Fatalf("expected rejected payload, got %d notes", len(res.Notes))
	}
	if len(hook.reasons) == 0 {
		t.Fatal("expected top-level validation rejection")
	}
}

func TestExtractorJSONContract_PersistsNotesWithProfileIDNotSlug(t *testing.T) {
	ctx, noteRepo, p, _ := newExtractorHarness(t)

	ex := memory.NewExtractor(noteRepo, extractorStubAdapter{content: `{"has_new_info":true,"confidence":0.91,"notes":[{"action":"add","target_id":"","category":"fact","content":"Profile ID mapping check.","importance":6}]}`}, nil)
	res, err := ex.ExtractTurn(ctx, p.ID, p.Slug, "", nil, "remember this", "ok")
	if err != nil {
		t.Fatalf("ExtractTurn: %v", err)
	}
	if len(res.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(res.Notes))
	}

	notes, err := noteRepo.ListByProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListByProfile: %v", err)
	}
	if len(notes) == 0 {
		t.Fatal("expected persisted notes")
	}
	if notes[0].ProfileID != p.ID {
		t.Fatalf("expected note profile_id=%q, got %q", p.ID, notes[0].ProfileID)
	}
	if notes[0].ProfileID == p.Slug {
		t.Fatalf("profile_id must not be slug (%q)", p.Slug)
	}
}
