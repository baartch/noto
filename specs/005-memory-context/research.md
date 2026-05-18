# Research: Cache Hardening Scope (FR-026..FR-037)

## Decision 1: Cache identity includes embedding model and all request-shaping inputs

**Decision**: Use a cache identity derived from `(profile, system prompt identity/content, notes hash, token budget, embedding model)`.

**Rationale**: Any change in these inputs can alter retrieval output. Identity parity is required to prevent stale/wrong cache reuse.

**Alternatives considered**:
- Keep current key (without embedding model): rejected due to incorrect cross-model reuse.
- Use TTL-only freshness without strict identity: rejected due to correctness risk.

## Decision 2: Serve slightly stale entries with asynchronous revalidation

**Decision**: Apply stale-while-revalidate for entries within a bounded stale window; return immediately, refresh in background.

**Rationale**: Preserves fast UX while converging cache to fresh state without blocking chat flow.

**Alternatives considered**:
- Block until rebuild for stale entries: rejected (latency spike).
- Never serve stale entries: rejected (worse cold-start/reopen experience).

## Decision 3: Event-driven invalidation for key data/config changes

**Decision**: Mark stale or invalidate on note create/update/delete, system prompt change, token budget change, embedding model change.

**Rationale**: Event-driven invalidation prevents waiting for TTL expiry when correctness-affecting changes occur.

**Alternatives considered**:
- TTL-only invalidation: rejected due to stale accuracy windows.
- Always full profile invalidate on every event: partially acceptable but less efficient; prefer targeted stale marking where possible.

## Decision 4: Two-level cache strategy (L1 in-process, L2 persistent)

**Decision**: Use L1 in-memory cache for fastest repeated lookups in current process; fallback to L2 persistent cache for cross-session reuse.

**Rationale**: Combines low-latency hot-path behavior with restart persistence.

**Alternatives considered**:
- L2 only: rejected due to avoidable lookup overhead during active sessions.
- L1 only: rejected due to loss of cross-session benefits.

## Decision 5: Diagnostics include hit/miss rates, rebuild time, miss reasons

**Decision**: Track and expose hit rate, miss rate, average rebuild time, and ranked miss reasons.

**Rationale**: Enables maintainers to tune cache behavior and quickly identify causes of rebuild churn.

**Alternatives considered**:
- Expose only raw counters: rejected (insufficient for tuning).
- Expose diagnostics only in debug mode: rejected (reduced operational visibility).
