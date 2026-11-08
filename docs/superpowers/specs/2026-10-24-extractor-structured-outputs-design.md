# Design: Structured Outputs for the Memory Extractor

Date: 2026-10-24
Status: approved in conversation; pending builder check
Final home (once out of plan mode): `docs/superpowers/specs/2026-10-24-extractor-structured-outputs-design.md`

## Intent and Why

The memory extractor (`internal/memory/extractor.go`) asks the LLM to return a
JSON payload describing extracted notes, then parses the reply by stripping
markdown code fences and leniently running `json.Unmarshal`. Whenever the model
wraps the JSON in prose or fences the parse fails, and the extraction is
silently dropped. OpenRouter (the primary provider) supports **structured
outputs**: passing `response_format: json_schema` with a strict schema makes
the provider enforce the exact JSON shape.

This change makes the extraction contract enforceable at the API level and
refuses to run extraction at all with any model that cannot honor it.

## Goals

1. The extractor LLM call uses `response_format` (json_schema, `strict: true`)
   with a schema derived from the existing `extractionResponse` /
   `extractedItem` structs.
2. A model is **extractor-eligible** only if the provider's `/models` endpoint
   reports `structured_outputs` in its `supported_parameters` list.
   - Known-unsupported → ineligible.
   - Unknown (endpoint does not report `supported_parameters`, e.g. vanilla
     OpenAI, LM Studio, Ollama) → ineligible. **No exceptions, no fallback.**
3. The eligibility rule is enforced at three points:
   - **TUI picker** (`pickerKindExtractorModel`, `internal/tui/picker_view.go`):
     lists only eligible models. The main chat model picker is unchanged.
   - **`/model extractor` command** (`internal/commands/model_extractor_command.go`):
     validates the requested model against the fetched model list and rejects
     ineligible IDs with a clear error.
   - **Extraction time** (`Extractor.ExtractTurn`): if the *effective*
     extractor model (configured extractor model, else the chat model under
     the kept fallback) is ineligible, return early with no LLM call. No
     prompt-and-pray fallback exists anywhere.
4. When the configured extractor model is ineligible, the user sees a one-line
   warning at TUI startup / profile switch, following the existing
   "embedding model missing" pattern (`extractorModelMissing` flag,
   `ProfileSwitched` message). Not a per-turn error. Extraction stays disabled
   until the user picks an eligible model.
5. On OpenRouter endpoints the extraction request also sets
   `require_parameters: true` in the provider routing preferences so OpenRouter
   routes only to endpoints that honor `response_format`. Generic
   OpenAI-compatible endpoints never receive this field (do not send unknown
   fields to strict servers).
6. The fence-stripping / lenient-parse path in `llmExtract` is **deleted**, not
   kept behind a flag.
7. `validateExtractionPayload` remains as defense-in-depth after parsing.
8. Runtime parse failures of an otherwise-eligible model still go through the
   existing capture-log hook (`ExtractionPayloadRejected`) and never break the
   chat flow.

## Non-Goals

- No change to the main chat model picker or chat completion requests.
- No change to embedding model selection or the reference selector.
- No migration of existing configured extractor models — they simply become
  ineligible and trigger the startup warning until replaced.
- No support for non-OpenAI-compatible providers.
- No user-facing schema editor; the schema lives in code.

## Anti-Goals

- **Any lenient fallback.** If the model cannot do structured outputs, we
  refuse. Never silently revert to fence-stripping.
- **Silent ineligibility.** A refused model must be visible (picker absence,
  command error, or startup warning), never a silent no-op the user has to
  discover in capture logs.

## Constraints

- Go 1.26+, existing patterns: Cobra commands, Bubble Tea v2 TUI,
  `provider.Adapter` interface, profile-local SQLite.
- `CompletionRequest` (in `internal/provider`) is shared across callers; the
  new `response_format` support must be optional and absent from chat
  requests.
- The adapter (`internal/provider/openai_compatible.go`) is generic
  OpenAI-compatible; OpenRouter-only fields must be gated on detecting an
  OpenRouter endpoint.
- Per OpenRouter docs: support is determined **per endpoint**, not per model —
  hence the `require_parameters: true` routing preference on OpenRouter.

## Decisions (as decided in conversation)

| Decision | Choice |
|---|---|
| Fallback when model lacks structured outputs | Refuse. No lenient-parse fallback. |
| Endpoint doesn't report `supported_parameters` | Refuse too (unknown = ineligible) |
| Unconfigured extractor model (chat-model fallback) | **Kept.** Extraction still runs on the main chat model; the eligibility rule gates the *effective* extractor model (configured extractor model, else chat model). Eligibility and the warning follow chat-model switches while the fallback is active. |
| `/models` fetch failure | Same treatment as unknown: effective model ineligible + one-line warning; extraction paused until a fetch succeeds |
| Warning surface | One-line startup/profile-switch warning, like embeddings-missing |
| Lenient parse path | Delete outright |
| Schema source | Generated in code from `extractionResponse`/`extractedItem`, descriptions carried from the current prompt template; no duplicated schema file |
| `require_parameters` routing | Send only on OpenRouter endpoints |

