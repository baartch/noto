# Contract: Timeline Context Retrieval

## Inputs

- **profile_id**: string
- **system_prompt** (or deterministic prompt identity): string
- **timeline_settings**:
  - `raw_note_days`
  - `weekly_summary_months`
  - `monthly_summary_months`

Validation:
- `raw_note_days > 0`
- `weekly_summary_months >= 0`
- `monthly_summary_months` is either `all_remaining` or an integer `>= 0`
- **note_budget_tokens**: int
- **embedding_model**: string
- **current_time**: timestamp

## Context Assembly Contract

1. The system MUST assemble default memory context as a time-layered window.
2. The system MUST include all raw notes that fall within the configured recent raw-note day window.
3. The raw-note window MUST be computed from at least the configured rolling-day span and then extended backward to the preceding Monday so there is no gap before the weekly-summary layer.
4. The system MUST include weekly summaries for the configured monthly span immediately preceding the raw-note window, starting with the first full week before the raw-note boundary.
5. The system MUST include monthly summaries for the configured older-history monthly-summary window, switching only on completed calendar periods after the weekly-summary layer.
6. If `monthly_summary_months` is a bounded integer instead of `all_remaining`, any history older than that monthly cutoff MUST be excluded from the default assembled context.
7. If a configured layer has value `0`, that layer is skipped without failing context assembly.
8. If a required summary is missing or stale, the system MUST use the best available memory for that period until regeneration completes.
9. Conversation summaries MUST NOT be required or included in the default assembled context.
10. The assembled context MUST distinguish raw notes, weekly summaries, and monthly summaries in its formatted output.

## Rollup Generation Contract

1. Entering a new completed week makes the previous week eligible for weekly summary generation.
2. Entering a new completed month makes the previous month eligible for monthly summary generation.
3. If the app was inactive during a boundary transition, missing summaries MUST be created the next time the profile is processed.
4. Only one weekly summary per profile/week and one monthly summary per profile/month may exist as the active artifact.
5. When underlying notes change for a covered period, the corresponding summary MUST be marked stale and become eligible for regeneration.

## Cache Contract

1. Cache identity MUST include profile, prompt identity, assembled memory state, token budget, embedding model, and timeline settings.
2. Lookup order MUST be:
   - L1 (in-session memory cache) first
   - L2 (persistent cache) second
3. Cache hit is valid only when identity fully matches.
4. If an entry is slightly stale and structurally valid:
   - return it immediately,
   - trigger background revalidation,
   - do not block the current request.
5. Cache entries MUST be invalidated or marked stale when notes, summaries, prompt, token budget, embedding model, or timeline settings change.

## Retrieval Output Contract

The retrieval result MUST expose at least:

- **system_prompt**
- **memory_block**
- **assembled_prompt**
- **cache_tier**: `l1 | l2 | none`
- **cache_hit**: bool
- **served_stale**: bool
- **revalidation_started**: bool
- **miss_reason**: enum (nullable)
- **timeline_layers_present**:
  - raw notes
  - weekly summaries
  - monthly summaries

## Diagnostics Contract

Expose aggregated diagnostics with at least:

- **hit_rate**
- **miss_rate**
- **average_rebuild_time**
- **top_miss_reasons**
- **recent_rollup_activity**
- **recent_rollup_failures**
