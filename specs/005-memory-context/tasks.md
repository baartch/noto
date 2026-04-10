# Tasks: Memory Context Indexing

**Input**: Design documents from `/specs/005-memory-context/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED for every user story and must be created before implementation tasks.

## Phase 1: Setup (Shared Infrastructure)

- [X] T001 Review current memory extraction and retrieval flow in internal/memory/extractor.go and internal/memory/retrieval.go
- [X] T002 Review vector index plumbing in internal/vector/ (index.go, sync.go, rebuild.go)

---

## Phase 2: Foundational (Blocking Prerequisites)

- [X] T003 Define token-budget setting storage and settings dialog behavior in specs/005-memory-context/quickstart.md
- [X] T004 Define footer warning copy and extractor fallback UX in specs/005-memory-context/quickstart.md

---

## Phase 3: User Story 1 - Relevant Memory Context (Priority: P1) 🎯 MVP

**Goal**: Inject only relevant notes within the token budget using vector ranking or deterministic fallback.

**Independent Test**: With many notes, verify context includes only top relevant notes within budget or fallback ordering.

### Tests for User Story 1 (REQUIRED) ⚠️

- [X] T005 [P] [US1] Add integration coverage for token-budgeted relevance selection in tests/integration/context_cache_lifecycle_test.go
- [X] T006 [P] [US1] Add integration coverage for fallback ordering in tests/integration/context_cache_lifecycle_test.go

### Implementation for User Story 1

- [X] T007 [US1] Add token budget setting (default 1500) and read path in internal/config/ (new settings file)
- [ ] T008 [US1] Apply token-budgeted relevance selection in internal/memory/retrieval.go
- [ ] T009 [US1] Implement importance-then-recency fallback in internal/memory/retrieval.go
- [ ] T010 [US1] Wire vector index ranking into retrieval in internal/memory/retrieval.go

**Checkpoint**: Relevance selection respects token budget and fallback ordering.

---

## Phase 4: User Story 2 - Persistent Context Across Sessions (Priority: P2)

**Goal**: Preserve context caching and index usage across restarts.

**Independent Test**: Restart app and ensure cached context and index are reused.

### Tests for User Story 2 (REQUIRED) ⚠️

- [ ] T011 [P] [US2] Add integration coverage for cache reuse across restarts in tests/integration/context_cache_lifecycle_test.go

### Implementation for User Story 2

- [ ] T012 [US2] Persist context cache metadata and reuse cache in internal/memory/retrieval.go
- [ ] T013 [US2] Ensure vector index manifest reuse across restarts in internal/vector/sync.go

**Checkpoint**: Cache and index reuse across restarts verified.

---

## Phase 5: User Story 3 - Automatic Context Maintenance (Priority: P2)

**Goal**: Maintain vector index incrementally with periodic compaction/rebuild.

**Independent Test**: Add notes and confirm index updates and compaction run automatically.

### Tests for User Story 3 (REQUIRED) ⚠️

- [ ] T014 [P] [US3] Add integration coverage for incremental indexing in tests/integration/vector_sync_retrieval_test.go
- [ ] T015 [P] [US3] Add integration coverage for compaction/rebuild in tests/integration/vector_rebuild_fallback_test.go

### Implementation for User Story 3

- [ ] T016 [US3] Implement incremental index updates on note changes in internal/vector/sync.go
- [ ] T017 [US3] Implement periodic compaction/rebuild triggers in internal/vector/rebuild.go

**Checkpoint**: Index updates/compaction run without manual commands.

---

## Phase 6: User Story 4 - Extractor Fallback Warning (Priority: P2)

**Goal**: Use main model when extractor model is missing and warn in footer.

**Independent Test**: Clear extractor model config and verify fallback + footer warning.

### Tests for User Story 4 (REQUIRED) ⚠️

- [ ] T018 [P] [US4] Add integration coverage for extractor fallback warning in tests/integration/tui_flow_regression_test.go

### Implementation for User Story 4

- [ ] T019 [US4] Use main model for extraction when extractor model missing in internal/memory/extractor.go
- [ ] T020 [US4] Surface footer warning state in internal/tui/model.go

**Checkpoint**: Fallback and warning active when extractor model absent.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T021 [P] Run go test ./... and golangci-lint run; capture results in specs/005-memory-context/quickstart.md
- [ ] T022 Validate UX and performance goals; update specs/005-memory-context/quickstart.md

---

## Dependencies & Execution Order

- **Phase 1** → **Phase 2** → **Phase 3** → **Phase 4** → **Phase 5** → **Phase 6** → **Phase 7**
- Tests must be completed before implementation tasks in each user story phase.
