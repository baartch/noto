# Data Model: Cache Hardening (FR-026..FR-037)

## Entities

### CacheIdentity

Represents the exact retrieval-shaping inputs required for cache correctness.

- **profile_id**: string
- **system_prompt_fingerprint**: string
- **notes_hash**: string
- **token_budget**: integer
- **embedding_model**: string
- **cache_key**: string (stable derived value)

Validation:
- All fields are required.
- Any field change yields a new `cache_key`.

### ContextCacheEntry

Cached assembled retrieval context with freshness metadata.

- **cache_key**: string
- **profile_id**: string
- **payload**: string
- **created_at**: timestamp
- **expires_at**: timestamp (nullable)
- **stale_at**: timestamp (nullable)
- **freshness_state**: `fresh | slightly_stale | stale | invalid`

Validation:
- Entry must map to one `CacheIdentity`.
- `slightly_stale` may be served only under SWR policy window.

### CacheTierSnapshot

Transient runtime view of cache lookup path.

- **l1_hit**: boolean
- **l2_hit**: boolean
- **served_stale**: boolean
- **revalidation_triggered**: boolean
- **miss_reason**: enum (nullable)

Validation:
- `l1_hit` and `l2_hit` cannot both be true for a single served response.
- `revalidation_triggered` can be true only when `served_stale` is true.

### CacheInvalidationEvent

Correctness-impacting event that marks entries stale/invalid.

- **event_type**: `note_created | note_updated | note_deleted | system_prompt_changed | token_budget_changed | embedding_model_changed`
- **profile_id**: string
- **occurred_at**: timestamp
- **scope**: `targeted | profile_wide`

Validation:
- Event type must be one of the six required triggers.

### CacheDiagnosticsSnapshot

Aggregated operational diagnostics for cache behavior.

- **window_start**: timestamp
- **window_end**: timestamp
- **total_requests**: integer
- **hits**: integer
- **misses**: integer
- **hit_rate**: number (0..1)
- **miss_rate**: number (0..1)
- **avg_rebuild_time_ms**: number
- **top_miss_reasons**: ordered list of `{reason, count}`

Validation:
- `hits + misses = total_requests`.
- `hit_rate + miss_rate = 1.0` (within rounding tolerance).

## Relationships

- `CacheIdentity` 1→N `ContextCacheEntry` over time (new entries on revalidation).
- `CacheInvalidationEvent` impacts one or more `ContextCacheEntry` records by profile/scope.
- `CacheTierSnapshot` records runtime decision for one retrieval request.
- `CacheDiagnosticsSnapshot` aggregates many `CacheTierSnapshot` records.

## State Transitions

`ContextCacheEntry.freshness_state` transitions:

- `fresh -> slightly_stale` (time/window based)
- `slightly_stale -> fresh` (successful background revalidation)
- `slightly_stale -> stale` (window exceeded)
- `fresh/slightly_stale/stale -> invalid` (event-driven invalidation)
- `invalid -> fresh` (rebuild + upsert)
