# Tasks: Messenger Chat UI History Scrolling

**Input**: Design documents from `/specs/009-messenger-chat-ui/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/ui-scroll-behavior.md, quickstart.md

**Tests**: Include tests (explicitly required by plan/constitution) for unit, integration, and contract-level behavior.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Every task includes an exact file path

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish constants, helper scaffolding, and baseline test harness updates for this feature.

- [X] T001 Create feature constants for conversation/input batch sizes and caps in `internal/tui/constants.go`
- [X] T002 [P] Define concrete TUI history state and helper method skeletons (`conversationHistoryWindow`, `inputHistoryWindow`, `loadInitialConversationHistory()`, `loadOlderConversationHistoryBatch()`, `loadInitialInputHistoryBatch()`) in `internal/tui/model.go`
- [X] T003 [P] Add concrete repository method signatures for conversation paging (`ListRecentByConversation`, `ListOlderByConversationBefore`) in `internal/store/message_repo.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core primitives required before user-story implementation.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T004 Implement store query to fetch latest N conversation messages in `internal/store/message_repo.go`
- [X] T005 [P] Implement store query to fetch older messages before cursor (id/timestamp) in `internal/store/message_repo.go`
- [X] T006 Implement active-profile most-recent conversation resolution helper in `internal/store/conversation_repo.go`
- [X] T007 Add/adjust shared mapping utility from store message to TUI chat message in `internal/tui/model.go`
- [X] T008 Add shared non-fatal history error state + rendering hook in `internal/tui/model.go`

**Checkpoint**: Foundation ready — user story implementation can now begin.

---

## Phase 3: User Story 1 - Conversation Continuity on Startup (Priority: P1) 🎯 MVP

**Goal**: On open/profile switch, show latest 10 conversation messages and land at bottom with graceful failure behavior.

**Independent Test**: Restart app with existing conversation and verify latest 10 render immediately at bottom; empty/no-history and load-failure remain usable.

### Tests for User Story 1

- [X] T009 [P] [US1] Add unit tests for latest-window conversion/order rules in `tests/unit/tui/history_window_test.go`
- [X] T010 [P] [US1] Add integration test for startup loading latest 10 messages in `tests/integration/tui_startup_history_test.go`
- [X] T011 [P] [US1] Add integration test for profile-switch reload behavior in `tests/integration/tui_profile_switch_history_test.go`
- [X] T012 [P] [US1] Add integration test for non-fatal startup history read failure in `tests/integration/tui_history_failure_test.go`

### Implementation for User Story 1

- [X] T013 [US1] Implement startup history load flow in `internal/tui/model.go`
- [X] T014 [US1] Wire startup/profile-switch message retrieval via repositories in `internal/app/chat_cmd.go`
- [X] T015 [US1] Ensure initial viewport position is bottom after load in `internal/tui/model.go`
- [X] T016 [US1] Implement empty-history rendering path without blocking input in `internal/tui/model.go`
- [X] T017 [US1] Implement non-fatal error indicator for load failures in `internal/tui/model.go`

**Checkpoint**: User Story 1 should be independently functional and testable.

---

## Phase 4: User Story 2 - Zone-Aware Mouse Wheel Scrolling (Priority: P2)

**Goal**: Route wheel input strictly by hover zone; add bounded input-history lazy loading and reset-after-send.

**Independent Test**: In messages area, wheel affects only history viewport; in input area, wheel affects only input-history window with 3/3/12 policy and reset after send.

### Tests for User Story 2

- [X] T018 [P] [US2] Add unit tests for scroll-zone detection and dispatch in `tests/unit/tui/scroll_zone_routing_test.go`
- [X] T019 [P] [US2] Add unit tests for input-history lazy-load policy (0 preload, 3 first, +3, cap 12) in `tests/unit/tui/input_history_window_test.go`
- [X] T020 [P] [US2] Add integration test for wheel isolation (messages zone does not mutate input) in `tests/integration/tui_scroll_zone_isolation_test.go`
- [X] T021 [P] [US2] Add integration test for wheel isolation (input zone does not move messages viewport) in `tests/integration/tui_scroll_zone_isolation_test.go`
- [X] T022 [P] [US2] Add integration test that Page Up/Page Down always scroll messages history regardless of hover zone in `tests/integration/tui_pagekey_history_routing_test.go`
- [X] T023 [P] [US2] Add integration test for clear-in-memory-input-history after send in `tests/integration/tui_input_history_reset_test.go`
- [X] T024 [P] [US2] Add contract test for input-history policy rules from UI contract in `tests/contract/tui_input_history_contract_test.go`

### Implementation for User Story 2

- [X] T025 [US2] Implement mouse hover zone calculation (`messages`/`input`/`outside`) in `internal/tui/model.go`
- [X] T026 [US2] Route mouse wheel events to zone-specific handlers only in `internal/tui/model.go`
- [X] T027 [US2] Ensure Page Up/Page Down always route to messages history scrolling, independent of hover zone, in `internal/tui/model.go`
- [X] T028 [US2] Remove overflow handoff behavior between textarea and message viewport in `internal/tui/model.go`
- [X] T029 [US2] Implement on-demand input-history first-load (latest 3) in `internal/tui/model.go`
- [X] T030 [US2] Implement incremental input-history backward lazy-load (+3) and cap enforcement (12) in `internal/tui/model.go`
- [X] T031 [US2] Preserve draft restoration and forward-navigation semantics with bounded window in `internal/tui/model.go`
- [X] T032 [US2] Clear in-memory input-history window immediately after send in `internal/tui/model.go`

**Checkpoint**: User Stories 1 and 2 both work independently.

---

