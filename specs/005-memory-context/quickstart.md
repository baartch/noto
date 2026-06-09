# Quickstart: Validate Timeline Memory, Tool Calling, and Footer Telemetry

This quickstart validates the broadened memory feature, including timeline-based context assembly, rollup generation, OpenRouter tool calling, cache correctness, and footer context-capacity telemetry.

## Prerequisites

- Active profile with notes spanning multiple months.
- Provider configuration that can use an OpenRouter-compatible endpoint.
- At least one active chat model with model metadata available from the provider models API.
- Embeddings model configured for vector-backed memory retrieval.
- Diagnostics/log output available for retrieval and provider behavior.

## Validation Scenarios

### 1. Configurable Timeline Context

1. Set timeline settings to defaults:
   - raw-note days = 30
   - weekly-summary weeks = 8
   - monthly-summary months = all remaining
2. Start a chat turn with profile history spanning more than three months.
3. Verify the assembled context contains:
   - raw recent notes for the configured recent rolling-day window,
   - weekly summaries for the configured mid-range week window,
   - monthly summaries for the configured older-history month window.
4. Change one or more timeline settings and repeat.
5. Verify the assembled context reflects the new configured windows on the next retrieval.

### 2. Weekly and Monthly Rollup Generation

1. Prepare notes covering at least one completed week and one completed month that lack summaries.
2. Open the profile or process a chat turn after the boundary has passed.
3. Verify missing weekly/monthly summaries are generated automatically.
4. Edit a note from a summarized period.
5. Verify the affected summary is marked stale and becomes eligible for regeneration.

### 3. Keyword Search Tool via OpenRouter Tool Calling

1. Use a model that supports the `tools` parameter.
2. Ask a question that should trigger memory search by topic.
3. Verify the provider request includes tool definitions.
4. Verify a `search_memory_keywords` tool call can be executed locally and returned to the provider.
5. Verify the final assistant answer incorporates tool results without failing the conversation.

### 4. Time-Range Search Tool via OpenRouter Tool Calling

1. Ask for information tied to a specific date or range.
2. Verify a `search_memory_time_range` tool call can be executed locally.
3. Verify returned records are all within the requested range.
4. Verify both raw notes and summary records can appear when appropriate.

### 5. Cache and Settings Invalidation

1. Warm the context cache for a profile.
2. Change one retrieval-shaping input at a time:
   - prompt
   - notes
   - summary state
   - embedding model
   - timeline settings
3. Verify cache reuse is rejected or marked stale when identity no longer matches.
4. Verify slightly stale entries still use stale-while-revalidate behavior.

### 6. Internal Token-Fitting Safeguard

1. Prepare a profile whose assembled timeline context is large enough to exceed the internal prompt-fitting limit.
2. Trigger context assembly with monthly summaries available for older history.
3. Verify the system preserves newer layers first and reduces context by dropping the oldest monthly-summary coverage before touching newer timeline layers.
4. Verify the conversation still proceeds without exposing any user-facing token-budget setting.

### 7. Footer Context-Capacity Telemetry

1. Open chat with a model whose metadata includes `context_length`.
2. Send one or more messages.
3. Verify the footer left side shows token telemetry plus current context usage percentage and max context size.
4. Switch to another model with a different max context size.
5. Verify the footer updates to the new max size.
6. Verify the footer shows an explicit unknown-capacity state if model metadata is unavailable.

## Expected Results

- Default memory context follows configured timeline windows.
- Weekly/monthly summaries are created automatically and reused across requests.
- Tool-capable models receive OpenRouter-compatible search tools and can use them successfully.
- Time-range search returns only in-range records.
- Cache identity reflects settings and summary-state changes.
- Footer shows context-capacity telemetry next to tokens when metadata is known, and an honest fallback when it is not.

## Validation Commands

- `make fmt`
- `make lint`
- `make test`

## Related Documents

- [Data Model](./data-model.md)
- [Timeline Context Retrieval Contract](./contracts/context-retrieval.md)
- [Memory Search Tools Contract](./contracts/memory-search-tools.md)
- [Footer Telemetry Contract](./contracts/footer-telemetry.md)
