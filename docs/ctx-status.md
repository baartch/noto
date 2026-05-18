# Context Status (`ctx:`) in the Footer

Noto shows a `ctx:` status in the footer to indicate how memory context was produced.

## States

- `ctx:l1-hit` — Context served from in-process memory cache (fastest).
- `ctx:l2-hit` — Context served from persistent cache (cross-session reuse).
- `ctx:swr` — **Stale-While-Revalidate (SWR)**: slightly stale context is served immediately while a background refresh starts.
- `ctx:rebuild` — No usable cache entry; context rebuilt from current data.
- `ctx:miss(<reason>)` — Cache miss with a classified reason (for example `embedding_model_changed`).
- `ctx:n/a` — Context status unavailable.

## What users should expect

- `l1-hit`/`l2-hit` usually mean faster startup and lower latency.
- `swr` means the app favored responsiveness and is refreshing cache in the background.
- `rebuild`/`miss(...)` means the app recomputed context to keep correctness.

## Operator logs

Session startup logs include context retrieval details:

- `tier` (`l1`/`l2`/`none`)
- `hit` (`true`/`false`)
- `stale` (`true`/`false`)
- `revalidate` (`true`/`false`)
- `miss_reason` (when present)

These logs help verify cache behavior matches footer status.
