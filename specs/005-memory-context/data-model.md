# Data Model: Memory Context Timeline & Tooling

## Entities

### TimelineSettings

Represents the profile-local settings that shape default memory context composition.

- **profile_id**: string
- **raw_note_months**: integer
- **weekly_summary_months**: integer
- **monthly_summary_months**: integer or sentinel for `all_remaining`
- **updated_at**: timestamp

Validation:
- `raw_note_months >= 0`
- `weekly_summary_months >= 0`
- `monthly_summary_months` is either `all_remaining` or an integer `>= 0`
- Setting changes take effect on the next context assembly.

### MemoryNote

Represents a durable note captured from conversations.

- **id**: string
- **profile_id**: string
- **category**: `fact | progress | blocker | action_item | other`
- **content**: string
- **importance**: integer
- **created_at**: timestamp
- **updated_at**: timestamp (nullable)

Validation:
- Category must be one allowed value.
- Notes remain the source material for rollups and search.

### WeeklySummary

Represents one completed calendar week of profile memory.

- **id**: string
- **profile_id**: string
- **week_start**: date/time
- **week_end**: date/time
- **content**: string
- **source_state_version**: string
- **freshness_state**: `fresh | stale | regenerating`
- **created_at**: timestamp
- **updated_at**: timestamp

Validation:
- Only one weekly summary may exist for a given profile and completed calendar week.
- `week_start < week_end`.
- `source_state_version` changes when the underlying covered notes change.

### MonthlySummary

Represents one completed calendar month of profile memory.

- **id**: string
- **profile_id**: string
- **month_key**: string
- **month_start**: date/time
- **month_end**: date/time
- **content**: string
- **source_state_version**: string
- **freshness_state**: `fresh | stale | regenerating`
- **created_at**: timestamp
- **updated_at**: timestamp

Validation:
- Only one monthly summary may exist for a given profile and calendar month.
- `month_start < month_end`.
- Older history may be represented by monthly summaries instead of raw notes or weekly summaries in the assembled default context.

### TimelineContextWindow

Represents the assembled default memory context for a chat turn.

- **profile_id**: string
- **raw_notes**: ordered list of `MemoryNote`
- **weekly_summaries**: ordered list of `WeeklySummary`
- **monthly_summaries**: ordered list of `MonthlySummary`
- **assembled_memory_state**: string
- **generated_at**: timestamp

Validation:
- Time coverage must follow the configured order: raw-note window, then weekly-summary window, then monthly-summary window.
- Empty layers may be omitted.
- Included records must clearly preserve their granularity type.

### MemorySearchToolDefinition

Represents a tool exposed to the LLM through provider tool calling.

- **name**: string
- **description**: string
- **input_schema**: structured object
- **result_shape**: structured object
- **enabled**: boolean

Validation:
- Tool names are unique per request.
- Tool definitions must be included with both the initial provider request and the follow-up request containing tool results.

### VectorSearchRequest

Represents a keyword-driven search request initiated by the LLM.

- **profile_id**: string
- **query**: string
- **limit**: integer
- **requested_at**: timestamp

Validation:
- `query` must be non-empty.
- `limit > 0` and bounded to a safe maximum.

### TimeRangeSearchRequest

Represents a date-bounded memory lookup initiated by the LLM.

- **profile_id**: string
- **start_time**: timestamp
- **end_time**: timestamp
- **limit**: integer
- **requested_at**: timestamp

Validation:
- `start_time <= end_time`.
- `limit > 0` and bounded to a safe maximum.

### MemorySearchResultItem

Represents one result returned by either search tool.

- **record_type**: `raw_note | weekly_summary | monthly_summary`
- **record_id**: string
- **profile_id**: string
- **content**: string
- **category**: string (nullable for summaries)
- **time_start**: timestamp
- **time_end**: timestamp
- **relevance_score**: number (nullable for time-range-only ordering)

Validation:
- `record_type` determines whether category is required.
- Time-range results must fall fully within the requested boundaries.

### ProviderModelMetadata

Represents cached provider model metadata relevant to chat telemetry.

- **model_id**: string
- **provider_type**: string
- **display_name**: string
- **context_length**: integer
- **supports_tools**: boolean
- **fetched_at**: timestamp

Validation:
- `context_length >= 0`.
- Zero context length means unknown.
- Tool exposure is allowed only when model/provider capability is known to support it.

### FooterTelemetrySnapshot

Represents the footer status values shown during chat.

- **tokens_in_total**: integer
- **tokens_out_total**: integer
- **cost_usd**: number
- **context_used_tokens**: integer
- **context_max_tokens**: integer
- **context_used_percent**: number
- **cache_status**: string
- **captured_at**: timestamp

Validation:
- `context_used_percent` is derived only when `context_max_tokens > 0`.
- Unknown max context must not yield a misleading percentage.

### ContextCacheIdentity

Represents the retrieval-shaping inputs required for safe context-cache reuse.

- **profile_id**: string
- **system_prompt_fingerprint**: string
- **assembled_memory_state**: string
- **token_budget**: integer
- **embedding_model**: string
- **timeline_settings_fingerprint**: string
- **cache_key**: string

Validation:
- Any change to prompt, assembled memory state, token budget, embedding model, or timeline settings yields a new cache identity.

## Relationships

- `TimelineSettings` shapes one `TimelineContextWindow` per retrieval request for a profile.
- `MemoryNote` entries are source material for `WeeklySummary` and `MonthlySummary` artifacts.
- `WeeklySummary` and `MonthlySummary` records may also appear as `MemorySearchResultItem` values.
- `MemorySearchToolDefinition` describes how the LLM may request `VectorSearchRequest` and `TimeRangeSearchRequest` operations.
- `ProviderModelMetadata` informs whether tool calling is enabled and what `context_length` should drive `FooterTelemetrySnapshot`.
- `ContextCacheIdentity` determines validity for caching one `TimelineContextWindow`.

## State Transitions

### Summary Freshness

`WeeklySummary` and `MonthlySummary` freshness transitions:

- `fresh -> stale` when covered notes change
- `stale -> regenerating` when summary rebuild begins
- `regenerating -> fresh` on successful replacement
- `regenerating -> stale` if regeneration fails

### Timeline Settings

`TimelineSettings` transitions:

- updated settings invalidate cache identity for subsequent retrievals
- zero-value layers remain valid and simply skip that layer in context assembly

### Tool Call Lifecycle

Tool call flow:

- tool definitions attached to provider request
- model emits tool call request
- local tool executor resolves request against vector index or SQLite
- tool results returned to provider in follow-up request
- final assistant response incorporates tool results

### Footer Telemetry

Footer telemetry transitions:

- `unknown_capacity` when model metadata is missing
- `known_capacity` once provider metadata supplies `context_length`
- updated each chat turn from latest usage plus cached model metadata
