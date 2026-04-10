# Tasks: Bubble Tea TUI Standard

**Input**: Design documents from `/specs/004-bubbletea-tui/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED for every user story and must be created before implementation tasks.

## Phase 1: Setup (Shared Infrastructure)

- [x] T001 Review current TUI layout and help usage in internal/tui/model.go

---

## Phase 2: Foundational (Blocking Prerequisites)

- [x] T002 Define help/keybinding UX expectations for footer + expanded help in specs/004-bubbletea-tui/quickstart.md

---

## Phase 3: User Story 1 - Consistent Bubble Tea TUI Usage (Priority: P1) 🎯 MVP

**Goal**: Ensure TUI interactions use Bubble Tea conventions with anchored input/footer and help rendering.

**Independent Test**: Launch TUI, toggle help, open pickers, and verify footer/input anchoring plus keybinding display.

### Tests for User Story 1 (REQUIRED) ⚠️

- [x] T003 [P] [US1] Add integration coverage for help placement and footer help in tests/integration/tui_flow_regression_test.go
- [x] T004 [P] [US1] Add integration coverage for Ctrl+D and Ctrl+L bindings in tests/integration/tui_flow_regression_test.go

### Implementation for User Story 1

- [x] T005 [US1] Add Help component state and keymaps in internal/tui/model.go
- [x] T006 [US1] Render footer help via Bubbles Help in internal/tui/model.go
- [x] T007 [US1] Render expanded help above input textarea in internal/tui/model.go
- [x] T008 [US1] Update styles for help placement in internal/tui/styles.go

**Checkpoint**: Help renders in footer and above input without shifting anchored elements.

---

## Phase 4: User Story 2 - Prefer Bubbles Components (Priority: P2)

**Goal**: Use Bubbles components for help and keybinding behavior.

**Independent Test**: Verify help component is used for footer/expanded help and keybindings match spec.

### Tests for User Story 2 (REQUIRED) ⚠️

- [x] T009 [P] [US2] Add integration assertion for Bubbles Help usage in tests/integration/tui_bubbles_usage_test.go

### Implementation for User Story 2

- [x] T010 [US2] Wire help bindings and toggles with Bubbles key/help in internal/tui/model.go
- [x] T011 [US2] Update help-related documentation in internal/tui/doc.go

**Checkpoint**: Help component usage is documented and verified.

---

## Phase 5: Polish & Cross-Cutting Concerns

- [x] T012 [P] Run go test ./... and golangci-lint run; capture results in specs/004-bubbletea-tui/quickstart.md
- [x] T013 Validate UX anchoring with help/picker overlays and update specs/004-bubbletea-tui/quickstart.md

---

## Dependencies & Execution Order

- **Phase 1** → **Phase 2** → **Phase 3** → **Phase 4** → **Phase 5**
- Tests (T003–T004, T009) must be completed before implementation tasks in their respective stories.
