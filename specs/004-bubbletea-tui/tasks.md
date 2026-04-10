# Tasks: Bubble Tea TUI Standard

**Input**: Design documents from `/specs/004-bubbletea-tui/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED for every user story and must be created before implementation tasks.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Phase 1: Setup (Shared Infrastructure)

- [x] T001 Inventory TUI flows and entry points in specs/004-bubbletea-tui/research.md
- [x] T002 [P] Catalog existing Bubble Tea models and Bubbles usage in specs/004-bubbletea-tui/research.md
- [x] T003 [P] Identify current Lip Gloss style definitions and gaps in specs/004-bubbletea-tui/research.md

---

## Phase 2: Foundational (Blocking Prerequisites)

- [x] T004 Define a shared Lip Gloss style registry in internal/tui/styles.go
- [x] T005 Document TUI flow inventory and component usage in specs/004-bubbletea-tui/data-model.md

---

## Phase 3: User Story 1 - Consistent Bubble Tea TUI Usage (Priority: P1) 🎯 MVP

**Goal**: Ensure all existing TUI flows are implemented as Bubble Tea models with update/view loops.

**Independent Test**: Verify every TUI entry point maps to a Bubble Tea model and runs without regressions.

### Tests for User Story 1 (REQUIRED)

- [ ] T006 [P] [US1] Add regression tests for refactored TUI flows in tests/integration/tui_flow_regression_test.go

### Implementation for User Story 1

- [ ] T007 [US1] Refactor any non-Bubble Tea TUI flows to Bubble Tea models in internal/tui/
- [ ] T008 [US1] Update TUI entry points to use Bubble Tea models in internal/app/ and internal/tui/

---

## Phase 4: User Story 2 - Prefer Bubbles Components (Priority: P2)

**Goal**: Replace custom UI components with Bubbles equivalents where suitable.

**Independent Test**: Review UI components to confirm Bubbles usage or documented rationale.

### Tests for User Story 2 (REQUIRED)

- [ ] T009 [P] [US2] Add integration tests validating picker/input behavior in tests/integration/tui_bubbles_usage_test.go

### Implementation for User Story 2

- [ ] T010 [US2] Replace custom list/input UI with Bubbles components in internal/tui/
- [ ] T011 [US2] Document any remaining custom UI rationale in internal/tui/ (code comments or doc blocks)

---

## Phase 5: User Story 3 - Lip Gloss Styling Definitions (Priority: P3)

**Goal**: Consolidate styling into reusable Lip Gloss definitions and apply them consistently.

**Independent Test**: Confirm all styled TUI elements reference shared Lip Gloss styles.

### Tests for User Story 3 (REQUIRED)

- [x] T012 [P] [US3] Add unit tests covering shared style usage in internal/tui/styles_test.go

### Implementation for User Story 3

- [x] T013 [US3] Implement shared style definitions in internal/tui/styles.go
- [x] T014 [US3] Update TUI components to use shared Lip Gloss styles in internal/tui/

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T015 [P] Update specs/004-bubbletea-tui/quickstart.md with manual validation notes
- [ ] T016 Run quickstart validation and record results in specs/004-bubbletea-tui/quickstart.md
- [ ] T017 Validate performance sanity check and document in specs/004-bubbletea-tui/research.md
- [x] T018 Ensure lint/test gates pass for all modified files

---

## Dependencies & Execution Order

- **Phase 1** → **Phase 2** → **User Stories (P1 → P2 → P3)** → **Polish**
- Story phases depend on the shared style registry and inventory (T004–T005).

## Parallel Opportunities

- T002 and T003 can run in parallel.
- Tests within each story phase (T006, T009, T012) can run in parallel.
- T015 and T017 can run in parallel during Polish.

## Implementation Strategy

- **MVP**: Deliver User Story 1 first (Bubble Tea compliance), then User Story 2 (Bubbles usage), then User Story 3 (Lip Gloss styles).
- **Incremental delivery**: Each story should remain independently testable and shippable.
