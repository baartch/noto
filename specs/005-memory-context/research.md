# Research: Memory Context Timeline & Tooling

## Decision 1: Use configurable timeline windows for default context composition

**Decision**: Build default context from three ordered layers: recent raw notes, then weekly summaries, then monthly summaries. Make each layer window configurable in settings with defaults of 1 month raw, 2 months weekly, and all remaining months monthly.

**Rationale**: This preserves detailed recent memory while compacting older history in a predictable way. It also aligns with the clarified spec and keeps the design close to the current memory-first behavior instead of replacing it with a radically new retrieval model.

**Alternatives considered**:
- Fixed 1/2/all windows hard-coded in retrieval: rejected because the spec now requires settings-driven adjustment.
- Pure semantic retrieval only: rejected because it loses the explicit time-layered behavior now required by the spec.
- Monthly summaries only for all history: rejected because it removes valuable mid-term detail the weekly layer is meant to preserve.

## Decision 2: Treat rollups as durable first-class memory artifacts with regeneration eligibility

**Decision**: Store weekly and monthly summaries as persistent profile-local artifacts in SQLite, attach period identity and freshness/version metadata, and mark them for regeneration when underlying notes for their covered time range change.

**Rationale**: Regenerating rollups on every request would waste latency and cost. Durable summary artifacts keep retrieval fast and make cache identity more stable while still allowing correctness through stale marking and regeneration.

**Alternatives considered**:
- Generate summaries on demand each retrieval: rejected due to latency and repeated cost.
- Never regenerate after creation: rejected due to correctness drift when notes change.
- Store summaries only in the vector file: rejected because time-range queries and freshness tracking need structured profile-local storage.

## Decision 3: Generate missed weekly/monthly rollups opportunistically on profile processing

**Decision**: When a profile is opened or a chat turn is processed, check for completed week/month periods that are missing required summaries and generate them opportunistically. If a summary exists but is stale, queue regeneration before trusting it as current for future context.

**Rationale**: The app may be inactive during boundary transitions. Catch-up generation on the next profile-processing opportunity satisfies the spec without introducing a scheduler or background daemon.

**Alternatives considered**:
- Require the app to be running exactly at the time boundary: rejected as unreliable.
- Add an external cron-like scheduler: rejected as too invasive for a local CLI/TUI application.
- Ignore missed periods until a manual rebuild: rejected because the feature must be automatic.

## Decision 4: Expose memory search through OpenRouter tool-calling semantics in the provider loop

**Decision**: Extend the OpenAI-compatible provider request/response path to support tool definitions and tool call/result turns using OpenRouter’s documented `tools` and `tool_calls` message flow. Provide two tools to the LLM: one keyword/vector search tool and one time-range DB search tool.

**Rationale**: OpenRouter standardizes tool-calling semantics and already works through an OpenAI-compatible surface. Adding tool support within the existing provider adapter keeps the design close to current architecture while enabling the LLM to request targeted memory on demand.

**Alternatives considered**:
- Inject search results directly into every prompt without tool calling: rejected because it increases prompt size and removes on-demand retrieval behavior.
- Add provider-specific branching only for OpenRouter outside the adapter: rejected because it would spread provider logic into chat/session layers.
- Use slash commands instead of tool calling: rejected because the spec explicitly requires search tools for the LLM, not end-user manual commands.

## Decision 5: Implement keyword search as vector-backed retrieval with deterministic fallback

**Decision**: Back the keyword search tool with the existing vector index when embeddings are available, but keep deterministic fallback behavior for cases where vector lookup is temporarily unavailable.

**Rationale**: The project already maintains a vector index, so reusing it minimizes architectural change. A fallback path preserves graceful degradation required by the spec.

**Alternatives considered**:
- Pure SQL LIKE keyword search only: rejected because it would underuse the existing semantic index and likely reduce relevance.
- Vector search with no fallback: rejected because memory tool availability would become brittle when embeddings are unavailable.

## Decision 6: Implement time-range search as structured SQLite retrieval over notes and summaries

**Decision**: Implement the time-range search tool against SQLite using requested start/end boundaries and return matching raw notes plus relevant weekly/monthly summary records that fall inside the requested range.

**Rationale**: Time-bounded retrieval is naturally expressed in the structured store. SQLite already holds the canonical memory state and is the right place to enforce date-range filtering.

**Alternatives considered**:
- Reconstruct time-range results from vector search metadata only: rejected because exact temporal filtering belongs in structured storage.
- Return raw notes only: rejected because the spec requires the ability to return summary records too.

## Decision 7: Read model max context from OpenRouter model metadata using `context_length`

**Decision**: Extend provider model metadata parsing to capture `context_length` from the OpenRouter Models API response and use it as the active model’s context-window maximum when available.

**Rationale**: OpenRouter documents `context_length` as the standardized max context size in the model object schema. The current provider layer already has a models endpoint and a `ContextMax` field in completion stats, so this is a close-fit extension.

**Alternatives considered**:
- Maintain a local hard-coded map only: rejected because it drifts and does not satisfy the explicit OpenRouter metadata requirement.
- Use `top_provider.context_length` only: rejected because the user explicitly pointed to the model object’s `context_length` field and the top-provider field is provider-specific.

## Decision 8: Surface footer context telemetry as usage percentage plus max context near token stats

**Decision**: Continue using the existing footer token status area on the left and extend it to show the current request’s context usage percentage and max context size next to token telemetry. When max context is unknown, keep the token display and show an unknown-capacity placeholder instead of misleading percentages.

**Rationale**: The footer already displays token/cost information, and the spec calls for the new information beside those values. This preserves UX consistency while making context pressure visible during chat.

**Alternatives considered**:
- Add a separate footer badge far from token stats: rejected because the user asked for it next to tokens.
- Hide the value when unknown: rejected because a clear unknown state is more transparent.
- Show only max context without percentage: rejected because the spec asks for the currently used percentage.

## Decision 9: Expand cache identity to include timeline settings and summary-derived state

**Decision**: Extend cache identity beyond prompt/note/token/embedding inputs to also reflect the active timeline-window settings and the selected summary state that shapes the assembled context.

**Rationale**: Changing configured windows or summary content can change the composed context even when raw notes and prompt are otherwise unchanged. Identity must cover all retrieval-shaping inputs to prevent incorrect reuse.

**Alternatives considered**:
- Invalidate on settings change without identity expansion: partially acceptable but weaker, because identity should still express all request-shaping inputs.
- Ignore summary state in cache identity: rejected because regenerated summaries can materially change context.

## Decision 10: Preserve the current architecture by extending existing modules instead of adding a new orchestration layer

**Decision**: Keep chat/session orchestration in `internal/chat`, provider transport in `internal/provider`, retrieval/rollups in `internal/memory`, and footer rendering in `internal/tui`, with only targeted new helpers/files where necessary.

**Rationale**: The user requested staying close to the design principles. Extending current modules keeps conceptual load low and avoids creating a second orchestration path for memory/tooling.

**Alternatives considered**:
- Introduce a standalone agent/runtime subsystem for tools: rejected as too heavy for the scope.
- Move tool execution into the TUI layer: rejected because tool execution belongs near chat/provider orchestration, not presentation.
