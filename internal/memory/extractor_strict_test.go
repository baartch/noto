package memory

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"noto/internal/provider"
)

// strictStubAdapter records extraction LLM calls and returns a canned response.
type strictStubAdapter struct {
	calls    int
	lastReq  *provider.CompletionRequest
	response string
	err      error
}

func (s *strictStubAdapter) Complete(_ context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	s.calls++
	s.lastReq = &req
	if s.err != nil {
		return nil, s.err
	}
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

func TestExtractorReportsProviderFailureInsteadOfNoNewInfo(t *testing.T) {
	_, noteRepo := openSaveNoteTestDB(t)
	providerErr := errors.New("provider failed")
	adapter := &strictStubAdapter{err: providerErr}
	hook := &captureReasonsHook{}
	ex := NewExtractor(noteRepo, adapter, nil).WithLogHook(hook)
	_, err := ex.ExtractTurn(context.Background(), strictTestProfileID, strictTestProfileSlug, "", nil, "user", "assistant")
	if !errors.Is(err, providerErr) {
		t.Fatalf("ExtractTurn error = %v, want provider failure", err)
	}
	if hook.sawReason("has_new_info=false") {
		t.Fatal("provider failure was reported as no new information")
	}
}

func TestExtractorWorksWithProviderThatDoesNotSupportTemperature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if _, ok := request["temperature"]; ok {
			http.Error(w, "no provider supports temperature", http.StatusNotFound)
			return
		}
		if _, ok := request["response_format"]; !ok {
			http.Error(w, "structured output required", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"has_new_info\":true,\"confidence\":0.9,\"notes\":[{\"action\":\"add\",\"target_id\":\"\",\"category\":\"fact\",\"content\":\"Noto now has a web interface\",\"importance\":8}]}"}}]}`))
	}))
	defer server.Close()
	_, repo := openSaveNoteTestDB(t)
	adapter := provider.NewOpenAICompatible(provider.Config{Endpoint: server.URL, Model: "extractor"})
	ex := NewExtractor(repo, adapter, nil)
	result, err := ex.ExtractTurn(context.Background(), strictTestProfileID, strictTestProfileSlug, "", nil, "Noto now has a web interface", "That's useful.")
	if err != nil {
		t.Fatalf("ExtractTurn: %v", err)
	}
	if len(result.Notes) != 1 {
		t.Fatalf("stored %d notes, want 1", len(result.Notes))
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
	if hook.sawReason("has_new_info=false") {
		t.Fatal("invalid JSON was reported as no new information")
	}
}
