# Quickstart: Validate Newly Added Cache Requirements (FR-026..FR-037)

This quickstart verifies **only** the recently added cache hardening requirements.

## Prerequisites

- Profile with existing notes and working retrieval.
- Ability to change token budget and embedding model config.
- Diagnostics surface/log output enabled for cache stats.

## Validation Steps

1. **Cache identity includes embedding model**
   - Build/serve context with embedding model A.
   - Switch to embedding model B with same prompt/notes/token budget.
   - Verify previous cache entry is not used as a hit.

2. **Identity mismatch rejects hit**
   - Repeat retrieval while changing one dimension at a time:
     - prompt
     - notes content (hash changes)
     - token budget
     - embedding model
   - Verify each change yields non-hit behavior with corresponding miss reason.

3. **Stale-while-revalidate behavior**
   - Force entry into slightly stale window.
   - Request context and verify:
     - immediate response returned,
     - response flagged as served stale,
     - background refresh starts.
   - Next request should use refreshed entry.

4. **Event-driven invalidation**
   - Trigger each required event:
     - note create/update/delete
     - system prompt change
     - token budget change
     - embedding model change
   - Verify affected cache entry is marked stale/invalid before next retrieval.

5. **Two-level cache lookup order**
   - Warm L2 and clear L1; verify retrieval from L2 then promotion/warm into L1.
   - Repeat request in same process; verify L1 hit takes precedence.

6. **Diagnostics**
   - Generate mix of hits and misses.
   - Verify diagnostics include:
     - hit/miss rate
     - average rebuild time
     - top miss reasons with counts

## Expected Results

- No stale/wrong hits across embedding model or other identity input changes.
- Slightly stale entries are served immediately and revalidated asynchronously.
- Event-driven invalidation occurs on all required triggers.
- L1 then L2 lookup order is consistently enforced.
- Diagnostics clearly explain cache effectiveness and miss causes.

## Validation Commands

- `make fmt`
- `make lint`
- `make test`
