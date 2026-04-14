# Tasks: note-extraction-strategy

**Input**: Design documents from `/specs/008-note-extraction-strategy/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Tests are required per constitution and plan gate; include unit and integration coverage for new behaviors.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [X] T001 Verify memory profile paths and storage files in internal/config/ and internal/store/
- [X] T002 [P] Capture current note extraction + storage flow references in internal/memory/ and internal/store/
- [X] T003 [P] Confirm TUI notification patterns in internal/tui/ for reuse in footer notifications
- [X] T004 [P] Capture provider base endpoint + fixed suffixes (/responses, /embeddings/models, /embeddings) and deprecate /chat/completions in internal/provider/

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

- [X] T005 Define note value scoring inputs and thresholds in internal/memory/scoring.go
- [X] T006 Define duplicate matching strategy using vector index in internal/vector/dedup.go
- [X] T007 Create NoteCandidate evaluation helper in internal/memory/candidates.go
- [X] T008 Add shared error handling/logging hooks for note capture in internal/memory/logging.go
- [X] T009 Add profile setting for embeddings model selection in internal/profile/settings.go

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Capture valuable notes without duplicates (Priority: P1) 🎯 MVP

**Goal**: Extract note candidates, score them, and store only valuable unique notes with deduplication.

**Independent Test**: Run a chat session with repeated facts and verify only one stored note plus linked context.

### Tests for User Story 1

- [X] T010 [P] [US1] Unit test for scoring threshold decisions in tests/unit/memory/scoring_test.go
- [X] T011 [P] [US1] Integration test for deduplication with vector index in tests/integration/memory/dedup_test.go
- [X] T012 [P] [US1] Unit test for candidate extraction inputs in tests/unit/memory/candidates_test.go

### Implementation for User Story 1

- [X] T013 [P] [US1] Implement scoring logic in internal/memory/scoring.go
- [X] T014 [P] [US1] Implement candidate extraction in internal/memory/candidates.go
- [X] T015 [US1] Implement deduplication comparison in internal/vector/dedup.go
- [X] T016 [US1] Implement note storage + link-to-existing behavior in internal/memory/store.go
- [X] T017 [US1] Wire extraction → scoring → dedup → storage flow in internal/memory/processor.go
- [X] T018 [US1] Require embeddings model selection before vector sync in internal/chat/session.go
- [X] T019 [US1] Show footer warning when embeddings model is missing in internal/tui/footer_view.go

**Checkpoint**: User Story 1 fully functional and testable independently

---

## Phase 4: User Story 2 - Surface important notes during chat (Priority: P2)

**Goal**: Retrieve and rank relevant notes for each prompt.

**Independent Test**: With multiple notes stored, ask a related prompt and verify top relevant notes are surfaced.

### Tests for User Story 2

- [ ] T020 [P] [US2] Unit test for ranking relevance in tests/unit/memory/retrieval_test.go
- [ ] T021 [P] [US2] Integration test for retrieval pipeline in tests/integration/memory/retrieval_flow_test.go
- [ ] T022 [P] [US2] Integration test for embeddings model requirement in tests/integration/memory/embed_model_gate_test.go

### Implementation for User Story 2

- [ ] T023 [P] [US2] Implement relevance scoring in internal/memory/retrieval.go
- [ ] T024 [US2] Implement ranking + top-N selection in internal/memory/retrieval.go
- [ ] T025 [US2] Wire retrieval into chat prompt handling in internal/memory/context.go
- [ ] T026 [US2] Use embeddings model setting for retrieval in internal/chat/session.go

**Checkpoint**: User Story 2 functional and testable independently

---

## Phase 5: User Story 3 - Review and manage captured notes (Priority: P3)

**Goal**: Provide footer notifications for stored notes and a review surface with rationale.

**Independent Test**: Store a note and verify footer notification appears for ~3 seconds; list notes with rationale.

### Tests for User Story 3

- [ ] T027 [P] [US3] Integration test for footer notification timing in tests/integration/ui/footer_note_test.go
- [ ] T028 [P] [US3] Integration test for note review listing in tests/integration/memory/review_test.go
- [ ] T029 [P] [US3] Integration test for embeddings model selector in tests/integration/tui/embed_model_picker_test.go

### Implementation for User Story 3

- [ ] T030 [P] [US3] Add embeddings model entry in settings menu in internal/tui/settings_menu.go
- [ ] T031 [US3] Implement embeddings model picker using /embeddings/models endpoint in internal/tui/model.go
- [ ] T032 [US3] Add provider call to list embeddings models at baseURL+/embeddings/models in internal/provider/openai_compatible.go
- [ ] T033 [US3] Persist embeddings model selection in internal/profile/settings.go
- [ ] T034 [US3] Show embeddings model selection state in settings list in internal/tui/settings_menu.go
- [ ] T035 [US3] Update embeddings request to use baseURL+/embeddings with selected model in internal/provider/openai_compatible.go
- [ ] T036 [US3] Replace chat/completions calls with Responses API at baseURL+/responses in internal/provider/openai_compatible.go
- [ ] T037 [US3] Map Responses API request/response payloads in internal/provider/openai_compatible.go

**Checkpoint**: User Story 3 functional and testable independently

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T038 [P] Run quickstart.md validation steps and capture any deltas in specs/008-note-extraction-strategy/quickstart.md
- [ ] T039 [P] Verify footer timing/placement matches existing TUI notification patterns in internal/tui/
- [ ] T040 [P] Update docs/ or README.md with note extraction, embeddings model selector, and Responses API behavior
- [ ] T041 [P] Measure retrieval latency against 2s target in tests/integration/memory/retrieval_perf_test.go
- [ ] T042 Run gofmt/go vet/make lint for feature scope and resolve issues

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational - no dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational - integrates with US1 data
- **User Story 3 (P3)**: Can start after Foundational - uses US1 storage and US2 context

### Parallel Opportunities

- Setup tasks T002-T004 can run in parallel
- US1 unit/integration tests (T010-T012) can run in parallel
- US2 tests (T020-T022) can run in parallel
- US3 tests (T027-T029) can run in parallel
- Implementation tasks T013, T014 can run in parallel

---

## Parallel Example: User Story 1

```bash
Task: "Unit test for scoring threshold decisions in tests/unit/memory/scoring_test.go"
Task: "Unit test for candidate extraction inputs in tests/unit/memory/candidates_test.go"
Task: "Implement scoring logic in internal/memory/scoring.go"
Task: "Implement candidate extraction in internal/memory/candidates.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Run US1 tests and manual chat verification

### Incremental Delivery

1. Foundation ready
2. Add User Story 1 → Test independently → Demo
3. Add User Story 2 → Test independently → Demo
4. Add User Story 3 → Test independently → Demo

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
