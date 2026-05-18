# Implementation Plan: Memory Context Cache Hardening

**Branch**: `005-memory-context` | **Date**: 2026-05-18 | **Spec**: [specs/005-memory-context/spec.md](./spec.md)
**Input**: Feature specification from `/specs/005-memory-context/spec.md`

## Summary

Implement only the newly added cache requirements (FR-026 to FR-037): extend cache identity to include embedding model and other retrieval-shaping inputs, add stale-while-revalidate behavior, add event-driven invalidation triggers, introduce a two-level cache strategy (in-session + persistent), and expose actionable cache diagnostics.

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: Cobra CLI, Bubble Tea/Bubbles/Lip Gloss, modernc.org/sqlite, internal memory/vector packages  
**Storage**: Profile-local SQLite (`~/.noto/profiles/<profile>/memory.db`) plus in-process memory cache  
**Testing**: `go test ./...`, plus `make test`, `make lint`, `make fmt`  
**Target Platform**: Cross-platform CLI/TUI runtime (Linux/macOS/Windows terminals)  
**Project Type**: CLI/TUI application  
**Performance Goals**: Preserve responsive context assembly and improve repeated-request latency by preferring in-session cache hits; stale entries should return immediately while refresh happens asynchronously  
**Constraints**: Cache correctness must reflect profile/prompt/notes hash/token budget/embedding model changes; invalidation events must be reflected before next retrieval; no blocking user chat flow for revalidation  
**Scale/Scope**: Scoped only to FR-026..FR-037 in `spec.md`; no changes to extractor schema, prompt bootstrap, or vector ranking logic beyond cache identity/invalidation integration

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Code Quality Is Enforced**: PASS — plan includes explicit updates to cache modules and deterministic miss-reason classification; validation commands include `make fmt`, `make lint`, `make test`.
- **Testing Standards Are Non-Negotiable**: PASS — plan requires unit/integration coverage for cache key identity, SWR behavior, invalidation events, tier ordering, and diagnostics aggregation.
- **User Experience Consistency First**: PASS — stale-while-revalidate returns immediate responses and avoids chat-flow blocking; diagnostics are maintainer-facing and do not alter core user interaction patterns.

Post-Design Re-check: PASS (no principle violations introduced by Phase 1 artifacts).

## Project Structure

### Documentation (this feature)

```text
specs/005-memory-context/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── context-retrieval.md
└── tasks.md
```

### Source Code (repository root)

```text
cmd/
└── noto/

internal/
├── cache/
│   ├── service.go
│   ├── invalidation.go
│   └── doc.go
├── memory/
│   └── retrieval.go
└── store/

tests/
├── unit/
│   ├── cache/
│   └── memory/
└── integration/
    └── memory/
```

**Structure Decision**: Use the existing single-project Go CLI/TUI structure and limit implementation to cache/retrieval components plus focused tests.

## Phase 0: Research Plan

1. Confirm cache identity dimensions and miss-reason taxonomy for FR-026/027/037.
2. Select stale-while-revalidate policy boundaries and non-blocking refresh pattern for FR-028/029.
3. Define event-driven invalidation semantics for FR-030..FR-033.
4. Define two-level cache read/write ordering and coherence behavior for FR-034/035.
5. Define diagnostics aggregation window and reporting shape for FR-036/037.

## Phase 1: Design Plan

1. Update data model docs with cache identity, freshness state, and diagnostics entities.
2. Update retrieval contract with:
   - identity inputs including embedding model,
   - SWR behavior,
   - invalidation-triggered staleness,
   - L1/L2 lookup order,
   - diagnostics output fields.
3. Update quickstart with scenarios verifying only FR-026..FR-037 behavior.
4. Update agent context reference to this plan in `AGENTS.md`.

## Phase 2: Implementation Planning (for `/speckit.tasks`)

- Add/adjust cache key creation to include embedding model and all identity dimensions.
- Implement in-session L1 cache in front of persistent L2 cache.
- Implement stale-while-revalidate path with safe background refresh and race protection.
- Wire event-driven invalidation hooks for note/prompt/token-budget/embedding-model changes.
- Add diagnostics counters/timers and miss-reason aggregation surfaced through existing observability surfaces.
- Add full automated tests for FR-026..FR-037 and regressions.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
