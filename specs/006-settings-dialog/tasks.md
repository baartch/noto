# Tasks: Settings Dialog Navigation

**Input**: Design documents from `/specs/006-settings-dialog/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, quickstart.md

**Tests**: Test tasks are REQUIRED for every user story and must be created before implementation tasks.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Phase 1: Setup (Shared Infrastructure)

- [X] T001 Review existing TUI keybindings/help and picker patterns in internal/tui/model.go
- [X] T002 Review settings-related storage for profile metadata and provider/system prompt data in internal/profile/ and internal/store/

---

## Phase 2: Foundational (Blocking Prerequisites)

- [X] T003 Define settings menu structure and entry mapping in internal/tui/settings_menu.go (new)
- [X] T004 Add system prompt persistence in SQLite via internal/store/system_prompt_repo.go (no migration; start with default prompt "You are Noto. A buddy who takes notes." for existing/new profiles when missing)

**Checkpoint**: Settings menu structure and storage layer ready.

---

## Phase 3: User Story 1 - Open Settings (Priority: P1) 🎯 MVP

**Goal**: Open settings dialog with Ctrl+, and show alphabetically sorted entries.

**Independent Test**: Press Ctrl+, verify dialog opens and entries are sorted alphabetically.

### Tests for User Story 1 (REQUIRED) ⚠️

- [X] T005 [P] [US1] Add integration test for Ctrl+, opening settings in tests/integration/tui_flow_regression_test.go
- [X] T006 [P] [US1] Add integration test for alphabetical ordering in tests/integration/tui_flow_regression_test.go

### Implementation for User Story 1

- [X] T007 [US1] Add Ctrl+, keybinding and help hint in internal/tui/model.go
- [X] T008 [US1] Render settings dialog list overlay in internal/tui/model.go
- [X] T009 [US1] Populate settings menu entries in internal/tui/settings_menu.go

**Checkpoint**: Settings dialog opens and lists sorted entries.

---

## Phase 4: User Story 2 - Edit Settings Values (Priority: P1)

**Goal**: Edit text/number settings via textarea; Enter saves, Esc cancels; numeric values validated.

**Independent Test**: Edit token budget and system prompt; verify save/cancel and numeric validation error.

### Tests for User Story 2 (REQUIRED) ⚠️

- [ ] T010 [P] [US2] Add integration test for textarea editing save/cancel in tests/integration/tui_flow_regression_test.go
- [ ] T011 [P] [US2] Add integration test for numeric validation error in tests/integration/tui_flow_regression_test.go

### Implementation for User Story 2

- [ ] T012 [US2] Add textarea editor flow for settings values in internal/tui/model.go
- [ ] T013 [US2] Implement numeric validation and error state in internal/tui/model.go
- [ ] T013a [US2] Define numeric validation error messaging/state in internal/tui/model.go
- [ ] T014 [US2] Persist token budget updates in internal/profile/settings.go
- [ ] T015 [US2] Store system prompt in SQLite via internal/store/system_prompt_repo.go (new) and wire save/load in internal/profile/

**Checkpoint**: Editing values works with validation and persistence.

---

## Phase 5: User Story 3 - Navigate Submenus (Priority: P2)

**Goal**: Enter submenus for grouped settings, Esc navigates up, Esc closes at top level.

**Independent Test**: Enter provider configuration submenu; Esc returns to top-level; Esc closes dialog at root.

### Tests for User Story 3 (REQUIRED) ⚠️

- [ ] T016 [P] [US3] Add integration test for submenu navigation and Esc behavior in tests/integration/tui_flow_regression_test.go

### Implementation for User Story 3

- [ ] T017 [US3] Implement submenu navigation stack in internal/tui/settings_menu.go
- [ ] T018 [US3] Wire Esc behavior for submenu/back/close in internal/tui/model.go
- [ ] T019 [US3] Add provider configuration submenu entries in internal/tui/settings_menu.go

**Checkpoint**: Submenus navigate correctly and Esc behaves per spec.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T020 [P] Run go test ./... and make lint; capture results in specs/006-settings-dialog/quickstart.md
- [ ] T021 Validate quickstart steps and update specs/006-settings-dialog/quickstart.md (manual performance measurement acceptable)

---

## Dependencies & Execution Order

- **Phase 1** → **Phase 2** → **Phase 3** → **Phase 4** → **Phase 5** → **Phase 6**
- Tests must be completed before implementation tasks in each story phase.

## Parallel Example: User Story 2

- T010 and T011 can run in parallel (tests).
- T012 and T013 should be sequential (same file). T014/T015 can run after T012.
