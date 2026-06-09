# Tasks: Memory Context Timeline & Tooling

**Input**: Design documents from `/specs/005-memory-context/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/, quickstart.md

**Tests**: Include automated tests per constitution requirements and plan commitments.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story label (`[US1]`, `[US2]`, `[US3]`, `[US4]`, `[US5]`)
- All tasks include exact file paths

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare the codebase and specs for the broadened timeline-memory scope.

- [X] T001 Align implementation scope notes and validation targets in /home/andy/gitrepos/noto/specs/005-memory-context/plan.md
- [X] T002 Create integration test fixtures for day-based raw windows, monthly summary windows, rollups, and tool calling in /home/andy/gitrepos/noto/tests/integration/memory/timeline_fixtures_test.go
- [X] T003 [P] Add shared test helpers/constants for OpenRouter tool-calling payloads in /home/andy/gitrepos/noto/tests/unit/provider/testdata/tool_calling.go
- [X] T004 [P] Add shared footer telemetry formatting fixtures in /home/andy/gitrepos/noto/tests/unit/tui/testdata/footer_telemetry.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure required before any user story can be completed.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T005 Add timeline settings persistence model and accessors for raw-note days, weekly-summary weeks, and monthly-summary months in /home/andy/gitrepos/noto/internal/store/timeline_settings_repo.go
- [X] T006 [P] Add weekly/monthly summary persistence models and repositories in /home/andy/gitrepos/noto/internal/store/memory_summary_repo.go
- [X] T007 [P] Add profile DB migration for timeline settings and summary artifacts in /home/andy/gitrepos/noto/internal/store/migrations/profile/0003_memory_timeline.sql
- [X] T008 Define timeline window selection primitives and period helpers in /home/andy/gitrepos/noto/internal/memory/timeline.go
- [X] T009 [P] Extend provider model metadata types to include context length and tool support in /home/andy/gitrepos/noto/internal/provider/models.go and /home/andy/gitrepos/noto/internal/provider/adapter.go
- [X] T010 [P] Extend OpenAI-compatible provider request/response types for tool definitions and tool call turns in /home/andy/gitrepos/noto/internal/provider/openai_compatible.go
- [X] T011 Add summary freshness/versioning helpers in /home/andy/gitrepos/noto/internal/memory/summary_rollups.go
- [X] T012 [P] Add unit tests for timeline period calculation and zero-window behavior in /home/andy/gitrepos/noto/tests/unit/memory/timeline_test.go
- [X] T013 [P] Add unit tests for provider model metadata parsing (`context_length`, tool support) in /home/andy/gitrepos/noto/tests/unit/provider/models_test.go
- [X] T014 [P] Add unit tests for provider tool-calling request/response normalization and invalid tool payload handling in /home/andy/gitrepos/noto/tests/unit/provider/tool_calling_test.go
- [X] T015 [P] Add unit tests for timeline settings serialization and validation covering arbitrary raw-note day values greater than 0, weekly-summary week values greater than 0, rejection of zero weekly/monthly summary values, bounded monthly windows greater than 0, and `all_remaining` sentinel values in /home/andy/gitrepos/noto/tests/unit/memory/timeline_settings_test.go

**Checkpoint**: Foundation ready — user story implementation can begin.

---

## Phase 3: User Story 1 - Time-Layered Context Assembly (Priority: P1) 🎯 MVP

**Goal**: Deliver configurable default context assembly using a day-based raw-note window plus weekly-summary and monthly-summary windows without conversation summaries.

**Independent Test**: Populate a profile with notes spanning several months, vary the timeline settings, and verify the assembled context includes the correct raw notes for the configured rolling-day window, weekly summaries for the configured week span, and monthly summaries for the configured older-history window.

### Tests for User Story 1

- [X] T016 [P] [US1] Add retrieval tests for configured raw-day, weekly-week, and monthly-summary windows plus preceding-Monday fill behavior, weekly-to-monthly no-gap coverage, and boundary-cutover rules in /home/andy/gitrepos/noto/tests/unit/memory/retrieval_timeline_test.go
- [X] T017 [P] [US1] Add integration test for timeline-based context assembly across multi-month history in /home/andy/gitrepos/noto/tests/integration/memory/timeline_context_test.go
- [X] T018 [P] [US1] Add integration test proving conversation summaries are excluded from assembled context in /home/andy/gitrepos/noto/tests/integration/memory/no_session_summary_test.go
- [X] T019 [P] [US1] Add integration test for bounded monthly-summary windows, rejection of zero weekly/monthly summary settings, and exclusion of history older than the monthly cutoff in /home/andy/gitrepos/noto/tests/integration/memory/timeline_settings_edges_test.go

### Implementation for User Story 1

- [X] T020 [US1] Add raw-note day settings (any integer greater than 0), weekly-summary week settings, and monthly-summary month settings to chat settings state and menu wiring in /home/andy/gitrepos/noto/internal/tui/model.go
- [X] T021 [US1] Load, persist, and apply raw-note day settings (any integer greater than 0), weekly-summary week settings, and monthly-summary month settings in chat bootstrap, live settings updates, and profile switching flows in /home/andy/gitrepos/noto/internal/app/chat_cmd.go
- [X] T022 [US1] Replace session-summary-based memory selection with timeline-window context assembly that fills the raw-note day window backward to the preceding Monday and extends weekly coverage to at least the first day of the following monthly-summary month in /home/andy/gitrepos/noto/internal/memory/retrieval.go
- [X] T023 [US1] Add formatted memory-block sections for raw notes, weekly summaries, and monthly summaries in /home/andy/gitrepos/noto/internal/memory/retrieval.go
- [X] T024 [US1] Remove reliance on conversation summaries during chat context assembly in /home/andy/gitrepos/noto/internal/chat/session.go and /home/andy/gitrepos/noto/internal/memory/retrieval.go
- [X] T025 [US1] Add fallback behavior for missing summary layers using best available memory in /home/andy/gitrepos/noto/internal/memory/retrieval.go

**Checkpoint**: User Story 1 should be fully functional and testable independently.

---

## Phase 4: User Story 2 - Automatic Summary Rollups (Priority: P2)

**Goal**: Automatically create, reuse, and regenerate weekly/monthly summary artifacts as profile history advances.

**Independent Test**: Cross week/month boundaries with eligible notes, verify summaries are created automatically, then edit underlying notes and verify affected summaries become stale and are regenerated or queued for regeneration.

### Tests for User Story 2

- [X] T026 [P] [US2] Add unit tests for weekly/monthly rollup eligibility and duplicate prevention in /home/andy/gitrepos/noto/tests/unit/memory/summary_rollups_test.go
- [X] T027 [P] [US2] Add integration test for catch-up rollup generation after inactive periods in /home/andy/gitrepos/noto/tests/integration/memory/rollup_catchup_test.go
- [X] T028 [P] [US2] Add integration test for stale-summary marking and failure fallback after covered-note changes in /home/andy/gitrepos/noto/tests/integration/memory/rollup_regeneration_test.go

### Implementation for User Story 2

- [X] T029 [US2] Implement weekly and monthly summary creation logic in /home/andy/gitrepos/noto/internal/memory/summary_rollups.go
- [X] T030 [US2] Trigger opportunistic rollup generation during profile processing/chat turns in /home/andy/gitrepos/noto/internal/chat/session.go
- [X] T031 [US2] Persist and query summary artifacts through repositories in /home/andy/gitrepos/noto/internal/store/memory_summary_repo.go
- [X] T032 [US2] Enforce unique weekly/monthly summary periods at repository and migration level in /home/andy/gitrepos/noto/internal/store/memory_summary_repo.go and /home/andy/gitrepos/noto/internal/store/migrations/profile/0003_memory_timeline.sql
- [X] T033 [US2] Mark summaries stale when covered notes are added or updated in /home/andy/gitrepos/noto/internal/memory/processor.go and /home/andy/gitrepos/noto/internal/memory/extractor.go
- [X] T034 [US2] Add regeneration path for stale summaries, failure handling, and safe replacement semantics in /home/andy/gitrepos/noto/internal/memory/summary_rollups.go

**Checkpoint**: User Stories 1 and 2 should work independently and together.

---

## Phase 5: User Story 3 - LLM Memory Search Tools (Priority: P3)

**Goal**: Expose keyword/vector and time-range memory search through OpenRouter-compatible tool calling.

**Independent Test**: Ask the assistant questions that require topical and date-bounded memory lookup, verify tool definitions are sent, tool calls are executed locally, and final replies use the returned search results.

### Tests for User Story 3

- [ ] T035 [P] [US3] Add unit tests for keyword search tool schema, execution, and deterministic fallback behavior in /home/andy/gitrepos/noto/tests/unit/memory/keyword_search_tool_test.go
- [ ] T036 [P] [US3] Add unit tests for time-range search tool schema, mixed raw+summary results, and execution in /home/andy/gitrepos/noto/tests/unit/memory/time_range_search_tool_test.go
- [ ] T037 [P] [US3] Add integration test for OpenRouter tool-calling round-trip in /home/andy/gitrepos/noto/tests/integration/provider/tool_calling_roundtrip_test.go
- [ ] T038 [P] [US3] Add integration test for no-tool-support graceful degradation and invalid tool-call payload handling in /home/andy/gitrepos/noto/tests/integration/provider/tool_support_fallback_test.go

### Implementation for User Story 3

- [ ] T039 [US3] Implement keyword/vector memory search executor with deterministic fallback in /home/andy/gitrepos/noto/internal/memory/search_tools.go
- [ ] T040 [US3] Implement time-range SQLite memory search executor with mixed raw+summary result support in /home/andy/gitrepos/noto/internal/memory/search_tools.go
- [ ] T041 [US3] Add OpenRouter-compatible tool definitions and follow-up request handling in /home/andy/gitrepos/noto/internal/provider/openai_compatible.go
- [ ] T042 [US3] Integrate and keep canonical tool-call orchestration in /home/andy/gitrepos/noto/internal/chat/pipeline.go and /home/andy/gitrepos/noto/internal/chat/session.go
- [ ] T043 [US3] Gate tool exposure on model/provider capability metadata in /home/andy/gitrepos/noto/internal/provider/models.go and /home/andy/gitrepos/noto/internal/chat/session.go

**Checkpoint**: User Story 3 should be independently functional with or without default timeline context.

---

## Phase 6: User Story 4 - Fast and Correct Cache Reuse (Priority: P4)

**Goal**: Preserve and extend cache correctness under timeline settings, summary state changes, and broadened retrieval state.

**Independent Test**: Warm the cache, change settings or summary state, and verify valid hits, stale-while-revalidate behavior, and invalidation when any retrieval-shaping input changes.

### Tests for User Story 4

- [ ] T044 [P] [US4] Add unit tests for cache identity including timeline settings and summary state, plus internal token-fitting reduction order that drops oldest monthly-summary coverage first, in /home/andy/gitrepos/noto/tests/unit/memory/retrieval_cache_identity_timeline_test.go
- [ ] T045 [P] [US4] Add integration test for timeline-settings invalidation, including live in-session settings updates, and persistent cache reuse in /home/andy/gitrepos/noto/tests/integration/memory/timeline_cache_invalidation_test.go
- [ ] T046 [P] [US4] Add integration test for summary-state-driven cache invalidation and miss-reason classification in /home/andy/gitrepos/noto/tests/integration/memory/summary_cache_invalidation_test.go

### Implementation for User Story 4

- [ ] T047 [US4] Extend cache key construction with raw-note day settings, weekly-summary week settings, monthly-summary month settings, and assembled summary state in /home/andy/gitrepos/noto/internal/memory/retrieval.go
- [ ] T048 [US4] Remove user-facing token-budget configuration paths while preserving internal token fitting safeguards, including dropping the oldest monthly-summary coverage first when assembled context must be reduced before touching newer timeline layers, in /home/andy/gitrepos/noto/internal/app/chat_cmd.go and /home/andy/gitrepos/noto/internal/memory/retrieval.go
- [ ] T049 [US4] Invalidate or stale cache entries on timeline-setting changes, including live in-session updates, in /home/andy/gitrepos/noto/internal/cache/invalidation.go and /home/andy/gitrepos/noto/internal/app/chat_cmd.go
- [ ] T050 [US4] Invalidate or stale cache entries on summary artifact changes and classify corresponding miss reasons in /home/andy/gitrepos/noto/internal/cache/invalidation.go and /home/andy/gitrepos/noto/internal/memory/summary_rollups.go
- [ ] T051 [US4] Preserve L1→L2 and stale-while-revalidate behavior under timeline retrieval in /home/andy/gitrepos/noto/internal/memory/retrieval.go and /home/andy/gitrepos/noto/internal/cache/service.go

**Checkpoint**: User Story 4 should maintain fast reuse without incorrect hits.

---

## Phase 7: User Story 5 - Observable Memory Health (Priority: P5)

**Goal**: Surface actionable diagnostics for rollups, retrieval, tool usage, and footer context-capacity telemetry.

**Independent Test**: Trigger rollup events, cache hits/misses, tool calls, and model switches, then verify diagnostics and footer telemetry accurately reflect the current state.

### Tests for User Story 5

- [ ] T052 [P] [US5] Add unit tests for footer token/context-capacity formatting and unknown-capacity fallback in /home/andy/gitrepos/noto/tests/unit/tui/footer_telemetry_test.go
- [ ] T053 [P] [US5] Add integration test for footer updates after model metadata and usage changes in /home/andy/gitrepos/noto/tests/integration/tui/footer_context_capacity_test.go
- [ ] T054 [P] [US5] Add integration test for rollup/tool/cache diagnostics visibility and miss-reason completeness in /home/andy/gitrepos/noto/tests/integration/memory/memory_diagnostics_test.go

### Implementation for User Story 5

- [ ] T055 [US5] Extend provider model listing to parse and return `context_length` and supported tool parameters in /home/andy/gitrepos/noto/internal/provider/models.go
- [ ] T056 [US5] Fetch/cache active model metadata during startup, profile switch, and model change flows in /home/andy/gitrepos/noto/internal/app/chat_cmd.go and /home/andy/gitrepos/noto/internal/chat/session.go
- [ ] T057 [US5] Propagate active model context maximum into session stats in /home/andy/gitrepos/noto/internal/provider/stats.go and /home/andy/gitrepos/noto/internal/chat/session.go
- [ ] T058 [US5] Update footer token/status rendering to show max context and used percentage next to tokens in /home/andy/gitrepos/noto/internal/tui/footer_view.go and /home/andy/gitrepos/noto/internal/tui/model.go
- [ ] T059 [US5] Add unknown-capacity footer fallback behavior in /home/andy/gitrepos/noto/internal/tui/footer_view.go
- [ ] T060 [US5] Extend diagnostics reporting with recent rollup activity, tool-call outcomes, timeline/summary miss-reason completeness, and bounded monthly-cutoff exclusion visibility in /home/andy/gitrepos/noto/internal/memory/retrieval.go and /home/andy/gitrepos/noto/internal/chat/pipeline.go

**Checkpoint**: All user stories should now be independently functional.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final cleanup, docs alignment, and full validation across all stories.

- [ ] T061 [P] Update implementation-aligned notes in /home/andy/gitrepos/noto/specs/005-memory-context/data-model.md and /home/andy/gitrepos/noto/specs/005-memory-context/contracts/context-retrieval.md
- [ ] T062 [P] Remove obsolete session-summary-specific comments or dead paths in /home/andy/gitrepos/noto/internal/memory/doc.go, /home/andy/gitrepos/noto/internal/chat/session.go, and /home/andy/gitrepos/noto/docs/CONTEXT_AT_A_GLANCE.txt
- [ ] T063 Run full verification suite (`make fmt`, `make lint`, `make test`) and capture outcomes in /home/andy/gitrepos/noto/specs/005-memory-context/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: starts immediately
- **Phase 2 (Foundational)**: depends on Setup completion; blocks all user story work
- **Phase 3 (US1)**: depends on Foundational completion
- **Phase 4 (US2)**: depends on Foundational completion; builds on US1 retrieval shapes but remains independently testable
- **Phase 5 (US3)**: depends on Foundational completion and can start once provider tool-call primitives are in place
- **Phase 6 (US4)**: depends on US1 retrieval behavior and summary-state primitives from US2
- **Phase 7 (US5)**: depends on provider metadata work and completed telemetry hooks from earlier stories
- **Phase 8 (Polish)**: depends on all desired stories being complete

### User Story Dependencies

- **US1 (P1)**: first MVP slice; no dependency on later stories
- **US2 (P2)**: depends on US1 timeline assembly foundations
- **US3 (P3)**: independent of US2 for basic tool execution, but benefits from US1 timeline context and shared summary entities
- **US4 (P4)**: depends on US1 and US2 because cache identity must include timeline settings and summary state
- **US5 (P5)**: depends on US3/US4 telemetry and diagnostics hooks

### Within Each User Story

- Tests are authored before or alongside implementation and should fail before the final implementation is considered complete.
- Storage/repository updates precede service orchestration.
- Service logic precedes chat/provider/TUI integration.
- Story-specific validation completes before moving to lower-priority polish.

### Parallel Opportunities

- **Setup**: T003 and T004 can run in parallel after T001/T002 framing work
- **Foundational**: T006, T007, T009, and T010 can run in parallel; T012–T014 can run in parallel after their corresponding primitives exist
- **US1**: T016–T019 can run in parallel; T020 and T021 can run in parallel
- **US2**: T026–T028 can run in parallel; T031 and T033 can run in parallel
- **US3**: T035–T038 can run in parallel; T039 and T040 can run in parallel
- **US4**: T044–T046 can run in parallel; T048 and T049 can run in parallel
- **US5**: T052–T054 can run in parallel; T058 and T059 can run in parallel
- **Polish**: T061 and T062 can run in parallel before T063

---

## Parallel Example: User Story 3

```bash
# Parallel tests for User Story 3
Task: "T035 [US3] /home/andy/gitrepos/noto/tests/unit/memory/keyword_search_tool_test.go"
Task: "T036 [US3] /home/andy/gitrepos/noto/tests/unit/memory/time_range_search_tool_test.go"
Task: "T037 [US3] /home/andy/gitrepos/noto/tests/integration/provider/tool_calling_roundtrip_test.go"
Task: "T038 [US3] /home/andy/gitrepos/noto/tests/integration/provider/tool_support_fallback_test.go"

