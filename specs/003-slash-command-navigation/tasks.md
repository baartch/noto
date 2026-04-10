# Tasks: Slash Command Navigation

**Input**: Design documents from `/specs/003-slash-command-navigation/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED for every user story and must be created before implementation tasks.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Phase 1: Setup (Shared Infrastructure)

- [x] T001 Capture current slash suggestion behavior notes in specs/003-slash-command-navigation/research.md (confirm no changes needed)
- [x] T002 [P] Review existing suggestion rendering in internal/tui/model.go and picker windowing in internal/tui/picker.go to confirm reuse strategy

---

## Phase 2: Foundational (Blocking Prerequisites)

- [x] T003 Define shared windowing helper or render strategy for suggestions in internal/tui/model.go (document approach inline)

---

## Phase 3: User Story 1 - Discover Slash Commands (Priority: P1) 🎯 MVP

**Goal**: Show the slash command suggestion list when the input starts with `/` and hide it when it no longer does.

**Independent Test**: Type `/` and see the list; remove `/` and see it disappear.

### Tests for User Story 1 (REQUIRED)

- [x] T004 [P] [US1] Add unit test coverage for suggestion visibility toggling in internal/tui/suggestions_test.go

### Implementation for User Story 1

- [x] T005 [US1] Ensure suggestion visibility logic remains correct while adding windowing in internal/tui/model.go

---

## Phase 4: User Story 2 - Filter Slash Commands (Priority: P2)

**Goal**: Filter the suggestion list as the user types after `/`.

**Independent Test**: Type `/pro` and verify the list shows only matching commands.

### Tests for User Story 2 (REQUIRED)

- [x] T006 [P] [US2] Add unit test coverage for suggestion filtering updates in internal/tui/suggestions_test.go

### Implementation for User Story 2

- [x] T007 [US2] Preserve filtering behavior while adding windowed rendering in internal/tui/model.go

---

## Phase 5: User Story 3 - Navigate and Execute Commands (Priority: P3)

**Goal**: Navigate the entire suggestion list with Up/Down, scroll the window as needed, and support Tab/Enter actions.

**Independent Test**: With a long list, use Up/Down to traverse beyond visible items and confirm the window scrolls; press Tab to autofill; press Enter to execute.

### Tests for User Story 3 (REQUIRED)

- [x] T008 [P] [US3] Add unit test coverage for suggestion cursor windowing logic in internal/tui/suggestions_test.go
- [x] T009 [P] [US3] Add integration test for end-to-end suggestion navigation in internal/tui/suggestions_integration_test.go

### Implementation for User Story 3

- [x] T010 [US3] Implement windowed rendering for suggestions in internal/tui/model.go (render only visible subset, keep cursor visible)
- [x] T011 [US3] Ensure Up/Down updates cursor across full list and triggers window scroll in internal/tui/model.go
- [x] T012 [US3] Implement Tab autofill behavior for current suggestion in internal/tui/model.go
- [x] T013 [US3] Confirm Enter executes selected suggestion without regression in internal/tui/model.go

---

## Phase 6: Polish & Cross-Cutting Concerns

- [x] T014 [P] Update quickstart validation notes in specs/003-slash-command-navigation/quickstart.md
- [ ] T015 Run manual quickstart in specs/003-slash-command-navigation/quickstart.md and record results in the file
- [ ] T016 Validate performance target (p95 suggestion refresh < 50 ms) and document check in specs/003-slash-command-navigation/research.md
- [x] T017 Ensure lint/test gates pass for all modified files

---

## Dependencies & Execution Order

- **Phase 1** → **Phase 2** → **User Stories (P1 → P2 → P3)** → **Polish**
- User stories depend on the foundational windowing strategy (T003).
- Tests for each story must be written before implementation tasks for that story.

## Parallel Opportunities

- T002 can run in parallel with T001.
- Tests within each story (T004, T006, T008, T009) can run in parallel.
- T014 and T016 can run in parallel during Polish.

## Implementation Strategy

- **MVP**: Deliver User Story 1 first (visibility), then User Story 2 (filtering), then User Story 3 (full navigation + scroll + actions).
- **Incremental delivery**: Each story should remain independently testable and shippable.
