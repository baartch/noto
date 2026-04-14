# Tasks: note-extraction-strategy

**Input**: Design documents from `/specs/008-note-extraction-strategy/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Tests are required per constitution and plan gate; include unit and integration coverage for new behaviors.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Verify existing memory profile paths and storage files in internal/profiles/ and internal/memory/
- [ ] T002 [P] Capture current note extraction + storage flow references in internal/memory/ for baseline documentation
- [ ] T003 [P] Confirm TUI notification patterns in internal/ui/ for reuse in footer notifications

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

- [ ] T004 Define note value scoring inputs and thresholds in internal/memory/scoring.go
- [ ] T005 Define duplicate matching strategy using vector index in internal/vector/dedup.go
- [ ] T006 Create NoteCandidate evaluation helper in internal/memory/candidates.go
- [ ] T007 Add shared error handling/logging hooks for note capture in internal/memory/logging.go

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Capture valuable notes without duplicates (Priority: P1) 🎯 MVP

**Goal**: Extract note candidates, score them, and store only valuable unique notes with deduplication.

**Independent Test**: Run a chat session with repeated facts and verify only one stored note plus linked context.

### Tests for User Story 1

- [ ] T008 [P] [US1] Unit test for scoring threshold decisions in tests/unit/memory/scoring_test.go
- [ ] T009 [P] [US1] Integration test for deduplication with vector index in tests/integration/memory/dedup_test.go
- [ ] T010 [P] [US1] Unit test for candidate extraction inputs in tests/unit/memory/candidates_test.go

### Implementation for User Story 1

- [ ] T011 [P] [US1] Implement scoring logic in internal/memory/scoring.go
- [ ] T012 [P] [US1] Implement candidate extraction in internal/memory/candidates.go
- [ ] T013 [US1] Implement deduplication comparison in internal/vector/dedup.go
- [ ] T014 [US1] Implement note storage + link-to-existing behavior in internal/memory/store.go
- [ ] T015 [US1] Wire extraction → scoring → dedup → storage flow in internal/memory/processor.go

**Checkpoint**: User Story 1 fully functional and testable independently

---

## Phase 4: User Story 2 - Surface important notes during chat (Priority: P2)

**Goal**: Retrieve and rank relevant notes for each prompt.

**Independent Test**: With multiple notes stored, ask a related prompt and verify top relevant notes are surfaced.

### Tests for User Story 2

- [ ] T016 [P] [US2] Unit test for ranking relevance in tests/unit/memory/retrieval_test.go
- [ ] T017 [P] [US2] Integration test for retrieval pipeline in tests/integration/memory/retrieval_flow_test.go

### Implementation for User Story 2

- [ ] T018 [P] [US2] Implement relevance scoring in internal/memory/retrieval.go
- [ ] T019 [US2] Implement ranking + top-N selection in internal/memory/retrieval.go
- [ ] T020 [US2] Wire retrieval into chat prompt handling in internal/memory/context.go

**Checkpoint**: User Story 2 functional and testable independently

---

## Phase 5: User Story 3 - Review and manage captured notes (Priority: P3)

**Goal**: Provide footer notifications for stored notes and a review surface with rationale.

**Independent Test**: Store a note and verify footer notification appears for ~3 seconds; list notes with rationale.

### Tests for User Story 3

- [ ] T021 [P] [US3] Integration test for footer notification timing in tests/integration/ui/footer_note_test.go
- [ ] T022 [P] [US3] Integration test for note review listing in tests/integration/memory/review_test.go

### Implementation for User Story 3

- [ ] T023 [P] [US3] Implement footer notification display in internal/ui/footer.go
- [ ] T024 [US3] Trigger footer notifications when notes are stored in internal/memory/processor.go
- [ ] T025 [US3] Implement note review listing in internal/memory/review.go
- [ ] T026 [US3] Add storage rationale metadata to review output in internal/memory/review.go

**Checkpoint**: User Story 3 functional and testable independently

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T027 [P] Run quickstart.md validation steps and capture any deltas in specs/008-note-extraction-strategy/quickstart.md
- [ ] T028 [P] Update docs/ or README.md with note extraction and footer notification behavior
- [ ] T029 Run gofmt/go vet/make lint for feature scope and resolve issues

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

- Setup tasks T002, T003 can run in parallel
- US1 unit/integration tests (T008-T010) can run in parallel
- US2 tests (T016-T017) can run in parallel
- US3 tests (T021-T022) can run in parallel
- Implementation tasks T011, T012 can run in parallel

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
