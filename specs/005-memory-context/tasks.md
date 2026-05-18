# Tasks: Memory Context Cache Hardening

**Input**: Design documents from `/specs/005-memory-context/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/, quickstart.md

**Tests**: Include automated tests per constitution requirements and plan commitments.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story label (`[US2]`, `[US3]`)
- All tasks include exact file paths

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare scaffolding for scoped cache-hardening changes.

- [X] T001 Review and align cache-hardening scope notes in specs/005-memory-context/plan.md
- [X] T002 Create cache-hardening test fixture helpers in tests/integration/memory/cache_hardening_fixtures_test.go
- [X] T003 [P] Add diagnostics/miss-reason constants shared by tests in tests/unit/cache/testdata/miss_reasons.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core cache primitives required before user-story implementation.

- [X] T004 Define cache identity value object and key builder in internal/cache/identity.go
- [X] T005 [P] Add cache freshness state model and helpers in internal/cache/freshness.go
- [X] T006 [P] Add cache diagnostics aggregator primitives in internal/cache/diagnostics.go
- [X] T007 Extend cache service interfaces for tier metadata and diagnostics in internal/cache/service.go
- [X] T008 Add unit tests for identity/freshness primitives in tests/unit/cache/identity_test.go
- [X] T009 Add unit tests for diagnostics aggregation primitives in tests/unit/cache/diagnostics_test.go

**Checkpoint**: Foundation complete; user story implementation can begin.

---

## Phase 3: User Story 2 - Fast and Correct Cache Reuse (Priority: P2) 🎯 MVP

**Goal**: Deliver correct cache identity matching, SWR behavior, event-driven invalidation, and L1→L2 lookup order.

**Independent Test**: Run retrieval flows where prompt/notes/token budget/embedding model vary and verify invalidation + SWR + tier order behavior without full rebuild on eligible repeats.

### Tests for User Story 2

- [X] T010 [P] [US2] Add retrieval cache-key mismatch tests (including embedding model) in tests/unit/memory/retrieval_cache_key_test.go
- [X] T011 [P] [US2] Add stale-while-revalidate behavior tests in tests/unit/memory/retrieval_swr_test.go
- [X] T012 [P] [US2] Add L1-before-L2 tier selection tests in tests/unit/cache/service_tiers_test.go
- [X] T013 [P] [US2] Add event-driven invalidation trigger tests in tests/unit/cache/invalidation_triggers_test.go
- [X] T014 [US2] Add integration test for cross-session persistent-cache reuse in tests/integration/memory/cache_persistence_test.go
- [X] T015 [US2] Add integration test for prompt/token/embedding invalidation pathways in tests/integration/memory/cache_invalidation_test.go

### Implementation for User Story 2

- [X] T016 [US2] Update retrieval cache key construction to include embedding model and full identity inputs in internal/memory/retrieval.go
- [X] T017 [US2] Implement in-process L1 cache with L1→L2 lookup and L2 promotion in internal/cache/service.go
- [X] T018 [US2] Implement slightly-stale serve path and async revalidation workflow in internal/cache/service.go
- [X] T019 [US2] Guard against stale refresh race overwrite using identity/freshness checks in internal/cache/service.go
- [X] T020 [US2] Wire note create/update/delete invalidation events in internal/cache/invalidation.go
- [X] T021 [US2] Wire system prompt, token budget, and embedding model invalidation events in internal/cache/invalidation.go
- [X] T022 [US2] Propagate tier/served-stale/revalidation metadata through retrieval result in internal/memory/retrieval.go
- [X] T023 [US2] Update cache behavior contract to match implemented US2 semantics in specs/005-memory-context/contracts/context-retrieval.md

**Checkpoint**: User Story 2 is independently functional and testable.

---

## Phase 4: User Story 3 - Observable Cache Health (Priority: P3)

**Goal**: Expose actionable cache diagnostics (hit/miss rate, average rebuild time, top miss reasons).

**Independent Test**: Trigger mixed hit/miss outcomes and verify diagnostics report accurate rates, rebuild timing, and ranked miss reasons.

### Tests for User Story 3

- [X] T024 [P] [US3] Add diagnostics snapshot accuracy tests in tests/unit/cache/diagnostics_snapshot_test.go
- [X] T025 [P] [US3] Add miss-reason classification tests in tests/unit/memory/retrieval_miss_reason_test.go
- [X] T026 [US3] Add integration test for diagnostics reporting window in tests/integration/memory/cache_diagnostics_test.go

### Implementation for User Story 3

- [X] T027 [US3] Record hit/miss/rebuild timing and miss reasons during retrieval lifecycle in internal/memory/retrieval.go
- [X] T028 [US3] Expose diagnostics snapshot API from cache service in internal/cache/service.go
- [X] T029 [US3] Surface diagnostics fields in retrieval-facing output contract in specs/005-memory-context/contracts/context-retrieval.md
- [X] T030 [US3] Document diagnostics validation workflow in specs/005-memory-context/quickstart.md

**Checkpoint**: User Story 3 is independently functional and testable.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Final verification and cleanup across stories.

- [X] T031 [P] Refine cache hardening data-model notes for final implementation alignment in specs/005-memory-context/data-model.md
- [X] T032 Run full verification suite (`make fmt`, `make lint`, `make test`) and capture outcomes in specs/005-memory-context/quickstart.md
- [X] T033 [P] Remove dead code/obsolete comments from pre-hardening cache paths in internal/cache/doc.go

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: starts immediately
- **Phase 2 (Foundational)**: depends on Phase 1; blocks story work
- **Phase 3 (US2)**: depends on Phase 2
- **Phase 4 (US3)**: depends on Phase 2 and can start after US2 core telemetry hooks (T022)
- **Phase 5 (Polish)**: depends on completion of US2 and US3

### User Story Dependencies

- **US2 (P2)**: no dependency on US3; delivers MVP for this scoped work
- **US3 (P3)**: depends on US2 instrumentation points but remains independently testable once available

### Within User Stories

- Test tasks should be authored first and executed before finalizing implementation tasks.
- Core implementation precedes contract/doc updates.

## Parallel Opportunities

- **Setup**: T003 can run with T001/T002
- **Foundational**: T005 and T006 can run in parallel; T008 and T009 can run in parallel after T004–T006
- **US2**: T010–T013 can run in parallel; T020 and T021 can run in parallel; T014 and T015 can run in parallel after implementation
- **US3**: T024 and T025 can run in parallel; T029 and T030 can run in parallel after T027/T028
- **Polish**: T031 and T033 can run in parallel before T032 final gate

---

## Parallel Example: User Story 2

```bash
# Parallel tests
Task: "T010 [US2] tests/unit/memory/retrieval_cache_key_test.go"
Task: "T011 [US2] tests/unit/memory/retrieval_swr_test.go"
Task: "T012 [US2] tests/unit/cache/service_tiers_test.go"
Task: "T013 [US2] tests/unit/cache/invalidation_triggers_test.go"

# Parallel invalidation wiring
Task: "T020 [US2] internal/cache/invalidation.go (note events)"
Task: "T021 [US2] internal/cache/invalidation.go (prompt/token/embedding events)"
```

## Implementation Strategy

### MVP First (US2)

1. Complete Phase 1 and Phase 2
2. Complete US2 (Phase 3)
3. Validate US2 independent test criteria and quickstart checks
4. Ship scoped MVP

### Incremental Delivery

1. Deliver US2 cache correctness + latency behavior
2. Deliver US3 diagnostics visibility
3. Final polish and full-suite validation

### Format Validation Checklist

- All tasks use `- [ ]` checkbox format
- All tasks include sequential IDs `T001`..`T033`
- `[P]` marker used only for parallelizable tasks
- User-story tasks include `[US2]` or `[US3]` labels
- Every task includes an explicit file path
