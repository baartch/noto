# Tasks: Settings Dialog Navigation – Profile Management

**Input**: Design documents from `/specs/006-settings-dialog/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/, quickstart.md

**Tests**: Test tasks are REQUIRED for every user story and must be created before implementation tasks.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Phase 1: Setup (Shared Infrastructure)

- [ ] T001 Review existing settings dialog and profile command registry to confirm available handlers (internal/tui/model.go, internal/commands, internal/profile)

---

## Phase 2: Foundational (Blocking Prerequisites)

- [ ] T002 Add Profiles submenu entries in internal/tui/settings_menu.go (profile_select, profile_create, profile_rename, profile_delete)
- [ ] T003 Add settings action mapping for profile commands in internal/tui/model.go (wire Enter to profile actions)
- [ ] T004 Add prompt/confirmation plumbing for settings-driven profile actions in internal/tui/model.go (reuse existing confirmation patterns)

**Checkpoint**: Profile submenu entries and command wiring ready.

---

## Phase 3: User Story 1 - Open Settings (Priority: P1) 🎯 MVP

**Goal**: Open settings dialog with Ctrl+J and show sorted entries (including profiles submenu).

**Independent Test**: Press Ctrl+J; verify settings dialog opens and entries are sorted alphabetically.

### Tests for User Story 1 (REQUIRED) ⚠️

- [ ] T005 [P] [US1] Update integration test for settings open/sorted list in tests/integration/tui_flow_regression_test.go

### Implementation for User Story 1

- [ ] T006 [US1] Ensure profiles submenu renders in settings list in internal/tui/model.go (sorted order with new entries)

**Checkpoint**: Settings dialog opens with profiles submenu visible and sorted.

---

## Phase 4: User Story 2 - Edit Settings Values (Priority: P1)

**Goal**: Keep value editing flow intact while profiles submenu exists.

**Independent Test**: Edit a value entry and save; ensure it persists and list updates immediately.

### Tests for User Story 2 (REQUIRED) ⚠️

- [ ] T007 [P] [US2] Update integration test coverage to include value edit flow after profile submenu is added (tests/integration/tui_flow_regression_test.go)

### Implementation for User Story 2

- [ ] T008 [US2] Ensure settings editor flow ignores profile actions and continues to work with new submenu entries (internal/tui/model.go)

**Checkpoint**: Value editing still works with profile submenu present.

---

## Phase 5: User Story 3 - Navigate Submenus (Priority: P2)

**Goal**: Navigate into Profiles submenu and back via Esc.

**Independent Test**: Enter Profiles submenu, press Esc to return, Esc again to close.

### Tests for User Story 3 (REQUIRED) ⚠️

- [ ] T009 [P] [US3] Add integration test for Profiles submenu navigation and Esc behavior in tests/integration/tui_flow_regression_test.go

### Implementation for User Story 3

- [ ] T010 [US3] Ensure submenu navigation works for Profiles in internal/tui/model.go (Esc to parent/root)

**Checkpoint**: Profiles submenu navigates correctly.

---

## Phase 6: User Story 4 - Manage Profiles (Priority: P1)

**Goal**: Allow select/create/rename/delete actions from settings.

**Independent Test**: Execute each profile action from settings and verify UI + storage updates.

### Tests for User Story 4 (REQUIRED) ⚠️

- [ ] T011 [P] [US4] Integration test for profile select/create/rename/delete flows from settings in tests/integration/tui_profile_management_test.go
- [ ] T012 [P] [US4] Integration test for delete confirmation flow in tests/integration/tui_profile_management_test.go

### Implementation for User Story 4

- [ ] T013 [US4] Wire Profile Select action to existing profile picker command in internal/tui/model.go
- [ ] T014 [US4] Wire Profile Create/Rename prompts to profile service in internal/tui/model.go
- [ ] T015 [US4] Wire Profile Delete action to confirmation flow and service in internal/tui/model.go
- [ ] T016 [US4] Ensure profile switch refreshes settings values and closes submenu after action (internal/tui/model.go)

**Checkpoint**: Profile actions work end-to-end from settings.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T017 Run go test ./... and make lint; record results in specs/006-settings-dialog/quickstart.md
- [ ] T018 Validate quickstart steps and manual performance check (<1s open) in specs/006-settings-dialog/quickstart.md

---

## Dependencies & Execution Order

- Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6 → Phase 7
- US1 (open settings) before US2/US3/US4 due to shared UI.
- US4 depends on Profile submenu entries from Phase 2.

## Parallel Execution Examples

- US1: T005 can run in parallel with T006.
- US3: T009 (test) can run in parallel with T010 (implementation) after Phase 2.
- US4: T011/T012 tests can run in parallel; T013–T016 sequential due to shared handler logic.

## Implementation Strategy

Deliver MVP by completing US1 (open settings + profiles submenu visible). Then add US3 submenu navigation, and finally US4 profile management actions.
