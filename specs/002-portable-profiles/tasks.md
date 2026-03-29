# Tasks: Portable Profiles

**Input**: Design documents from `/specs/002-portable-profiles/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED for every user story and must be created before implementation tasks.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [x] T001 Review existing profile metadata usage in internal/profile and internal/store (notes in specs/002-portable-profiles/research.md)
- [x] T002 [P] Inventory global DB profile table usage in internal/app, internal/store, internal/commands
- [x] T003 [P] Confirm profile directory layout in internal/config and document in specs/002-portable-profiles/research.md

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

- [x] T004 Define profile metadata file format in internal/profile/metadata.go
- [x] T005 [P] Add profile metadata read/write helpers in internal/profile/metadata.go
- [x] T006 [P] Add active profile selection config file helpers in internal/config/active_profile.go
- [x] T007 Update profile discovery to scan profile directories in internal/profile/discovery.go
- [x] T008 Update CLI/TUI profile listing to use discovery helpers in internal/commands/profile_commands.go and internal/app/profile_cmd.go

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Profile Data Stays Local (Priority: P1) 🎯 MVP

**Goal**: Profile metadata lives in the profile directory and travels with the profile.

**Independent Test**: Move a profile directory to a fresh instance and confirm it loads without global DB metadata.

### Tests for User Story 1 (REQUIRED) ⚠️

- [x] T009 [P] [US1] Integration test for portable profile load in tests/integration/profile_portability_test.go
- [x] T010 [P] [US1] Integration test for metadata file creation in tests/integration/profile_metadata_test.go

### Implementation for User Story 1

- [x] T011 [P] [US1] Persist metadata file on profile create in internal/profile/service.go
- [x] T012 [US1] Update profile rename to update metadata file in internal/profile/service.go
- [x] T013 [US1] Update profile delete to remove metadata file in internal/profile/service.go
- [x] T014 [US1] Ensure prompt paths in metadata align with internal/profile/prompt_store.go

**Checkpoint**: User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - Global DB Excludes Profile Metadata (Priority: P2)

**Goal**: The global database no longer stores profile metadata or selection state.

**Independent Test**: Create/update/delete profiles and confirm global DB has no profile records.

### Tests for User Story 2 (REQUIRED) ⚠️

- [x] T015 [P] [US2] Integration test for global DB exclusion in tests/integration/profile_global_db_test.go

### Implementation for User Story 2

- [x] T016 [US2] Remove global profile table usage in internal/store/profile_repo.go and internal/app/db.go
- [x] T017 [US2] Remove global DB migrations (legacy removed)
- [x] T018 [US2] Update profile selection paths to avoid global DB in internal/profile/service.go

**Checkpoint**: User Story 2 should be fully functional and testable independently

---

## Phase 5: User Story 3 - Multiple Profiles Remain Usable (Priority: P3)

**Goal**: Profile listing and selection remain fully functional with directory scanning and local selection storage.

**Independent Test**: Create multiple profile directories and verify list/select uses slug disambiguation.

### Tests for User Story 3 (REQUIRED) ⚠️

- [x] T019 [P] [US3] Integration test for profile listing via directory scan in tests/integration/profile_discovery_test.go
- [x] T020 [P] [US3] Integration test for duplicate names disambiguation in tests/integration/profile_duplicate_names_test.go

### Implementation for User Story 3

- [x] T021 [US3] Implement directory scan listing in internal/profile/discovery.go
- [x] T022 [US3] Use slug disambiguation in listing/selection in internal/commands/profile_commands.go
- [x] T023 [US3] Persist active profile selection to local config in internal/profile/service.go and internal/config/active_profile.go

**Checkpoint**: User Story 3 should be fully functional and testable independently

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [x] T024 [P] Update quickstart docs in specs/002-portable-profiles/quickstart.md
- [x] T025 Validate UX messaging consistency in internal/commands/profile_commands.go and internal/app/profile_cmd.go
- [x] T026 Performance check for directory scan in tests/integration/profile_discovery_bench_test.go
- [x] T027 Run quickstart.md validation steps manually and note results in specs/002-portable-profiles/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - depends on US1 metadata helpers
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - depends on discovery helpers and active profile config

### Parallel Opportunities

- T002, T003 can run in parallel during Setup
- T004-T006 can run in parallel after initial format decision
- US1 test tasks (T009, T010) can run in parallel
- US3 test tasks (T019, T020) can run in parallel

---

## Parallel Example: User Story 1

- Parallel set A: T009 (portable profile load test)
- Parallel set B: T010 (metadata file creation test)
- After tests: T011-T014 in sequence

## Parallel Example: User Story 3

- Parallel set A: T019 (discovery test)
- Parallel set B: T020 (duplicate names test)
- After tests: T021-T023 in sequence

---

## Implementation Strategy

- Deliver MVP by completing Foundational + User Story 1.
- Remove global DB metadata (User Story 2) immediately after MVP validation.
- Complete multi-profile enhancements (User Story 3) and polish tasks.