# Parallel implementation for search executors
Task: "T039 [US3] /home/andy/gitrepos/noto/internal/memory/search_tools.go"
Task: "T040 [US3] /home/andy/gitrepos/noto/internal/memory/search_tools.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Confirm timeline-based context assembly works independently
5. Demo/deploy the MVP slice if desired

### Incremental Delivery

1. Deliver US1 configurable timeline context assembly
2. Deliver US2 rollup creation/regeneration
3. Deliver US3 OpenRouter tool-calling search tools
4. Deliver US4 cache correctness for the broadened retrieval model
5. Deliver US5 diagnostics and footer telemetry
6. Finish with cross-cutting cleanup and validation

### Parallel Team Strategy

With multiple developers:

1. Complete Setup + Foundational together
2. Then split by story focus:
   - Developer A: US1/US2 memory timeline + rollups
   - Developer B: US3 provider tool calling + search execution
   - Developer C: US4/US5 cache identity + footer telemetry + diagnostics
3. Re-integrate through the shared validation scenarios in `quickstart.md`

---

## Notes

- This task set focuses on the newly adjusted timeline-memory, tool-calling, cache, and footer-telemetry scope.
- Legacy extraction and prompt-bootstrap requirements already implemented in the product are intentionally not re-tasked here.
- Traceability by scope:
  - US1 covers timeline window settings and default context assembly
  - US2 covers weekly/monthly rollup generation, uniqueness, and regeneration
  - US3 covers LLM memory search tools and tool-calling integration
  - US4 covers cache identity, invalidation, and stale-while-revalidate behavior
  - US5 covers diagnostics, model context metadata, and footer telemetry
- All tasks use the required checklist format with checkbox, ID, optional `[P]`, story label where applicable, and exact file paths.
- Tasks are specific enough to execute directly against the documented plan, contracts, and data model.
- Automated tests are included because the constitution and plan explicitly require them.
