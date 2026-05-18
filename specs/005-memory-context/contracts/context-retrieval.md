# Contract: Context Retrieval Cache Behavior (FR-026..FR-037)

## Inputs (Cache Identity)

- **profile_id**: string
- **system_prompt** (or deterministic prompt identity): string
- **notes_hash**: string
- **note_budget_tokens**: int
- **embedding_model**: string

## Retrieval Behavior Contract

1. Compute cache identity from all five inputs above.
2. Lookup order MUST be:
   - L1 (in-session memory cache) first
   - L2 (persistent cache) second
3. Cache hit is valid only when identity fully matches.
4. If entry is **slightly stale** and structurally valid:
   - return it immediately,
   - trigger background revalidation,
   - do not block current request.
5. Revalidation result is used for subsequent requests.
6. Event-driven stale/invalidation MUST happen on:
   - note create/update/delete,
   - system prompt change,
   - token budget change,
   - embedding model change.
7. Miss reason classification MUST be recorded for non-hit outcomes.

## Output Additions

In addition to existing retrieval output fields, expose:

- **cache_tier**: `l1 | l2 | none`
- **cache_tier**: `l1 | l2 | none`
- **cache_hit**: bool
- **served_stale**: bool
- **revalidation_started**: bool
- **miss_reason**: enum (nullable)

## Diagnostics Contract

Expose aggregated diagnostics with at least:

- **hit_rate**
- **miss_rate**
- **average_rebuild_time**
- **top_miss_reasons** (ordered by frequency)

Example miss reasons (non-exhaustive):
- `notes_changed`
- `prompt_changed`
- `token_budget_changed`
- `embedding_model_changed`
- `cache_expired`
- `not_found`
