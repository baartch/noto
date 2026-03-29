# Tasks: Noto Profile Memory CLI

**Input**: Design documents from `/specs/001-build-profile-memory-cli/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED for every user story and are created before implementation tasks.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and baseline tooling

- [X] T001 Initialize Go module and root build/test targets in /home/andy/gitrepos/noto/go.mod and /home/andy/gitrepos/noto/Makefile
- [X] T002 [P] Add core runtime dependencies (cobra, bubbletea, bubbles, lipgloss, sqlite driver) in /home/andy/gitrepos/noto/go.mod
- [X] T003 [P] Add lint/format/staticcheck configuration in /home/andy/gitrepos/noto/.golangci.yml
- [X] T004 [P] Create baseline package directories and package docs in /home/andy/gitrepos/noto/internal/{app,tui,profile,chat,commands,parser,suggest,provider,memory,vector,cache,backup,security,store,observe,config}/doc.go
- [X] T005 Define user-local app path conventions and constants in /home/andy/gitrepos/noto/internal/config/paths.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Create SQLite connection manager, pragma setup, and transaction helpers in /home/andy/gitrepos/noto/internal/store/sqlite.go
- [X] T007 [P] Implement base schema migrations for profile/conversation/message/memory/session_summary/provider_config/context_cache tables in /home/andy/gitrepos/noto/internal/store/migrations/0001_init.sql
- [X] T008 [P] Implement profile-local backup/restore primitives for DB and vector files in /home/andy/gitrepos/noto/internal/backup/filesystem.go
- [X] T009 [P] Implement credential encryption/decryption helpers for provider secrets in /home/andy/gitrepos/noto/internal/security/credentials.go
- [X] T010 [P] Implement structured logger and metrics emitter interfaces in /home/andy/gitrepos/noto/internal/observe/observe.go
- [X] T011 Implement shared command registry abstractions for CLI and slash parity in /home/andy/gitrepos/noto/internal/commands/registry.go
- [X] T012 [P] Implement slash lexer/parser core with hierarchical command parsing in /home/andy/gitrepos/noto/internal/parser/slash_parser.go
- [X] T013 [P] Implement slash suggestion engine with prefix ranking in /home/andy/gitrepos/noto/internal/suggest/engine.go
- [X] T014 Implement vector index adapter interface and profile-scoped file contract in /home/andy/gitrepos/noto/internal/vector/index.go
- [X] T015 Implement vector manifest + source-state tracking repository in /home/andy/gitrepos/noto/internal/store/vector_manifest_repo.go
- [X] T016 Implement app bootstrap wiring (config/store/commands/observe) in /home/andy/gitrepos/noto/internal/app/bootstrap.go

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Start and Chat with a Profile (Priority: P1) 🎯 MVP

**Goal**: Users can start the terminal app, create/select profile, and chat in active profile

**Independent Test**: Launch with zero/one/multiple profiles and verify correct startup routing with last-used profile auto-selection and successful chat response in active profile

### Tests for User Story 1 (REQUIRED) ⚠️

- [X] T017 [P] [US1] Add startup behavior integration tests for zero/one/multiple profile flows in /home/andy/gitrepos/noto/tests/integration/startup_profile_selection_test.go
- [X] T018 [P] [US1] Add CLI contract tests for profile and chat command surface in /home/andy/gitrepos/noto/tests/contract/cli_profile_chat_contract_test.go
- [X] T019 [P] [US1] Add slash parity tests for command execution equivalence in /home/andy/gitrepos/noto/tests/contract/slash_parity_profile_chat_test.go

### Implementation for User Story 1

- [X] T020 [P] [US1] Implement profile repository CRUD/select/default operations in /home/andy/gitrepos/noto/internal/store/profile_repo.go
- [X] T021 [P] [US1] Implement profile lifecycle service (create/list/select/rename/delete guards) in /home/andy/gitrepos/noto/internal/profile/service.go
- [X] T022 [US1] Implement startup profile resolution flow in /home/andy/gitrepos/noto/internal/app/startup_flow.go
- [X] T023 [P] [US1] Implement CLI profile/chat commands using shared registry in /home/andy/gitrepos/noto/internal/commands/profile_commands.go
- [X] T024 [P] [US1] Implement slash dispatcher integration in chat input loop in /home/andy/gitrepos/noto/internal/chat/slash_dispatch.go
- [X] T025 [US1] Implement conversation/message persistence repositories in /home/andy/gitrepos/noto/internal/store/conversation_repo.go and /home/andy/gitrepos/noto/internal/store/message_repo.go
- [X] T026 [US1] Implement provider adapter abstraction and base OpenAI-compatible adapter in /home/andy/gitrepos/noto/internal/provider/adapter.go and /home/andy/gitrepos/noto/internal/provider/openai_compatible.go
- [X] T027 [US1] Implement core chat turn pipeline with active profile context binding in /home/andy/gitrepos/noto/internal/chat/pipeline.go
- [X] T028 [US1] Implement TUI startup and chat model transitions for profile flows in /home/andy/gitrepos/noto/internal/tui/model.go

**Checkpoint**: User Story 1 fully functional and independently testable

---

## Phase 4: User Story 2 - Persistent Memory Continuity (Priority: P2)

**Goal**: Persistent profile memory continuity with cache reuse and vector-assisted semantic retrieval

**Independent Test**: Complete session with memory-worthy facts/actions; restart; verify continuity, cache reuse, cache rebuild, and vector fallback behavior

### Tests for User Story 2 (REQUIRED) ⚠️

- [X] T029 [P] [US2] Add memory extraction and retrieval integration tests in /home/andy/gitrepos/noto/tests/integration/memory_continuity_test.go
- [X] T030 [P] [US2] Add context cache hit/miss/invalidation tests in /home/andy/gitrepos/noto/tests/integration/context_cache_lifecycle_test.go
- [X] T031 [P] [US2] Add vector sync and semantic top-k retrieval tests in /home/andy/gitrepos/noto/tests/integration/vector_sync_retrieval_test.go
- [X] T032 [P] [US2] Add vector corruption/missing-file rebuild and fallback tests in /home/andy/gitrepos/noto/tests/integration/vector_rebuild_fallback_test.go

### Implementation for User Story 2

- [X] T033 [P] [US2] Implement memory note and session summary repositories in /home/andy/gitrepos/noto/internal/store/memory_note_repo.go and /home/andy/gitrepos/noto/internal/store/session_summary_repo.go
- [X] T034 [US2] Implement memory extraction service for fact/progress/blocker/action-item capture in /home/andy/gitrepos/noto/internal/memory/extractor.go
- [X] T035 [US2] Implement context assembly/retrieval service with source-of-truth SQLite reads in /home/andy/gitrepos/noto/internal/memory/retrieval.go
- [X] T036 [P] [US2] Implement context cache repository and invalidation triggers in /home/andy/gitrepos/noto/internal/store/context_cache_repo.go and /home/andy/gitrepos/noto/internal/cache/service.go
- [X] T037 [US2] Implement vector embedding + upsert sync pipeline from SQLite sources in /home/andy/gitrepos/noto/internal/vector/sync.go
- [X] T038 [US2] Implement hybrid retrieval orchestrator (vector candidate recall + SQLite authoritative hydrate/filter) in /home/andy/gitrepos/noto/internal/vector/retrieval.go
- [X] T039 [US2] Implement vector rebuild command/service from SQLite state in /home/andy/gitrepos/noto/internal/vector/rebuild.go
- [X] T040 [US2] Integrate memory capture/session-end handoff generation in chat lifecycle in /home/andy/gitrepos/noto/internal/chat/session_handoff.go
- [X] T041 [US2] Emit retrieval/cache/vector observability events and metrics in /home/andy/gitrepos/noto/internal/observe/events_memory.go

**Checkpoint**: User Stories 1 and 2 functional with continuity and semantic retrieval acceleration

---

## Phase 5: User Story 3 - Manage Profiles and Prompts Safely (Priority: P3)

**Goal**: Full profile lifecycle and prompt management with strict isolation and destructive-action safety

**Independent Test**: Execute create/list/select/rename/delete + prompt show/edit flows and verify confirmation, isolation, and immediate effect

### Tests for User Story 3 (REQUIRED) ⚠️

- [X] T042 [P] [US3] Add profile lifecycle integration tests (create/list/select/rename/delete) in /home/andy/gitrepos/noto/tests/integration/profile_lifecycle_test.go
- [X] T043 [P] [US3] Add prompt show/edit behavior tests with immediate effect in /home/andy/gitrepos/noto/tests/integration/prompt_management_test.go
- [X] T044 [P] [US3] Add cross-profile isolation and destructive confirmation tests in /home/andy/gitrepos/noto/tests/integration/profile_isolation_safety_test.go

### Implementation for User Story 3

- [X] T045 [P] [US3] Implement prompt file repository and profile prompt version tracking in /home/andy/gitrepos/noto/internal/profile/prompt_store.go
- [X] T046 [US3] Implement prompt show/edit command handlers (CLI + slash parity) in /home/andy/gitrepos/noto/internal/commands/prompt_commands.go
- [X] T047 [US3] Implement strong explicit confirmation flow for profile deletion in /home/andy/gitrepos/noto/internal/profile/delete_confirmation.go
- [X] T048 [US3] Implement active-profile deletion fallback selection behavior in /home/andy/gitrepos/noto/internal/profile/delete_flow.go
- [X] T049 [US3] Implement profile isolation guards across memory/cache/vector repositories in /home/andy/gitrepos/noto/internal/store/isolation_guards.go
- [X] T050 [US3] Wire prompt-change invalidation for cache and vector entries in /home/andy/gitrepos/noto/internal/cache/invalidation.go and /home/andy/gitrepos/noto/internal/vector/invalidation.go

**Checkpoint**: All user stories independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Hardening and validation across all stories

- [X] T051 [P] Add corruption recovery orchestration tests (auto-repair then backup restore) in /home/andy/gitrepos/noto/tests/integration/recovery_orchestration_test.go
- [X] T052 Add recovery coordinator for SQLite and vector sidecar artifacts in /home/andy/gitrepos/noto/internal/backup/recovery.go
- [X] T053 [P] Add slash suggestion visibility/ambiguity/unknown-command UX regression tests in /home/andy/gitrepos/noto/tests/integration/slash_ux_regression_test.go
- [X] T054 [P] Add performance benchmark tests for startup/retrieval/suggestions/vector lookup in /home/andy/gitrepos/noto/tests/integration/performance_bench_test.go
- [X] T055 Enforce lint/format/static analysis and test gates in /home/andy/gitrepos/noto/.github/workflows/ci.yml
- [X] T056 Update user docs and operational runbook for backup/recovery/vector rebuild in /home/andy/gitrepos/noto/docs/operations.md
- [X] T057 Run quickstart validation and capture evidence in /home/andy/gitrepos/noto/specs/001-build-profile-memory-cli/checklists/requirements.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all user stories
- **User Story phases (Phase 3+)**: Depend on Foundational completion
- **Polish (Phase 6)**: Depends on completion of target user stories

### User Story Dependencies

- **US1 (P1)**: Starts after Foundational; no dependency on US2/US3
- **US2 (P2)**: Starts after Foundational; depends on US1 chat pipeline/repositories being present
- **US3 (P3)**: Starts after Foundational; can proceed largely independently but validates integration with US1/US2 storage boundaries

### Within Each User Story

- Tests first (must fail before implementation)
- Repositories/models before services
- Services before command/UI integration
- Story-specific observability and validation before phase close

## Parallel Opportunities

- Setup parallel: T002, T003, T004
- Foundational parallel: T007, T008, T009, T010, T012, T013
- US1 tests parallel: T017, T018, T019
- US1 implementation parallel: T020, T021, T023, T024 (then integrate via T022/T027/T028)
- US2 tests parallel: T029, T030, T031, T032
- US2 implementation parallel: T033, T036 (then T034/T035/T037/T038/T039/T040)
- US3 tests parallel: T042, T043, T044
- US3 implementation parallel: T045 and T047 can run in parallel before T046/T048/T049/T050
- Polish parallel: T051, T053, T054

---

## Parallel Example: User Story 2

```bash
# Run US2 test tasks together:
Task: "T029 [US2] Add memory extraction and retrieval integration tests in tests/integration/memory_continuity_test.go"
Task: "T030 [US2] Add context cache hit/miss/invalidation tests in tests/integration/context_cache_lifecycle_test.go"
Task: "T031 [US2] Add vector sync and semantic top-k retrieval tests in tests/integration/vector_sync_retrieval_test.go"
Task: "T032 [US2] Add vector corruption/missing-file rebuild and fallback tests in tests/integration/vector_rebuild_fallback_test.go"

# Run early US2 data-layer tasks together:
Task: "T033 [US2] Implement memory note and session summary repositories in internal/store/memory_note_repo.go and internal/store/session_summary_repo.go"
Task: "T036 [US2] Implement context cache repository and invalidation triggers in internal/store/context_cache_repo.go and internal/cache/service.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. Validate US1 independent tests and acceptance
5. Demo/release MVP baseline

### Incremental Delivery

1. Deliver US1 (startup + profile + chat baseline)
2. Deliver US2 (memory continuity + cache + vector secondary retrieval)
3. Deliver US3 (profile/prompt safety + strict isolation hardening)
4. Deliver Phase 6 polish (recovery/perf/ops hardening)

### Suggested MVP Scope

- **MVP = User Story 1 only** after Setup + Foundational.
- Keep vector work in US2 to preserve fast MVP path while retaining planned architecture hooks.

---

## Notes

- All tasks follow required checklist format: `- [ ] T### [P?] [US?] Description with file path`
- `[P]` tasks target different files and no unresolved intra-task dependencies
- User stories remain independently testable at each checkpoint
