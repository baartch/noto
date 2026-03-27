# Tasks: Noto Profile Memory CLI

**Input**: Design documents from `/specs/001-build-profile-memory-cli/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED for every user story and must be created before implementation tasks.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Project paths used here follow the plan structure: `cmd/`, `internal/`, `tests/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create Go module and entrypoint scaffold in `go.mod` and `cmd/noto/main.go`
- [ ] T002 Create package directories per plan in `internal/app`, `internal/tui`, `internal/profile`, `internal/chat`, `internal/provider`, `internal/memory`, `internal/cache`, `internal/backup`, `internal/security`, `internal/store`, `internal/observe`, `internal/config`
- [ ] T003 [P] Add base configuration loader for `~/.noto` paths in `internal/config/config.go`
- [ ] T004 [P] Add CLI root command and subcommand wiring in `internal/app/root.go` and `internal/app/commands.go`
- [ ] T005 [P] Configure formatting/lint/static analysis in `.golangci.yml` and `Makefile`
- [ ] T006 [P] Create test folders and harness bootstrap in `tests/unit/.keep`, `tests/integration/.keep`, and `tests/contract/.keep`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T007 Implement SQLite connection manager and pragmas in `internal/store/sqlite.go`
- [ ] T008 Implement schema migrations for core entities in `internal/store/migrations/001_init.sql`
- [ ] T009 [P] Implement profile-scoped repositories (`Profile`, `Conversation`, `Message`) in `internal/store/profile_repo.go` and `internal/store/chat_repo.go`
- [ ] T010 [P] Implement repositories for `MemoryNote`, `SessionSummary`, `ContextCacheEntry` in `internal/store/memory_repo.go` and `internal/store/cache_repo.go`
- [ ] T011 [P] Implement repository for `BackupSnapshot` and retention policy in `internal/store/backup_repo.go`
- [ ] T012 [P] Implement credential encryption/decryption service in `internal/security/credentials.go`
- [ ] T013 [P] Implement provider abstraction interface and error normalization in `internal/provider/provider.go`
- [ ] T014 [P] Implement structured logging and local metrics emitter in `internal/observe/observe.go`
- [ ] T015 Implement profile isolation guard helpers used by all services in `internal/app/isolation.go`
- [ ] T016 Implement shared TUI shell/state model for startup/select/chat/recovery screens in `internal/tui/model.go`

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Start and Chat with a Profile (Priority: P1) 🎯 MVP

**Goal**: User can start app, resolve 0/1/many profile startup path, and chat in active profile.

**Independent Test**: Run startup flows for 0, 1, and many profiles; verify active profile chat works and remains profile-scoped.

### Tests for User Story 1 (REQUIRED) ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T017 [P] [US1] Add contract test for startup branching (0/1/many profiles) in `tests/contract/startup_flow_contract_test.go`
- [ ] T018 [P] [US1] Add integration test for profile creation + auto-select in `tests/integration/startup_profile_flow_test.go`
- [ ] T019 [P] [US1] Add integration test for active-profile chat response path in `tests/integration/chat_active_profile_test.go`

### Implementation for User Story 1

- [ ] T020 [US1] Implement startup profile resolver service in `internal/profile/startup_resolver.go`
- [ ] T021 [US1] Implement profile create/list/select command handlers in `internal/profile/commands_basic.go`
- [ ] T022 [US1] Implement startup TUI screens and selection transitions in `internal/tui/startup.go`
- [ ] T023 [US1] Implement chat session bootstrap with active profile context in `internal/chat/session.go`
- [ ] T024 [US1] Implement base `noto chat` command integration in `internal/app/chat_cmd.go`
- [ ] T025 [US1] Add startup/chat observability events and profile-scope validation in `internal/observe/events_startup_chat.go`

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - Persistent Memory Continuity (Priority: P2)

**Goal**: Conversation notes, summaries, cache reuse, and continuity work across sessions with recovery behavior.

**Independent Test**: End a session with notes, restart, verify continuity context reuse (cache hit/miss), and verify corruption recovery path.

### Tests for User Story 2 (REQUIRED) ⚠️

- [ ] T026 [P] [US2] Add contract test for session-end note/summary/cache persistence in `tests/contract/memory_continuity_contract_test.go`
- [ ] T027 [P] [US2] Add integration test for cache hit/miss rebuild behavior in `tests/integration/context_cache_flow_test.go`
- [ ] T028 [P] [US2] Add integration test for DB corruption auto-repair + backup restore in `tests/integration/recovery_flow_test.go`

### Implementation for User Story 2

- [ ] T029 [US2] Implement memory note extraction and categorization service in `internal/memory/extractor.go`
- [ ] T030 [US2] Implement session summary generation and persistence in `internal/memory/session_summary.go`
- [ ] T031 [US2] Implement context cache manager (build/load/invalidate) in `internal/cache/manager.go`
- [ ] T032 [US2] Implement session-end and periodic backup scheduler/orchestrator in `internal/backup/scheduler.go` and `internal/backup/service.go`
- [ ] T033 [US2] Implement corruption detection and deterministic recovery pipeline in `internal/store/recovery.go`
- [ ] T034 [US2] Emit retrieval/cache/recovery metrics and structured logs in `internal/observe/events_memory_recovery.go`

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Manage Profiles and Prompts Safely (Priority: P3)

