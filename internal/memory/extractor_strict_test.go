package memory

import (
	"context"
	"slices"
	"testing"

	"noto/internal/provider"
)

// strictStubAdapter records extraction LLM calls and returns a canned response.
type strictStubAdapter struct {
	calls    int
	lastReq  *provider.CompletionRequest
	response string
}

func (s *strictStubAdapter) Complete(_ context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	s.calls++
	s.lastReq = &req
	return &provider.CompletionResponse{Content: s.response}, nil
}
func (s *strictStubAdapter) Embed(_ context.Context, _ provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	return &provider.EmbeddingResponse{Embedding: []float32{0.1}, Model: "stub"}, nil
}
func (s *strictStubAdapter) ProviderType() string { return "stub" }

// captureReasonsHook records ExtractionPayloadRejected reasons.
type captureReasonsHook struct {
	reasons []string
}

func (h *captureReasonsHook) CandidateScored(NoteCandidate)           {}
func (h *captureReasonsHook) DuplicateDetected(NoteCandidate, string) {}
func (h *captureReasonsHook) NoteStored(NoteCandidate, string)        {}
func (h *captureReasonsHook) NoteStorageFailed(NoteCandidate, error)  {}
func (h *captureReasonsHook) ExtractionPayloadRejected(reason string) {
	h.reasons = append(h.reasons, reason)
}

func (h *captureReasonsHook) sawReason(reason string) bool {
	return slices.Contains(h.reasons, reason)
}

const (
	strictTestProfileID   = "p-strict"
	strictTestProfileSlug = "strict-profile"
)

func TestExtractorIneligible_MakesNoLLMCall(t *testing.T) {
	_, noteRepo := openSaveNoteTestDB(t)
	adapter := &strictStubAdapter{response: `{"has_new_info":true,"confidence":0.9,"notes":[{"action":"add","target_id":"","category":"fact","content":"x","importance":5}]}`}
	ex := NewExtractor(noteRepo, adapter, nil).WithStructuredOutputsEligible(false)
	res, err := ex.ExtractTurn(context.Background(), strictTestProfileID, strictTestProfileSlug, "", nil, "user", "assistant")
	if err != nil {
		t.Fatalf("ExtractTurn: %v", err)
	}
	if adapter.calls != 0 {
		t.Fatalf("LLM called %d times, want 0", adapter.calls)
	}
	if res == nil || len(res.Notes) != 0 {
		t.Fatalf("want empty result, got %+v", res)
	}
}

func TestExtractorSendsResponseFormat(t *testing.T) {
	_, noteRepo := openSaveNoteTestDB(t)
	adapter := &strictStubAdapter{response: `{"has_new_info":false,"confidence":0.9,"notes":[]}`}
	ex := NewExtractor(noteRepo, adapter, nil).WithStructuredOutputsEligible(true)
	_, _ = ex.ExtractTurn(context.Background(), strictTestProfileID, strictTestProfileSlug, "", nil, "user", "assistant")
	if adapter.lastReq == nil || adapter.lastReq.ResponseFormat == nil {
		t.Fatal("expected ResponseFormat on the completion request")
	}
	rf := adapter.lastReq.ResponseFormat
	if rf.Name != "memory_extraction" || !rf.Strict || rf.Schema == nil {
		t.Fatalf("ResponseFormat = %+v, want name memory_extraction strict schema", rf)
	}
}

func TestExtractorRejectsFencedJSON(t *testing.T) {
	_, noteRepo := openSaveNoteTestDB(t)
	adapter := &strictStubAdapter{response: "```json\n" + `{"has_new_info":true,"confidence":0.9,"notes":[]}` + "\n```"}
	hook := &captureReasonsHook{}
	ex := NewExtractor(noteRepo, adapter, nil).WithStructuredOutputsEligible(true).WithLogHook(hook)
	res, err := ex.ExtractTurn(context.Background(), strictTestProfileID, strictTestProfileSlug, "", nil, "user", "assistant")
	if err != nil {
		t.Fatalf("ExtractTurn: %v", err)
	}
	if len(res.Notes) != 0 {
		t.Fatal("fenced JSON must not be accepted")
	}
	if !hook.sawReason("invalid json") {
		t.Fatalf("expected 'invalid json' rejection, got %v", hook.reasons)
	}
}