## Implementation Sketch (what the builder builds)

1. **`internal/provider/models.go`**: add
   `ModelInfo.SupportsStructuredOutputs`; set it from
   `slices.Contains(m.SupportedParameters, "structured_outputs")` in the
   existing parse loop (next to the `tools` check).
2. **`internal/provider`**: extend `CompletionRequest` with an optional
   `ResponseFormat` value (json_schema: name, strict flag, schema). The
   OpenAI-compatible adapter marshals it into the request payload as
   `response_format` when set. When the resolved endpoint host is
   `openrouter.ai`, also set the OpenRouter provider preference
   `require_parameters: true`.
3. **`internal/memory/extractor.go`**:
   - Build the JSON schema from the existing structs (add schema tags /
     descriptions matching `DefaultExtractorPromptTemplate` semantics).
   - `llmExtract` sends `ResponseFormat`, parses the reply with
     `json.Unmarshal` only; delete fence-stripping.
   - `ExtractTurn` performs the eligibility check before any LLM call and
     returns early when ineligible.
4. **`internal/commands/model_extractor_command.go`**: handler fetches the
   model list and rejects ineligible models with a clear error message.
5. **`internal/chat` + TUI wiring**:
   - Thread `SupportsStructuredOutputs` through `ListModelsFunc` results (it is
     already `provider.ModelInfo`).
   - `profileSwitchedMsg` gains an `extractorIneligible` (or similar) flag,
     set by checking the configured extractor model against the current model
     list; TUI renders the one-line warning.
   - `pickerKindExtractorModel` filters items to
     `SupportsStructuredOutputs == true`.
6. **Tests**:
   - Unit: models parse (`structured_outputs` present/absent/absent field);
     adapter payload includes `response_format` and OpenRouter-only
     `require_parameters` only for OpenRouter endpoints.
   - Integration: update `tests/integration/memory/extractor_json_contract_test.go`
     stub adapter for the new request shape; add a refusal test
     (ineligible model → no LLM call, empty result, warning surfaced).

## Builder-Check Settlements (my calls, not the partner's)

- **Eligibility mechanism (was open):** the Extractor cannot fetch `/models`
  itself. Eligibility is injected: the session/chat wiring (which already
  calls `ListModels` at profile switch and model selection) computes it and
  passes it into the Extractor via a new builder option
  (`WithStructuredOutputsEligible(bool)`), threaded through the Session.
  Re-evaluated on: startup, profile switch, model selection. Note:
  `cmd/dedup-notes/main.go` builds its own provider adapter for a separate
  dedup-classification flow — it does **not** use `memory.Extractor` and is
  out of scope for this design.
- **`/model extractor` validation layer (was open):** `ExecContext` gains a
  `ListModels func(context.Context) ([]provider.ModelInfo, error)` hook
  (wired in `internal/app` where credentials live). The command validates
  through it; no decryption logic enters `commands`.
- **Schema subset (was open):** strict mode requires all properties
  `required` + `additionalProperties: false`. `target_id` is always present,
  possibly empty (its "required when action=update" rule stays enforced by
  `validateExtractionPayload`). `enum` is used for `action` and `category`.
  `importance` 1–10 range stays in `validateExtractionPayload` (JSON Schema
  numeric ranges are allowed but the validator is the cleaner guard).
  `mergeIDs` is excluded — it is unexported and set by `reconcileAdditions`,
  never accepted from the LLM (already true today).
- **Custom prompt templates (was open):** per-profile `prompts/*.md`
  overrides remain honored as *prompt prose only*. The JSON contract is owned
  by the code-generated schema; a custom template cannot change the output
  shape, and its JSON-shape instructions become advisory text the schema
  overrules.
- **OpenRouter detection:** simple host-suffix check (`openrouter.ai`) on the
  configured endpoint.

## Resolved Questions (partner's answers, from builder check)

1. **Unconfigured extractor model:** the chat-model fallback survives
   (option a). The eligibility rule gates the *effective* extractor model —
   the configured extractor model if set, otherwise the main chat model.
   While the fallback is active, eligibility is re-evaluated on chat-model
   switches (`modelSelected` wiring) and the one-line warning appears/disappears
   with it.
2. **`/models` fetch failure:** same treatment as an endpoint that doesn't
   report parameters — effective model ineligible + one-line warning;
   extraction stays paused until a fetch succeeds. No silent "can't
   determine" pass.

## Left to the Builder

- Exact struct/type shape for `ResponseFormat` in `CompletionRequest`.
- Where the eligibility check for the configured extractor model lives at
  startup (session construction vs. TUI profile-switch path) — follow the
  existing `extractorModelMissing` pattern for consistency.
- Details of the startup warning text.