**Goal**: User can manage profile lifecycle and prompt edits safely with strict isolation and explicit deletion confirmation.

**Independent Test**: Execute create/list/select/rename/delete/prompt-edit commands, verify confirmation enforcement and no cross-profile leakage.

### Tests for User Story 3 (REQUIRED) ⚠️

- [ ] T035 [P] [US3] Add contract test for profile lifecycle commands in `tests/contract/profile_management_contract_test.go`
- [ ] T036 [P] [US3] Add integration test for prompt edit immediate effect + cache invalidation in `tests/integration/prompt_edit_effect_test.go`
- [ ] T037 [P] [US3] Add integration test for deletion confirmation safety and isolation in `tests/integration/profile_delete_safety_test.go`

### Implementation for User Story 3

- [ ] T038 [US3] Implement profile rename/delete handlers with explicit confirmation phrase in `internal/profile/commands_manage.go`
- [ ] T039 [US3] Implement prompt show/edit service and file persistence in `internal/profile/prompt_service.go`
- [ ] T040 [US3] Wire prompt edit to context-cache invalidation in `internal/cache/invalidation.go`
- [ ] T041 [US3] Implement provider credential setup/update flow with encrypted storage in `internal/provider/credentials_cmd.go`
- [ ] T042 [US3] Add destructive-action and credential-safety observability events in `internal/observe/events_profile_security.go`

**Checkpoint**: All user stories should now be independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T043 [P] Add end-to-end quickstart validation script in `scripts/validate-quickstart.sh`
- [ ] T044 Enforce CI gates for fmt/lint/static analysis/tests in `.github/workflows/ci.yml`
- [ ] T045 [P] Add performance benchmark tests for startup, cache hit/miss, and profile commands in `tests/integration/performance_bench_test.go`
- [ ] T046 Add docs for local security/recovery/observability behavior in `README.md`
- [ ] T047 Execute and record quickstart + reliability checks in `specs/001-build-profile-memory-cli/quickstart.md`
- [ ] T048 Final code cleanup/refactor pass for module boundaries in `internal/app/app.go` and `internal/chat/pipeline.go`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: Depend on Foundational phase completion
  - US1 (P1) first for MVP
  - US2 (P2) depends on US1 chat/session baseline
  - US3 (P3) can run after Foundational, but safest after US1 due to shared profile command paths
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational; no dependency on other stories
- **User Story 2 (P2)**: Starts after US1 baseline chat/session path exists
- **User Story 3 (P3)**: Starts after Foundational; partial overlap with US1 profile command code

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Data/repo integration before service orchestration
- Service orchestration before command/TUI wiring
- Observability and error handling included before phase checkpoint

### Parallel Opportunities

- Setup tasks marked [P] can run concurrently (T003, T004, T005, T006)
- Foundational tasks marked [P] can run concurrently after T007/T008 baseline
- US1 tests T017-T019 can run in parallel
- US2 tests T026-T028 can run in parallel
- US3 tests T035-T037 can run in parallel
- Polish tasks T043 and T045 can run in parallel

---

## Parallel Example: User Story 1

```bash
Task: "T017 [US1] startup flow contract test in tests/contract/startup_flow_contract_test.go"
Task: "T018 [US1] startup profile integration test in tests/integration/startup_profile_flow_test.go"
Task: "T019 [US1] active-profile chat integration test in tests/integration/chat_active_profile_test.go"
```

## Parallel Example: User Story 2

```bash
Task: "T026 [US2] continuity contract test in tests/contract/memory_continuity_contract_test.go"
Task: "T027 [US2] cache hit/miss integration test in tests/integration/context_cache_flow_test.go"
Task: "T028 [US2] recovery integration test in tests/integration/recovery_flow_test.go"
```

## Parallel Example: User Story 3

```bash
Task: "T035 [US3] profile management contract test in tests/contract/profile_management_contract_test.go"
Task: "T036 [US3] prompt edit integration test in tests/integration/prompt_edit_effect_test.go"
Task: "T037 [US3] delete safety integration test in tests/integration/profile_delete_safety_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. Validate startup + active-profile chat end-to-end
5. Demo MVP

### Incremental Delivery

1. Setup + Foundational
2. Deliver US1 (startup/profile/chat baseline)
3. Deliver US2 (memory continuity/cache/recovery)
4. Deliver US3 (profile lifecycle/prompt safety)
5. Run polish and performance gates

### Parallel Team Strategy

1. Team completes Setup + Foundational
2. Then split:
   - Dev A: US1 command/TUI/chat baseline
   - Dev B: US2 memory/cache/backup/recovery
   - Dev C: US3 profile lifecycle/prompt/credential flows
3. Merge after each story checkpoint with full test gate

---

## Notes

- [P] tasks = different files, low coupling
- [Story] label maps tasks to specific user stories
- Each story has tests first and independent validation target
- All tasks include explicit file paths
- Suggested MVP scope: Phase 1 + Phase 2 + Phase 3 (US1)
