# Research: Bubble Tea TUI Standard

**Date**: 2026-04-27

## Decision 1: Footer usage and cost metrics source

- **Decision**: Parse token/cost usage directly from every API response `usage` object (including chunk responses), then aggregate for display fields: up/down/cache read/cache write/cost.
- **Rationale**: Usage is already emitted by providers and is the authoritative source for prompt/completion/cached/cache-write/cost values.
- **Alternatives considered**:
  - Infer usage from request payload sizes (rejected: inaccurate and provider-dependent)
  - Display only per-request values (rejected: footer needs session-level running totals)

## Decision 2: Multi-model accounting scope

- **Decision**: Include usage/cost from all model classes involved in a user flow: main chat model, extractor model, and embeddings model.
- **Rationale**: Footer cost/usage must represent true session consumption, not only chat completions.
- **Alternatives considered**:
  - Track only main model usage (rejected: underreports real cost)
  - Track only visible chat requests (rejected: misses background extraction/embedding work)

## Decision 3: Aggregation model

- **Decision**: Maintain an in-memory session usage accumulator in TUI state with additive updates on each response carrying usage.
- **Rationale**: Footer requires fast render-time values and avoids repeated DB scans.
- **Alternatives considered**:
  - Persist and query on each render (rejected: unnecessary IO and complexity for live footer)

## Decision 4: Missing usage handling

- **Decision**: If a response lacks `usage`, do not mutate counters; keep last known totals.
- **Rationale**: Some provider responses may omit usage in edge cases; stale-but-correct totals are better than guessed values.
- **Alternatives considered**:
  - Estimate missing values heuristically (rejected: misleading)

## Decision 5: Footer payload contract

- **Decision**: Footer must always render these fields: tokens (up/down/cache read/cache write), context cache stats (`ctx:miss|hit`), total cost, current profile, current main model, app version, and help keybinding.
- **Rationale**: Matches clarified UX requirement and keeps operational context visible.
- **Alternatives considered**:
  - Progressive disclosure/toggled details (rejected: conflicts with “always visible” clarification)
