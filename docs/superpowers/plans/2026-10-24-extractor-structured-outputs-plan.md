# Extractor Structured Outputs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the memory extractor use OpenRouter-style structured outputs (strict JSON schema) and refuse to run with any model that doesn't report `structured_outputs` support.

**Architecture:** The provider layer gains a `ResponseFormat` field on `CompletionRequest` and parses `structured_outputs` from `/models` into `ModelInfo`. The extractor builds a strict JSON schema from its existing response structs, sends it with every request, and gates all LLM calls on an injected eligibility flag computed by the app layer (effective extractor model = configured extractor model, else the chat model under the existing fallback). No lenient fallback anywhere: the fence-stripping parse path is deleted.

**Tech Stack:** Go 1.26+, stdlib only (no new dependencies). Existing provider adapter (`internal/provider`), memory package, chat session, commands registry, Bubble Tea v2 TUI.

**Spec:** `/home/andy/.opencode/plan/2026-10-24-extractor-structured-outputs-design.md` (final home once committed: `docs/superpowers/specs/2026-10-24-extractor-structured-outputs-design.md`). The plan argues from the spec; executors read both.

## Global Constraints

- Go 1.26+. **No new dependencies** — stdlib covers everything; do not run `make tidy` unless go.mod changed (it shouldn't).
- Run `make fmt lint test` before each commit; fix anything it reports.
- Core rule, verbatim from the spec: eligibility means `/models` reports `structured_outputs` in `supported_parameters`; **known-unsupported, unreported support, fetch failure, and model-absent-from-list all mean ineligible. No exceptions, no fallback.**
- The fence-stripping/lenient-parse code in `llmExtract` is **deleted**, not flagged.
- `validateExtractionPayload` stays as defense-in-depth after parsing.
- `response_format` must be absent from ordinary chat completions (only set when the caller passes it).
- OpenRouter-only `require_parameters` must never be sent to non-OpenRouter endpoints.
- Use lean-ctx MCP tools (`ctx_read`, `ctx_search`, `ctx_compose`) for file reads/searches during implementation.
- Commit messages: conventional style (`feat:`, `test:`, `refactor:`), one logical change per commit.

## Review Focus

Five input classes the spec implies that no happy path covers — each pinned by a test in the task that owns the code:

1. **Endpoint reports no `supported_parameters`** (vanilla OpenAI, LM Studio, Ollama): model must be ineligible. Test: Task 1 (`TestListModels_ParsesStructuredOutputsSupport` absent-field case).
2. **`/models` fetch fails** (flaky network): session eligibility stays at its zero-value `false` → extraction paused + footer warning. Test: Task 5 (`TestSession_ExtractorEligibilityDefaultsFalse`).
3. **Model wraps JSON in fences/prose despite the schema**: now rejected (the lenient rescue is gone). Test: Task 4 (`TestExtractorRejectsFencedJSON`).
4. **Configured extractor model absent from the fetched list** (typo, removed model): ineligible. Test: Task 7 (`TestResolveExtractorEligibility` unknown-ID case).
5. **`require_parameters` leaking to non-OpenRouter endpoints** would break strict servers. Test: Task 2 (`TestBuildChatCompletionsPayload_NoProviderPrefsForNonOpenRouter` + `TestIsOpenRouterEndpoint`).

Also tested (falls out of task tests): chat-model fallback re-evaluation — Task 7 `TestResolveExtractorEligibility` fallback cases.

---

### Task 1: Parse `structured_outputs` into `ModelInfo`

**Files:**
- Modify: `internal/provider/models.go` (`ModelInfo` struct ~L17; parse loop ~L88 where `supportsTools` is computed)
- Test: `tests/unit/provider/models_test.go` (existing file, add test)

**Interfaces:**
- Consumes: nothing new.
- Produces: `ModelInfo.SupportsStructuredOutputs bool` — later tasks (7, 8) and the command (Task 6) read this field.

- [ ] **Step 1: Write the failing test**

Add to `tests/unit/provider/models_test.go`:

```go
func TestListModels_ParsesStructuredOutputsSupport(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[
			{"id":"model-a","context_length":1000,"supported_parameters":["structured_outputs","tools"]},
			{"id":"model-b","context_length":1000,"supported_parameters":["tools"]},
			{"id":"model-c","context_length":1000}
		]}`))
	}))
	defer ts.Close()

	models, err := provider.ListModels(context.Background(), provider.Config{Endpoint: ts.URL})
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	byID := map[string]provider.ModelInfo{}
	for _, m := range models {
		byID[m.ID] = m
	}
	if !byID["model-a"].SupportsStructuredOutputs {
		t.Fatalf("model-a: expected structured outputs support")
	}
	if byID["model-b"].SupportsStructuredOutputs {
		t.Fatalf("model-b: expected no structured outputs support")
	}
	if byID["model-c"].SupportsStructuredOutputs {
		t.Fatalf("model-c: expected no support when supported_parameters absent")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests/unit/provider/ -run TestListModels_ParsesStructuredOutputsSupport -v`
Expected: FAIL — `models[i].SupportsStructuredOutputs undefined`

- [ ] **Step 3: Implement in `internal/provider/models.go`**

Add field `SupportsStructuredOutputs bool` to `ModelInfo`. In the parse loop, next to the existing `supportsTools` line:

```go
supportsStructured := slices.Contains(m.SupportedParameters, "structured_outputs")
```

and set `SupportsStructuredOutputs: supportsStructured` in the `ModelInfo` literal.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tests/unit/provider/ -run TestListModels_ParsesStructuredOutputsSupport -v`
Expected: PASS

- [ ] **Step 5: Run full provider tests + lint, commit**

Run: `make fmt lint test` (or at minimum `go test ./internal/provider/... ./tests/unit/provider/...`)
Commit:

```bash
git add internal/provider/models.go tests/unit/provider/models_test.go
git commit -m "feat(provider): parse structured_outputs model support"
```

---

### Task 2: `ResponseFormat` on requests + OpenRouter `require_parameters`

**Files:**
- Modify: `internal/provider/adapter.go` (`CompletionRequest` L58–64)
- Modify: `internal/provider/openai_compatible.go` (`Complete` ~L146–289; wire types ~L291–368)
- Test: create `internal/provider/openai_compatible_internal_test.go` (package `provider`); create `tests/unit/provider/openai_compatible_response_format_test.go` (package `provider_test`)

**Interfaces:**
- Consumes: nothing new.
- Produces (Tasks 4 depends on all of these):
  - `type ResponseFormat struct { Name string; Strict bool; Schema map[string]any }` in `internal/provider/adapter.go`
  - `CompletionRequest.ResponseFormat *ResponseFormat` — nil means ordinary chat request (wire field omitted)
  - Wire JSON when set: `"response_format": {"type":"json_schema","json_schema":{"name":...,"strict":...,"schema":...}}`
  - `func isOpenRouterEndpoint(endpoint string) bool` — case-insensitive substring check for `"openrouter.ai"` on the configured endpoint
  - On OpenRouter endpoints with `ResponseFormat` set, payload also includes `"provider":{"require_parameters":true}`; never otherwise

- [ ] **Step 1: Write the failing tests**

`internal/provider/openai_compatible_internal_test.go` (package `provider`):

```go
func TestIsOpenRouterEndpoint(t *testing.T) {
	if !isOpenRouterEndpoint("https://openrouter.ai/api/v1") {
		t.Fatal("openrouter.ai endpoint not detected")
	}
	if !isOpenRouterEndpoint("https://OpenRouter.AI/api/v1") {
		t.Fatal("case-insensitive host not detected")
	}
	if isOpenRouterEndpoint("https://api.openai.com/v1") {
		t.Fatal("non-OpenRouter endpoint misdetected")
	}
	if isOpenRouterEndpoint("") {
		t.Fatal("empty endpoint misdetected")
	}
}
```

Plus payload-shape tests in the same file (these require the Step 3 refactor):

```go
func TestBuildChatCompletionsPayload_ResponseFormat(t *testing.T) {
	req := CompletionRequest{Messages: []Message{{Role: "user", Content: "hi"}},
		ResponseFormat: &ResponseFormat{Name: "test_schema", Strict: true, Schema: map[string]any{"type": "object"}}}
	payload := buildChatCompletionsPayload(Config{Endpoint: "https://api.openai.com/v1"}, req)
	if payload.ResponseFormat == nil || payload.ProviderPrefs != nil {
		t.Fatalf("want response_format without provider prefs, got %+v / %+v", payload.ResponseFormat, payload.ProviderPrefs)
	}
	if payload.ResponseFormat.JSONSchema.Name != "test_schema" || !payload.ResponseFormat.JSONSchema.Strict {
		t.Fatalf("schema name/strict mismatch: %+v", payload.ResponseFormat.JSONSchema)
	}
	if payload.ResponseFormat.Type != "json_schema" {
		t.Fatalf("type = %q, want json_schema", payload.ResponseFormat.Type)
	}
}

func TestBuildChatCompletionsPayload_NoProviderPrefsForNonOpenRouter(t *testing.T) {
	req := CompletionRequest{ResponseFormat: &ResponseFormat{Name: "s", Strict: true, Schema: map[string]any{}}}
	payload := buildChatCompletionsPayload(Config{Endpoint: "https://api.example.com/v1"}, req)
	if payload.ProviderPrefs != nil {
		t.Fatal("provider prefs must not be set for non-OpenRouter endpoints")
	}
}

func TestBuildChatCompletionsPayload_OpenRouterRequiresParameters(t *testing.T) {
	req := CompletionRequest{ResponseFormat: &ResponseFormat{Name: "s", Strict: true, Schema: map[string]any{}}}
	payload := buildChatCompletionsPayload(Config{Endpoint: "https://openrouter.ai/api/v1"}, req)
	if payload.ProviderPrefs == nil || !payload.ProviderPrefs.RequireParameters {
		t.Fatal("want require_parameters=true on OpenRouter")
	}
	// Without ResponseFormat, even OpenRouter gets no prefs:
	payload = buildChatCompletionsPayload(Config{Endpoint: "https://openrouter.ai/api/v1"}, CompletionRequest{})
	if payload.ProviderPrefs != nil {
		t.Fatal("provider prefs sent without response_format")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/provider/ -run 'TestIsOpenRouterEndpoint|TestBuildChatCompletionsPayload' -v`
Expected: FAIL — `undefined: isOpenRouterEndpoint` / `buildChatCompletionsPayload`

- [ ] **Step 3: Implement**

1. `internal/provider/adapter.go`: add the `ResponseFormat` struct (doc comment: "ResponseFormat requests provider-enforced structured output via json_schema.") and `ResponseFormat *ResponseFormat` field on `CompletionRequest`.
2. `internal/provider/openai_compatible.go`, wire types:

```go
type openAIJSONSchemaFormat struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type openAIResponseFormat struct {
	Type       string              `json:"type"` // "json_schema"
	JSONSchema openAIJSONSchemaFormat `json:"json_schema"`
}

type openAIProviderPreferences struct {
	RequireParameters bool `json:"require_parameters,omitempty"`
}
```

Add to `openAIChatCompletionsRequest`:

```go
ResponseFormat *openAIResponseFormat       `json:"response_format,omitempty"`
ProviderPrefs  *openAIProviderPreferences  `json:"provider,omitempty"`
```

3. Refactor `Complete`: extract everything between `model := req.Model ...` and the `json.Marshal(payload)` (payload construction: model resolution, tools, messages, and the new fields below) into:

```go
func buildChatCompletionsPayload(cfg Config, req CompletionRequest) openAIChatCompletionsRequest
```

Inside it, after the existing construction:

```go
if req.ResponseFormat != nil {
	payload.ResponseFormat = &openAIResponseFormat{
		Type:       "json_schema",
		JSONSchema: openAIJSONSchemaFormat{Name: req.ResponseFormat.Name, Strict: req.ResponseFormat.Strict, Schema: req.ResponseFormat.Schema},
	}
	if isOpenRouterEndpoint(cfg.Endpoint) {
		payload.ProviderPrefs = &openAIProviderPreferences{RequireParameters: true}
	}
}
```

`Complete` then calls `buildChatCompletionsPayload(a.cfg, req)` and continues from `json.Marshal` unchanged. Add:

```go
func isOpenRouterEndpoint(endpoint string) bool {
	return strings.Contains(strings.ToLower(endpoint), "openrouter.ai")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/provider/ -run 'TestIsOpenRouterEndpoint|TestBuildChatCompletionsPayload' -v`
Expected: PASS (4 tests)

- [ ] **Step 5: End-to-end adapter test (wire JSON, non-OpenRouter endpoint)**

Create `tests/unit/provider/openai_compatible_response_format_test.go` (package `provider_test`): httptest server captures the POST body, returns a minimal choices response; call `Complete` with a request whose `ResponseFormat` is set; unmarshal captured body into `map[string]any`; assert:
- `response_format.type == "json_schema"`, `response_format.json_schema.name` and `.strict` round-trip, `.strict == true`
- no `provider` key present (httptest URL is not an OpenRouter endpoint)

Run: `go test ./tests/unit/provider/ -run TestCompleteMarshalsResponseFormat -v` → PASS

- [ ] **Step 6: Full verification + commit**

Run: `make fmt lint test`
Commit:

```bash
git add internal/provider/adapter.go internal/provider/openai_compatible.go internal/provider/openai_compatible_internal_test.go tests/unit/provider/openai_compatible_response_format_test.go
git commit -m "feat(provider): response_format json_schema support with OpenRouter require_parameters"
```

---

### Task 3: Strict JSON schema for the extraction contract

**Files:**
- Create: `internal/memory/extractor_schema.go`
- Test: create `internal/memory/extractor_schema_test.go` (package `memory`)

**Interfaces:**
- Consumes: JSON shape of `extractionResponse` / `extractedItem` (field names, enums) from `internal/memory/extractor.go`.
- Produces: `func extractionSchema() map[string]any` — Task 4 passes it as `provider.ResponseFormat.Schema`.

- [ ] **Step 1: Write the failing test**

`internal/memory/extractor_schema_test.go`:

```go
func TestExtractionSchemaStrictShape(t *testing.T) {
	s := extractionSchema()
	if s["type"] != "object" {
		t.Fatalf("type = %v, want object", s["type"])
	}
	if s["additionalProperties"] != false {
		t.Fatalf("additionalProperties = %v, want false", s["additionalProperties"])
	}
	req, ok := s["required"].([]string)
	if !ok || len(req) != 3 {
		t.Fatalf("required = %v, want [has_new_info confidence notes]", s["required"])
	}
	props := s["properties"].(map[string]any)
	if len(props) != 3 {
		t.Fatalf("properties = %v, want exactly has_new_info confidence notes", props)
	}
	notes := props["notes"].(map[string]any)
	items := notes["items"].(map[string]any)
	if items["additionalProperties"] != false {
		t.Fatalf("items additionalProperties = %v, want false", items["additionalProperties"])
	}
	itemProps := items["properties"].(map[string]any)
	if len(itemProps) != 5 {
		t.Fatalf("item properties = %v, want exactly action target_id category content importance", itemProps)
	}
	if got := itemProps["action"].(map[string]any)["enum"].([]string); len(got) != 3 {
		t.Fatalf("action enum = %v, want [add update merge]", got)
	}
	if got := itemProps["category"].(map[string]any)["enum"].([]string); len(got) != 5 {
		t.Fatalf("category enum = %v, want [fact progress blocker action_item other]", got)
	}
	itemReq, ok := items["required"].([]string)
	if !ok || len(itemReq) != 5 {
		t.Fatalf("items required = %v, want all five item keys", items["required"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/memory/ -run TestExtractionSchemaStrictShape -v`
Expected: FAIL — `undefined: extractionSchema`

- [ ] **Step 3: Implement `extractionSchema()` in `internal/memory/extractor_schema.go`**

Build this exact schema (typed as `map[string]any`, values may be typed as `[]string` for `required`/`enum` as in the test):

```json
{
  "type": "object",
  "properties": {
    "has_new_info": {
      "type": "boolean",
      "description": "Whether the exchange contains at least one piece of durable information worth remembering."
    },
    "confidence": {
      "type": "number",
      "description": "Confidence in the extraction, from 0.0 to 1.0."
    },
    "notes": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "action": {
            "type": "string",
            "enum": ["add", "update", "merge"],
            "description": "Action to take: add a new note, update an existing note, or merge duplicates."
          },
          "target_id": {
            "type": "string",
            "description": "ID of the existing note to update. Required when action is update; empty otherwise."
          },
          "category": {
            "type": "string",
            "enum": ["fact", "progress", "blocker", "action_item", "other"],
            "description": "The note category."
          },
          "content": {
            "type": "string",
            "description": "The note content, concise and self-contained."
          },
          "importance": {
            "type": "integer",
            "description": "Importance from 1 (trivial) to 10 (critical)."
          }
        },
        "required": ["action", "target_id", "category", "content", "importance"],
        "additionalProperties": false
      }
    }
  },
  "required": ["has_new_info", "confidence", "notes"],
  "additionalProperties": false
}
```

Notes: `mergeIDs` is deliberately absent (unexported, set only by `reconcileAdditions`, never accepted from the LLM). The `importance` 1–10 range and update-requires-`target_id` rule stay enforced by `validateExtractionPayload` — do not encode them in the schema.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/memory/ -run TestExtractionSchemaStrictShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/memory/extractor_schema.go internal/memory/extractor_schema_test.go
git commit -m "feat(memory): strict JSON schema for extraction contract"
```

---

### Task 4: Extractor eligibility gate + strict parse (delete fence-stripping)

**Files:**
- Modify: `internal/memory/extractor.go` (struct L49–57; `NewExtractor` area; `ExtractTurn` L92+; `llmExtract` ~L190–230)
- Test: create `internal/memory/extractor_strict_test.go` (package `memory`)
- Test: modify `tests/integration/memory/extractor_json_contract_test.go` (stub adapter)

**Interfaces:**
- Consumes: `provider.ResponseFormat` (Task 2), `extractionSchema()` (Task 3).
- Produces: `(*Extractor).WithStructuredOutputsEligible(eligible bool) *Extractor` — Task 5 (session) calls this. Default (unset) is **true** so direct-construction tests stay meaningful; the session always sets it explicitly.

- [ ] **Step 1: Write the failing tests**

`internal/memory/extractor_strict_test.go`. Use a local stub adapter in package `memory`:

```go
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
```

Three tests (harness: reuse the repo/DB setup pattern from `tests/integration/memory` — in package `memory` use `store.NewMemoryNoteRepo` on a temp DB via the same helper style used by other `internal/memory` tests; if a temp-DB helper already exists in the package, reuse it):

```go
func TestExtractorIneligible_MakesNoLLMCall(t *testing.T) {
	adapter := &strictStubAdapter{response: `{"has_new_info":true,"confidence":0.9,"notes":[{"action":"add","target_id":"","category":"fact","content":"x","importance":5}]}`}
	ex := NewExtractor(noteRepo, adapter, nil).WithStructuredOutputsEligible(false)
	res, err := ex.ExtractTurn(context.Background(), profileID, slug, "", nil, "user", "assistant")
	if err != nil { t.Fatalf("ExtractTurn: %v", err) }
	if adapter.calls != 0 { t.Fatalf("LLM called %d times, want 0", adapter.calls) }
	if res == nil || len(res.Notes) != 0 { t.Fatalf("want empty result, got %+v", res) }
}

func TestExtractorSendsResponseFormat(t *testing.T) {
	adapter := &strictStubAdapter{response: `{"has_new_info":false,"confidence":0.9,"notes":[]}`}
	ex := NewExtractor(noteRepo, adapter, nil).WithStructuredOutputsEligible(true)
	_, _ = ex.ExtractTurn(context.Background(), profileID, slug, "", nil, "user", "assistant")
	if adapter.lastReq == nil || adapter.lastReq.ResponseFormat == nil {
		t.Fatal("expected ResponseFormat on the completion request")
	}
	rf := adapter.lastReq.ResponseFormat
	if rf.Name != "memory_extraction" || !rf.Strict || rf.Schema == nil {
		t.Fatalf("ResponseFormat = %+v, want name memory_extraction strict schema", rf)
	}
}

func TestExtractorRejectsFencedJSON(t *testing.T) {
	adapter := &strictStubAdapter{response: "```json\n{\"has_new_info\":true,\"confidence\":0.9,\"notes\":[]}\n```"}
	hook := &captureReasonsHook{}
	ex := NewExtractor(noteRepo, adapter, nil).WithStructuredOutputsEligible(true).WithLogHook(hook)
	res, err := ex.ExtractTurn(context.Background(), profileID, slug, "", nil, "user", "assistant")
	if err != nil { t.Fatalf("ExtractTurn: %v", err) }
	if len(res.Notes) != 0 { t.Fatal("fenced JSON must not be accepted") }
	if !hook.sawReason("invalid json") {
		t.Fatalf("expected 'invalid json' rejection, got %v", hook.reasons)
	}
}
```

(`captureReasonsHook` = local stub implementing `CaptureLogHook` recording `ExtractionPayloadRejected` reasons; if the package already has one, reuse it.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/memory/ -run 'TestExtractorIneligible|TestExtractorSends|TestExtractorRejects' -v`
Expected: FAIL — `undefined: WithStructuredOutputsEligible`

- [ ] **Step 3: Implement in `internal/memory/extractor.go`**

1. Add field `eligible bool` to `Extractor` and `NewExtractor` sets `eligible: true` (zero-value-safe default for direct constructors).
2. Add option (following the `WithDeduper` builder style):

```go
// WithStructuredOutputsEligible gates LLM extraction on whether the
// effective extractor model supports structured outputs. The session
// wiring sets this from the provider model list; false means refuse.
func (e *Extractor) WithStructuredOutputsEligible(eligible bool) *Extractor {
	e.eligible = eligible
	return e
}
```

3. In `ExtractTurn`, after the `e.adapter == nil` early return, add:

```go
if !e.eligible {
	return &ExtractionResult{}, nil
}
```

4. In `llmExtract`, add `ResponseFormat` to the request and delete the fence-stripping (remove the four lines `raw = strings.TrimPrefix(raw, "```json")` … `raw = strings.TrimSpace(raw)` after it; keep exactly one `strings.TrimSpace` for the value). The call becomes:

```go
resp, err := e.adapter.Complete(ctx, provider.CompletionRequest{
	Messages:    []provider.Message{{Role: "user", Content: prompt}},
	Temperature: 0.2,
	ResponseFormat: &provider.ResponseFormat{
		Name:   "memory_extraction",
		Strict: true,
		Schema: extractionSchema(),
	},
})
```

Parsing below stays: `raw := strings.TrimSpace(resp.Content)` → `json.Unmarshal` → `validateExtractionPayload`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/memory/ -v`
Expected: PASS (new tests + all existing memory tests — the deleted fence-stripping must not break any existing memory test; if an existing test fed fenced JSON, that test is asserting the old lenient contract and must be updated to plain JSON with a comment noting the schema enforcement).

- [ ] **Step 5: Update the integration contract test to pin the wire contract**

In `tests/integration/memory/extractor_json_contract_test.go`: convert `extractorStubAdapter` to pointer receivers with a `lastReq *provider.CompletionRequest` capture (update constructions from `extractorStubAdapter{content: ...}` to `&extractorStubAdapter{content: ...}`). In `TestExtractorJSONContract_ValidPerNoteActionsAccepted`, additionally assert `adapter.lastReq.ResponseFormat != nil && adapter.lastReq.ResponseFormat.Name == "memory_extraction" && adapter.lastReq.ResponseFormat.Strict`.

Run: `go test ./tests/integration/memory/ -run TestExtractorJSONContract -v`
Expected: PASS

- [ ] **Step 6: Full verification + commit**

Run: `make fmt lint test`
Commit:

```bash
git add internal/memory/extractor.go internal/memory/extractor_strict_test.go tests/integration/memory/extractor_json_contract_test.go
git commit -m "feat(memory): gate extraction on structured-outputs eligibility, enforce schema, drop lenient parse"
```

---

### Task 5: Session eligibility state

**Files:**
- Modify: `internal/chat/session.go` (struct fields near `extractorFallback`; method near `SetExtractorModel` L683; `extractAsync` L546–571)
- Test: `internal/chat/session_test.go` (existing file, add test)

**Interfaces:**
- Consumes: `(*memory.Extractor).WithStructuredOutputsEligible` (Task 4).
- Produces (Task 7 uses both):
  - `func (s *Session) SetExtractorEligible(eligible bool)`
  - `func (s *Session) ExtractorEligibleActive() bool`
  - New zero-value field `extractorEligible bool` on `Session` — **default false** (unknown = ineligible; fetch failure leaves it false). Guard with the existing `adapterMu` exactly like `extractorFallback`.

- [ ] **Step 1: Write the failing test**

Add to `internal/chat/session_test.go` (reuse the file's existing session-construction helper if present; otherwise construct a minimal `Session{}` value via `NewSession` with the stub-adapter pattern already used by `internal/chat` tests):

```go
func TestSession_ExtractorEligibilityDefaultsFalse(t *testing.T) {
	s := newTestSession(t) // existing helper in this file/package
	if s.ExtractorEligibleActive() {
		t.Fatal("default eligibility must be false (unknown = ineligible)")
	}
	s.SetExtractorEligible(true)
	if !s.ExtractorEligibleActive() {
		t.Fatal("SetExtractorEligible(true) not observed")
	}
	s.SetExtractorEligible(false)
	if s.ExtractorEligibleActive() {
		t.Fatal("SetExtractorEligible(false) not observed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/chat/ -run TestSession_ExtractorEligibilityDefaultsFalse -v`
Expected: FAIL — `undefined: SetExtractorEligible`

- [ ] **Step 3: Implement in `internal/chat/session.go`**

Add field `extractorEligible bool` next to `extractorFallback`. Add (mirroring `ExtractorFallbackActive`):

```go
// SetExtractorEligible records whether the effective extractor model
// supports structured outputs. Zero value (false) means ineligible:
// extraction is refused until a successful model-list fetch proves otherwise.
func (s *Session) SetExtractorEligible(eligible bool) {
	s.adapterMu.Lock()
	s.extractorEligible = eligible
	s.adapterMu.Unlock()
}

// ExtractorEligibleActive reports structured-outputs eligibility for the effective extractor model.
func (s *Session) ExtractorEligibleActive() bool {
	s.adapterMu.RLock()
	defer s.adapterMu.RUnlock()
	return s.extractorEligible
}
```

In `extractAsync`, apply the flag in **both** construction branches — after `extractor := s.extractor` initialization and the adapter selection block:

```go
extractor.WithStructuredOutputsEligible(s.ExtractorEligibleActive())
```

(placed with the other `extractor.With…` calls, after `extractor.WithLogHook(...)`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/chat/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chat/session.go internal/chat/session_test.go
git commit -m "feat(chat): track structured-outputs eligibility in session, gate extractAsync"
```

---

### Task 6: `/model extractor` validation via `ExecContext.ListModels`

**Files:**
- Modify: `internal/commands/registry.go` (`ExecContext` struct L52–90)
- Modify: `internal/commands/model_extractor_command.go` (handler L35–63)
- Test: create `internal/commands/model_extractor_command_test.go` (package `commands`)
- Modify: `internal/app/chat_cmd.go` (execCtx construction ~L90)

**Interfaces:**
- Consumes: `ModelInfo.SupportsStructuredOutputs` (Task 1).
- Produces: `ExecContext.ListModels func(ctx context.Context) ([]provider.ModelInfo, error)` — wired by Task 7's file owner (chat_cmd). Nil hook = refuse.

- [ ] **Step 1: Write the failing test**

`internal/commands/model_extractor_command_test.go`. Use a temp DB + profile the same way other `commands` package tests do (if none exist, create profile via `store` + `store.NewProviderConfigRepo` with an active provider_config row so `SetExtractorModel`'s `RowsAffected > 0` path works; model the setup on `model_extractor_command.go`'s own DB usage):

```go
func TestModelExtractorHandler_RejectsIneligibleModels(t *testing.T) {
	// eligible: true model, eligible: false model, missing model ID
	// hook returns []provider.ModelInfo{
	//   {ID: "good", SupportsStructuredOutputs: true},
	//   {ID: "bad"},
	// }
	// handler with args ["good"] → nil error, extractor_model persisted as "good"
	// handler with args ["bad"] → error containing "does not support structured outputs"
	// handler with args ["missing"] → error containing "not found"
	// hook returning error → handler error containing "could not verify"
	// nil hook → handler error containing "model list unavailable"
	// no args → *ErrOpenExtractorModelPicker
}
```

Write the seven cases as subtests with those exact error substrings; assert the DB value via `store.NewProviderConfigRepo(db).GetProviderConfig`-style read (use whatever getter exists).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/commands/ -run TestModelExtractorHandler -v`
Expected: FAIL — `ctx.ListModels undefined`

- [ ] **Step 3: Implement**

1. `registry.go` — add to `ExecContext`:

```go
// ListModels fetches the provider's model list, if available in this
// execution context. Commands use it to validate model capabilities.
ListModels func(ctx context.Context) ([]provider.ModelInfo, error)
```

(add the `noto/internal/provider` import).

2. `model_extractor_command.go` — after the existing profile guard and before `model := args[0]` usage, validate:

```go
if len(args) == 0 {
	return &ErrOpenExtractorModelPicker{} // unchanged
}
model := args[0]
if ctx.ListModels == nil {
	return errors.New("model list unavailable — cannot verify structured outputs support")
}
models, err := ctx.ListModels(context.Background())
if err != nil {
	return fmt.Errorf("could not verify model support: %w", err)
}
found := false
for _, mi := range models {
	if mi.ID == model {
		found = true
		if !mi.SupportsStructuredOutputs {
			return fmt.Errorf("model %s does not support structured outputs — extraction requires it", model)
		}
		break
	}
}
if !found {
	return fmt.Errorf("model %s not found in provider model list", model)
}
```

3. `internal/app/chat_cmd.go` — in `runChat`, where `adapterCfg` is defined (inside the `providerCfg != nil` branch), add after `listModelsFn` is created:

```go
execCtx.ListModels = func(callCtx context.Context) ([]provider.ModelInfo, error) {
	return provider.ListModels(callCtx, adapterCfg)
}
```

Leave it nil when no provider is configured.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/commands/ ./internal/app/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/commands/registry.go internal/commands/model_extractor_command.go internal/commands/model_extractor_command_test.go internal/app/chat_cmd.go
git commit -m "feat(commands): validate structured-outputs support in /model extractor"
```

---

### Task 7: App wiring — eligibility resolution, session updates, TUI messages

**Files:**
- Modify: `internal/app/chat_cmd.go` (`resolveSessionModelMetadata` L560–585; `applySessionModelMetadata` L587–598; `runChat` startup ~L194 and post-`tui.New` block; `modelSelectedFn` ~L213; `extractorModelSelectedFn` ~L237; the same four spots inside `profileSwitchCmd`'s closure)
- Test: `internal/app/chat_model_metadata_test.go` (existing file, add test)

**Interfaces:**
- Consumes: `ModelInfo.SupportsStructuredOutputs` (Task 1), `Session.SetExtractorEligible` (Task 5).
- Produces:
  - `func resolveExtractorEligibility(models []provider.ModelInfo, activeModel, extractorModel string, extractorFallback bool) bool`
  - `applySessionModelMetadata` changes return signature: `func applySessionModelMetadata(sess *chatpkg.Session, models []provider.ModelInfo, activeModel, extractorModel string, extractorFallback bool) (extractorEligible bool)` — calls `sess.SetExtractorEligible(extractorEligible)` and returns the value.
  - TUI **types only** (definitions here; behavior in Task 8): `Model.extractorIneligible bool` field, `func ExtractorEligibilityUpdated(eligible bool) tea.Msg`, internal `type extractorEligibilityUpdatedMsg struct{ eligible bool }`, `func (m *Model) SetExtractorIneligible(ineligible bool)`.

- [ ] **Step 1: Write the failing test**

Add to `internal/app/chat_model_metadata_test.go` (mirroring `TestResolveSessionModelMetadata`):

```go
func TestResolveExtractorEligibility(t *testing.T) {
	models := []provider.ModelInfo{
		{ID: "chat-ok", SupportsStructuredOutputs: true},
		{ID: "chat-bad"},
		{ID: "ex-ok", SupportsStructuredOutputs: true},
		{ID: "ex-bad"},
	}
	cases := []struct {
		name                             string
		active, extractor                string
		fallback                         bool
		want                             bool
	}{
		{"explicit eligible extractor", "chat-ok", "ex-ok", false, true},
		{"explicit ineligible extractor", "chat-ok", "ex-bad", false, false},
		{"fallback eligible chat model", "chat-ok", "chat-ok", true, true},
		{"fallback ineligible chat model", "chat-bad", "chat-bad", true, false},
		{"unknown model id", "chat-ok", "missing", false, false},
		{"empty model list", "chat-ok", "ex-ok", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list := models
			if tc.name == "empty model list" {
				list = nil
			}
			if got := resolveExtractorEligibility(list, tc.active, tc.extractor, tc.fallback); got != tc.want {
				t.Fatalf("resolveExtractorEligibility = %t, want %t", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestResolveExtractorEligibility -v`
Expected: FAIL — `undefined: resolveExtractorEligibility`

- [ ] **Step 3: Implement resolution + session update**

In `internal/app/chat_cmd.go`:

```go
func resolveExtractorEligibility(models []provider.ModelInfo, activeModel, extractorModel string, extractorFallback bool) bool {
	effective := extractorModel
	if extractorFallback {
		effective = activeModel
	}
	if effective == "" {
		return false
	}
	for _, model := range models {
		if model.ID == effective {
			return model.SupportsStructuredOutputs
		}
	}
	return false
}
```

Change `applySessionModelMetadata` to compute `eligible := resolveExtractorEligibility(models, activeModel, extractorModel, extractorFallback)`, call `sess.SetExtractorEligible(eligible)`, and return `(extractorEligible bool)` (existing body otherwise unchanged).

Then add the TUI type definitions (definitions only — no Update handler, footer, or picker changes here; Task 8 implements the behavior) in `internal/tui/model.go`:

```go
// field on Model, next to extractorFallback:
extractorIneligible bool

// constructor near ProfileSwitched:
// ExtractorEligibilityUpdated reports the effective extractor model's
// structured-outputs eligibility after a model-list fetch or selection.
func ExtractorEligibilityUpdated(eligible bool) tea.Msg {
	return extractorEligibilityUpdatedMsg{eligible: eligible}
}

// near profileSwitchedMsg:
type extractorEligibilityUpdatedMsg struct{ eligible bool }

// setter mirroring SetStartupConversationHistory's receiver style:
// SetExtractorIneligible sets the initial footer warning state for the
// effective extractor model.
func (m *Model) SetExtractorIneligible(ineligible bool) {
	m.extractorIneligible = ineligible
}
```

- [ ] **Step 4: Wire the callers**

In `internal/app/chat_cmd.go` — **both** the `runChat` top-level wiring and the identical blocks inside `profileSwitchCmd`'s closure:

1. Every `applySessionModelMetadata(sess, models, ...)` call site becomes `eligible := applySessionModelMetadata(...)` followed by:

```go
if prog != nil {
	prog.Send(tui.ExtractorEligibilityUpdated(eligible))
}
```

2. `runChat` startup: initialize `extractorIneligible := true` before the provider branch; after the startup `applySessionModelMetadata` call set `extractorIneligible = !eligible`. After `m := tui.New(...)` (next to `m.SetStartupConversationHistory(...)`), add `m.SetExtractorIneligible(extractorIneligible)`.

3. `modelSelectedFn` (both copies): after the `applySessionModelMetadata` call, send the message (the eligible value it returns — covers the fallback-follows-chat-model case).
4. `extractorModelSelectedFn` (both copies): same, after its `applySessionModelMetadata` call.
5. `profileSwitchCmd` closure: after its `applySessionModelMetadata` call, `prog.Send(tui.ExtractorEligibilityUpdated(eligible))` before returning `tui.ProfileSwitched(...)`.
6. Fetch-failure paths need no code: eligibility stays default-false (session) and `extractorIneligible` stays true (`runChat`) — exactly the "paused until fetch succeeds" behavior.

- [ ] **Step 5: Run tests + verify compile**

Run: `go build ./... && go test ./internal/app/ ./internal/chat/ -v`
Expected: PASS, clean compile.

- [ ] **Step 6: Commit**

```bash
git add internal/app/chat_cmd.go internal/app/chat_model_metadata_test.go internal/tui/model.go
git commit -m "feat(app): resolve extractor structured-outputs eligibility, drive session and TUI"
```

---

### Task 8: TUI — filtered picker, footer warning, eligibility message

**Files:**
- Modify: `internal/tui/model.go` (Model field ~L304; `ProfileSwitched` area L79 for the new constructor; msg type near `profileSwitchedMsg` L165; Update `profileSwitchedMsg` case ~L1055)
- Modify: `internal/tui/footer_view.go` (`renderFooter` ~L51)
- Modify: `internal/tui/picker_view.go` (`openPicker` `pickerKindExtractorModel` case L60–75)
- Test: create `tests/integration/tui/extractor_eligibility_test.go`

**Interfaces:**
- Consumes: `ModelInfo.SupportsStructuredOutputs` (Task 1, via `ListModelsFunc` results); the TUI types Task 7 defined (`extractorEligibilityUpdatedMsg`, `SetExtractorIneligible`, `Model.extractorIneligible`).
- Produces: the **behavior** for those types — the `Update` handler for `extractorEligibilityUpdatedMsg`, the footer warning, and the filtered picker.

- [ ] **Step 1: Write the failing tests**

`tests/integration/tui/extractor_eligibility_test.go` (use the existing `newTestModel` helper from `tests/integration/tui/model_helpers_test.go`; its default `listModels` stub is nil — pass one returning eligible + ineligible models):

```go
func TestExtractorPicker_ListsOnlyStructuredOutputsModels(t *testing.T) {
	m := newTestModel(func(context.Context) ([]provider.ModelInfo, error) {
		return []provider.ModelInfo{
			{ID: "good-model", SupportsStructuredOutputs: true},
			{ID: "bad-model"},
			{ID: "also-bad"},
		}, nil
	})
	// Open the extractor picker exactly the way the existing
	// embeddings-picker test opens its picker
	// (tests/integration/tui/embed_model_picker_test.go) — same settings-menu
	// entry path, but for the extractor-model entry.
	// Then run the returned tea.Cmd and assert the modelsLoadedMsg items
	// are exactly ["good-model"].
}

func TestExtractorPicker_NoEligibleModels_ShowsError(t *testing.T) {
	m := newTestModel(func(context.Context) ([]provider.ModelInfo, error) {
		return []provider.ModelInfo{{ID: "bad-model"}}, nil
	})
	// Open the picker as above; run the tea.Cmd; assert the picker state shows
	// the error "no models support structured outputs on this endpoint"
	// and the item list is empty.
}

func TestFooter_ExtractorIneligibleWarning(t *testing.T) {
	m := newTestModel(nil)
	m.SetExtractorIneligible(true)
	footer := m.View() // assert via the same render path TestFooterIncludesVersion uses
	if !strings.Contains(footer, "Extractor model lacks structured outputs") {
		t.Fatal("footer missing extractor warning")
	}
	m.SetExtractorIneligible(false)
	if strings.Contains(m.View(), "Extractor model lacks structured outputs") {
		t.Fatal("footer must not show warning when eligible")
	}
}

func TestExtractorEligibilityUpdatedMsg_TogglesFooter(t *testing.T) {
	m := newTestModel(nil)
	updated, _ := m.Update(tui.ExtractorEligibilityUpdated(false))
	if !strings.Contains(updated.(tui.Model).View(), "Extractor model lacks structured outputs") {
		t.Fatal("expected ineligible warning after ExtractorEligibilityUpdated(false)")
	}
	updated, _ = updated.Update(tui.ExtractorEligibilityUpdated(true))
	if strings.Contains(updated.(tui.Model).View(), "Extractor model lacks structured outputs") {
		t.Fatal("expected warning gone after ExtractorEligibilityUpdated(true)")
	}
}
```

All four assertions go through observable behavior (picker items/error, footer text) — no new getters.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests/integration/tui/ -run 'TestExtractorPicker|TestFooter_Extractor|TestExtractorEligibility' -v`
Expected: FAIL — the types compile (Task 7 defined them), but all four tests fail: the `Update` case ignores the msg (footer never toggles), the footer has no warning line, and the picker lists ineligible models.

- [ ] **Step 3: Implement**

1. `internal/tui/model.go`: `Update` handler for `extractorEligibilityUpdatedMsg` (near the `profileSwitchedMsg` case, L1055): `m.extractorIneligible = !msg.eligible`. The field, msg type, constructor, and setter already exist from Task 7 — do not re-create them.

2. `internal/tui/footer_view.go` in `renderFooter`, after the `m.extractorFallback` block (L51):

```go
if m.extractorIneligible {
	leftParts = append(leftParts, yellow.Render("Extractor model lacks structured outputs — extraction paused."))
}
```

3. `internal/tui/picker_view.go` in `openPicker`, `pickerKindExtractorModel` case: filter before building items:

```go
eligible := make([]provider.ModelInfo, 0, len(models))
for _, mi := range models {
	if mi.SupportsStructuredOutputs {
		eligible = append(eligible, mi)
	}
}
if len(eligible) == 0 {
	m.picker.loading = false
	m.picker.err = errors.New("no models support structured outputs on this endpoint")
	return // do not set items
}
```

(build `items` from `eligible` instead of `models`; keep the `Active` marker behavior).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests/integration/tui/ ./internal/tui/ -v`
Expected: PASS. Then: `go build ./...` — confirms Task 7's references resolve.

- [ ] **Step 5: Full verification + commit**

Run: `make fmt lint test`
Expected: all green. If an existing TUI test that opens the extractor picker breaks because its stub models lack `SupportsStructuredOutputs`, add `SupportsStructuredOutputs: true` to that stub's models — that is the new contract, not a workaround.

Commit:

```bash
git add internal/tui/model.go internal/tui/footer_view.go internal/tui/picker_view.go tests/integration/tui/extractor_eligibility_test.go
git commit -m "feat(tui): structured-outputs-only extractor picker with eligibility warning"
```

---

## Final Verification (whole branch)

- [ ] Run `make fmt lint test` — everything green.
- [ ] Manual smoke (if an OpenRouter key is configured): `make run`, open the extractor picker, confirm only structured-outputs models are listed, select one, send a chat turn, confirm notes still extract; set an ineligible model via `/model extractor` and confirm the command refuses with a clear error.
- [ ] Then: commit the design doc and this plan into the repo (`docs/superpowers/specs/2026-10-24-extractor-structured-outputs-design.md`, `docs/superpowers/plans/2026-10-24-extractor-structured-outputs.md`) — they currently live only in `/home/andy/.opencode/plan`.
