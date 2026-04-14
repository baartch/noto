# Tasks: Settings Dialog Navigation (Profiles List)

**Generated**: 2026-04-12
**Source**: /home/andy/gitrepos/noto/specs/006-settings-dialog/spec.md

## Phase 1: Setup

- [X] T001 Review existing profile slash commands and settings menu/profile list code paths (internal/commands/profile_commands.go, internal/tui/model.go, internal/tui/settings_menu.go)

## Phase 2: Foundational

- [X] T002 Remove profile command registration from command registry and slash dispatch pathways (internal/commands/registry.go, internal/chat/slash_dispatch.go)
- [X] T003 Remove profile slash command handlers and related tests (internal/commands/profile_commands.go, tests/integration/tui_profile_management_test.go)
- [X] T004 Ensure profile service CRUD can be invoked directly from settings model (internal/profile/service.go, internal/store/profile_repo.go)

## Phase 3: User Story 1 - Open Settings (P1)

**Goal**: Users can open settings with Ctrl+J and see entries sorted alphabetically.

**Independent Test**: Press Ctrl+J and confirm settings list opens and remains sorted.

- [X] T005 [US1] Verify settings open flow still works after profile command removal (internal/tui/model.go)
- [X] T006 [US1] Update settings help text if it referenced profile slash commands (internal/tui/styles.go, internal/tui/model.go)

## Phase 4: User Story 2 - Edit Settings Values (P1)

**Goal**: Users can edit settings values via the textarea editor and save or cancel.

**Independent Test**: Edit a value, save, confirm persistence, cancel and verify original value remains.

- [X] T007 [US2] Ensure settings editor flows remain intact after profile changes (internal/tui/model.go)

## Phase 5: User Story 4 - Manage Profiles (P1)

**Goal**: Users can manage profiles from a single list with Enter/Ctrl+N/Ctrl+R/Ctrl+D, no slash commands.

**Independent Test**: Open Profiles list, switch profile with Enter, create/rename with Ctrl+N/Ctrl+R and Enter, delete with Ctrl+D.

- [X] T008 [US4] Replace Profiles submenu action entries with a profiles list view (internal/tui/settings_menu.go, internal/tui/model.go)
- [X] T009 [US4] Implement Enter to switch profile directly via profile service and update UI (internal/tui/model.go, internal/profile/service.go)
- [X] T010 [US4] Implement Ctrl+N create flow using settings textarea, then create via profile service (internal/tui/model.go)
- [X] T011 [US4] Implement Ctrl+R rename flow using settings textarea, then rename via profile service (internal/tui/model.go)
- [X] T012 [US4] Implement Ctrl+D delete flow using profile service; handle last-profile error (internal/tui/model.go, internal/profile/service.go)
- [X] T013 [US4] Add keybinding hints in the Profiles list view (internal/tui/model.go, internal/tui/styles.go)
- [X] T020 [US1] Highlight current list entries with dot indicator + color (internal/tui/model.go, internal/tui/settings_menu.go)
- [X] T014 [US4] Update profile management integration tests for list-based workflow (tests/integration/tui_profile_management_test.go)

## Phase 6: User Story 3 - Navigate Submenus (P2)

**Goal**: Users can enter submenus and return with Esc.

**Independent Test**: Enter Provider submenu, press Esc, return to parent list.

- [X] T015 [US3] Confirm submenu navigation still works with Profiles list changes (internal/tui/model.go)

## Phase 7: Polish & Cross-Cutting

- [X] T016 [P] Run gofmt on modified files (internal/tui/model.go, internal/tui/settings_menu.go, internal/commands/profile_commands.go, internal/commands/registry.go)
- [X] T017 Run go test ./... and update quickstart build status (specs/006-settings-dialog/quickstart.md)
- [X] T018 Run make lint and update quickstart build status (specs/006-settings-dialog/quickstart.md)
- [X] T019 Update AGENTS.md if new tech/context added (./.specify/scripts/bash/update-agent-context.sh pi)

## Dependencies

- US1 → US2 → US4 → US3

## Parallel Execution Examples

- T002 (remove registry wiring) can run in parallel with T004 (ensure service direct use).
- T008 (profiles list view) can run in parallel with T014 (update tests) once list structure is defined.
- T016 (gofmt) can run after implementation tasks complete.

## Implementation Strategy

- Start by removing profile slash command registration and handlers to avoid relying on the command registry.
- Replace Profiles submenu action entries with a dedicated profiles list view and hook Enter/Ctrl+N/Ctrl+R/Ctrl+D to profile service operations.
- Add keybinding hints in the Profiles view to keep UX clear.
- Update integration tests to match the new list-based workflow.
- Finish with formatting, tests, linting, and quickstart updates.
