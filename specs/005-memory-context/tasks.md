# Tasks: Memory Context Indexing

**Input**: Design documents from `/specs/005-memory-context/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/, quickstart.md

**Tests**: Test tasks are included because the spec mandates automated coverage for changed behavior.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare scaffolding for prompt-file + extraction-contract work.

- [X] T001 Add/update feature wiring notes in specs/005-memory-context/quickstart.md for prompt files and extractor schema validation paths
- [X] T002 Create shared test fixtures for extractor JSON payload cases in tests/integration/memory/extractor_payload_fixtures.go
- [X] T003 [P] Add helper utilities for profile prompt file setup in tests/integration/testutil/prompt_files.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure required before user stories.

- [X] T004 Implement profile prompt file path helpers (`system.md`, `extractor.md`) in internal/config/paths.go
- [X] T005 Implement prompt file read/write + default bootstrap logic in internal/config/settings.go
- [X] T006 [P] Define extractor payload validation types and schema checks in internal/memory/extractor.go
- [X] T007 Implement extractor payload validation logging/warnings for malformed payload rejection in internal/memory/logging.go
- [X] T008 Add unit tests for prompt file persistence and defaults in tests/unit/store/prompt_file_store_test.go
- [X] T009 [P] Add unit tests for extractor payload schema validation (including top-level `has_new_info` and `confidence` range) in tests/unit/memory/extractor_payload_validation_test.go
- [X] T010 [P] Add unit tests for missing prompt-file bootstrap warning behavior in tests/unit/store/prompt_file_store_test.go

**Checkpoint**: Prompt file persistence + extractor schema validation foundation complete.

---

## Phase 3: User Story 1 - Relevant Memory Context (Priority: P1) 🎯 MVP

**Goal**: Ensure extraction and retrieval produce accurate, useful note context with strict JSON semantics, including top-level metadata.

**Independent Test**: Run extraction on multi-fact input and verify valid JSON with top-level `has_new_info` + `confidence` (0.0..1.0), note array, per-note actions, `target_id` on updates, constrained category, and relevance-limited injection.

### Tests for User Story 1

- [ ] T011 [P] [US1] Add integration test for valid extractor JSON with per-note actions in tests/integration/memory/extractor_json_contract_test.go
- [ ] T012 [P] [US1] Add integration test for update notes requiring `target_id` in tests/integration/memory/extractor_json_contract_test.go
- [ ] T013 [P] [US1] Add integration test for category enforcement (`fact|progress|blocker|action_item|other`) in tests/integration/memory/extractor_json_contract_test.go
- [ ] T014 [P] [US1] Add integration test for malformed extractor payload rejection in tests/integration/memory/extractor_json_contract_test.go
- [ ] T015 [P] [US1] Add integration test for required top-level `has_new_info` + `confidence` validation in tests/integration/memory/extractor_json_contract_test.go
- [ ] T016 [P] [US1] Extend relevance-selection integration coverage for token-budgeted injection in tests/integration/context_cache_lifecycle_test.go

### Implementation for User Story 1

- [ ] T017 [US1] Update extractor prompt content/template to require strict JSON output and per-note action semantics in internal/memory/extractor.go
- [ ] T018 [US1] Enforce note-level `action` and `target_id` rules during extraction processing in internal/memory/processor.go
- [ ] T019 [US1] Enforce allowed note category values during extraction processing in internal/memory/processor.go
- [ ] T020 [US1] Reject invalid extractor payloads before persistence and emit warning telemetry in internal/memory/extractor.go
- [ ] T021 [US1] Parse and validate top-level `has_new_info` and `confidence` in internal/memory/extractor.go
- [ ] T022 [US1] Preserve relevance ranking + token budget injection behavior with validated notes in internal/memory/retrieval.go

**Checkpoint**: US1 fully functional and independently testable, including explicit top-level metadata validation (`has_new_info`, `confidence`).

---

## Phase 4: User Story 2 - Persistent Context Across Sessions (Priority: P2)

**Goal**: Keep prompt/config/context behavior consistent across restarts with profile-local prompt files.

**Independent Test**: Restart app/profile and verify prompt files are reused, context/index state persists, and no prompt DB dependency exists.

### Tests for User Story 2

- [ ] T023 [P] [US2] Add integration test for prompt file persistence across restarts in tests/integration/prompt_management_test.go
- [ ] T024 [P] [US2] Add integration test for missing `system.md`/`extractor.md` auto-bootstrap + visible warning in tests/integration/prompt_management_test.go
- [ ] T025 [P] [US2] Add integration test verifying system/extractor prompts are loaded from files (not DB) in tests/integration/prompt_management_test.go
- [ ] T026 [P] [US2] Extend cache lifecycle restart test coverage for prompt-file changes in tests/integration/context_cache_lifecycle_test.go

### Implementation for User Story 2

- [ ] T027 [US2] Wire prompt loading to profile prompt files in runtime setup path in internal/app/app.go
- [ ] T028 [US2] Ensure settings save flow writes system prompt to `<profile>/prompts/system.md` in internal/tui/settings_menu.go
- [ ] T029 [US2] Ensure extractor prompt persistence uses `<profile>/prompts/extractor.md` in internal/config/settings.go
- [ ] T030 [US2] Emit visible warning when prompt files are missing and defaults are auto-created in internal/tui/footer_view.go
- [ ] T031 [US2] Invalidate context cache on system prompt save from file-backed flow in internal/cache/context.go

**Checkpoint**: US2 fully functional and independently testable.

---

## Phase 5: User Story 3 - Automatic Context Maintenance (Priority: P2)

**Goal**: Maintain vector index automatically while respecting validated extracted-note updates.

**Independent Test**: Add and update notes through extraction pipeline and verify incremental index sync, rebuild fallback, and deterministic retrieval fallback.

### Tests for User Story 3

- [ ] T032 [P] [US3] Extend integration test for incremental sync after `add|update` extraction actions in tests/integration/vector_sync_retrieval_test.go
- [ ] T033 [P] [US3] Extend integration test for compaction/rebuild non-blocking behavior in tests/integration/vector_rebuild_fallback_test.go
- [ ] T034 [P] [US3] Add integration test for deterministic retrieval fallback when index unavailable in tests/integration/vector_rebuild_fallback_test.go

### Implementation for User Story 3

- [ ] T035 [US3] Apply validated extraction actions to note store writes used by vector sync in internal/memory/store.go
- [ ] T036 [US3] Ensure vector sync handles update-target note mutations incrementally in internal/vector/sync.go
- [ ] T037 [US3] Ensure periodic rebuild/compaction triggers remain automatic and non-blocking in internal/vector/rebuild.go

**Checkpoint**: US3 fully functional and independently testable.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T038 [P] Update contract examples and validation notes in specs/005-memory-context/contracts/context-retrieval.md
- [ ] T039 [P] Update data model documentation for any implementation-level field adjustments in specs/005-memory-context/data-model.md
- [ ] T040 Run full verification (`go test ./...`, `make lint`, `make fmt`) and record outcomes in specs/005-memory-context/quickstart.md
- [ ] T041 Add explicit UX validation checklist for missing prompt-file warning copy/placement in specs/005-memory-context/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: starts immediately
- **Phase 2 (Foundational)**: depends on Phase 1, blocks all user stories
- **Phase 3 (US1)**: depends on Phase 2
- **Phase 4 (US2)**: depends on Phase 2; may run in parallel with US3 after Phase 2
- **Phase 5 (US3)**: depends on Phase 2; may run in parallel with US2 after Phase 2
- **Phase 6 (Polish)**: depends on all selected user stories

### User Story Dependencies

- **US1 (P1)**: no dependency on other stories after foundational work
- **US2 (P2)**: no hard dependency on US1; integrates same prompt/config infrastructure
- **US3 (P2)**: depends on foundational extraction-validation pipeline, not on US2 completion

### Within Each User Story

- Tests first (write and verify failing where applicable)
- Validation/data handling before integration wiring
- Story-specific integration and checkpoint validation last

---

## Parallel Execution Examples

### User Story 1

```bash
# Parallel tests
T011, T012, T013, T014, T015, T016

# Parallel implementation after schema types are in place
T018, T019, T021
```

### User Story 2

```bash
# Parallel tests
T023, T024, T025, T026

# Parallel implementation
T028, T029, T030
```

### User Story 3

```bash
# Parallel tests
T032, T033, T034
```

---

## Implementation Strategy

### MVP First (US1)

1. Complete Phase 1 + Phase 2
2. Complete Phase 3 (US1)
3. Validate US1 independently
4. Demo/deploy MVP increment

### Incremental Delivery

1. Deliver US1 (strict extraction contract + accurate retrieval)
2. Deliver US2 (profile prompt persistence across restarts)
3. Deliver US3 (automatic maintenance with validated updates)
4. Finish with Phase 6 polish and full-suite validation