## Phase 5: User Story 3 - Lazy Loading Older Messages by Scrolling (Priority: P3)

**Goal**: Load older conversation messages in batches of 10 when scrolling at top, preserving visual anchor.

**Independent Test**: With >10 messages, scrolling to top repeatedly prepends older batches of 10, stops at history end, and keeps visible anchor stable.

### Tests for User Story 3

- [X] T033 [P] [US3] Add unit tests for conversation lazy-load cursor and has-older transitions in `tests/unit/tui/conversation_paging_test.go`
- [X] T034 [P] [US3] Add integration test for top-boundary lazy-load prepend behavior in `tests/integration/tui_conversation_lazyload_test.go`
- [X] T035 [P] [US3] Add integration test for anchor preservation after prepend in `tests/integration/tui_conversation_anchor_test.go`
- [X] T036 [P] [US3] Add integration test for no-op at absolute history top in `tests/integration/tui_conversation_lazyload_test.go`
- [X] T037 [P] [US3] Add contract test for conversation paging rules from UI contract in `tests/contract/tui_conversation_scroll_contract_test.go`

### Implementation for User Story 3

- [X] T038 [US3] Implement top-boundary detection and lazy-load trigger in messages viewport in `internal/tui/model.go`
- [X] T039 [US3] Integrate older-message paged query into TUI loading flow in `internal/tui/model.go`
- [X] T040 [US3] Prepend older messages while preserving viewport visual anchor in `internal/tui/model.go`
- [X] T041 [US3] Stop additional fetches when no older messages remain in `internal/tui/model.go`
- [X] T042 [US3] Ensure new send/assistant reply snaps viewport back to bottom after mid-history browsing in `internal/tui/model.go`

**Checkpoint**: All user stories are independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finish quality gates, docs sync, and cross-story regression hardening.

- [X] T043 [P] Update feature validation procedure with any discovered nuances in `specs/009-messenger-chat-ui/quickstart.md`
- [X] T044 [P] Add/refresh regression coverage references in `specs/009-messenger-chat-ui/contracts/ui-scroll-behavior.md`
- [X] T045 Run full quality gates (`make fmt && make lint && make vet && make test`) and record outcomes in `specs/009-messenger-chat-ui/plan.md`
- [X] T046 Manually verify FR-012 baseline alignment (user messages right-aligned, assistant messages left-aligned) and record reviewer sign-off in `specs/009-messenger-chat-ui/plan.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies.
- **Phase 2 (Foundational)**: Depends on Phase 1; blocks all user stories.
- **Phase 3 (US1)**: Depends on Phase 2; serves as MVP.
- **Phase 4 (US2)**: Depends on Phase 2; can proceed after/alongside US1 if team capacity allows.
- **Phase 5 (US3)**: Depends on Phase 2 and reuses US1 history-loading primitives.
- **Phase 6 (Polish)**: Depends on completion of desired user stories.

### User Story Dependencies

- **US1 (P1)**: No dependency on other stories.
- **US2 (P2)**: Independent of US1 for core zone routing, but shares model file and should merge after US1 baseline is stable.
- **US3 (P3)**: Depends on conversation startup/history primitives introduced in US1.

### Dependency Graph

- Foundation: `T001 -> {T004,T005,T006} -> {T007,T008}`
- US1: `{T009,T010,T011,T012} -> {T013,T014,T015,T016,T017}`
- US2: `{T018,T019,T020,T021,T022,T023,T024} -> {T025,T026,T027,T028,T029,T030,T031,T032}`
- US3: `{T033,T034,T035,T036,T037} -> {T038,T039,T040,T041,T042}`
- Polish: `{T043,T044,T045,T046}` after selected stories complete

---

## Parallel Execution Examples

### User Story 1

```bash
# Parallel US1 tests
T009  T010  T011  T012

# Then implement sequence
T013 -> T014 -> T015 -> T016 -> T017
```

### User Story 2

```bash
# Parallel US2 tests
T018  T019  T020  T021  T022  T023  T024

# Parallel-safe implementation split (different concerns, same file requires staged merges)
T025/T026/T027/T028
T029/T030/T031
Then T032
```

### User Story 3

```bash
# Parallel US3 tests
T033  T034  T035  T036  T037

# Then paging implementation path
T038 -> T039 -> T040 -> T041 -> T042
```

---

## Implementation Strategy

### MVP First (US1)

1. Complete Setup + Foundational (Phases 1–2).
2. Complete US1 (Phase 3).
3. Validate independent test criteria for US1 via T010/T011/T012.
4. Demo/release MVP with conversation continuity.

### Incremental Delivery

1. Add US2 for deterministic scroll-zone behavior and bounded input-history loading.
2. Add US3 for long-history browsing with anchor-preserving lazy load.
3. Finish Polish phase and run full quality gates.

### Parallel Team Strategy

1. Team aligns on shared `internal/tui/model.go` edit boundaries.
2. One engineer handles store/repo tasks, one handles US2 routing/input-window logic, one handles US3 paging/anchor behavior.
3. Rebase frequently due to shared TUI file and run targeted tests before merge.

---

## Notes

- All tasks follow required checklist format: `- [ ] T### [P?] [US?] Description with file path`.
- Tests are included because planning docs explicitly require them.
- Prefer small, reviewable commits grouped by task or tightly-coupled task pair.
- Resolve `internal/tui/model.go` merge conflicts carefully due to heavy shared edits.
- FR-012 (user/right + assistant/left bubble alignment) is already implemented baseline behavior; this feature must not modify it, and verification will be done manually by the reviewer.
- SC-005 startup performance timing will not be instrumented in this feature; performance confidence is based on bounded loading design and manual validation.
