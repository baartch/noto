# Tasks: Embedding Index for Noto Vector Layer

**Input**: Design documents from `/specs/20260328-120000-embedding-index/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED for every user story and must be created before implementation tasks.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [X] T001 Confirm existing Go module/lint setup in /home/andy/gitrepos/noto/go.mod and /home/andy/gitrepos/noto/.golangci.yml
- [X] T002 [P] Create vector index directories /home/andy/gitrepos/noto/internal/vector/hnsw and /home/andy/gitrepos/noto/internal/vector/file with doc.go stubs

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 Define vector file header + codec interfaces in /home/andy/gitrepos/noto/internal/vector/file/codec.go
- [X] T004 [P] Define HNSW graph interfaces and node storage in /home/andy/gitrepos/noto/internal/vector/hnsw/graph.go
- [X] T005 Add embedding request/response support to provider adapter in /home/andy/gitrepos/noto/internal/provider/adapter.go
- [X] T006 Implement OpenAI-compatible embeddings call in /home/andy/gitrepos/noto/internal/provider/openai_compatible.go
- [X] T007 Extend vector index adapter to load/store memory.vec in /home/andy/gitrepos/noto/internal/vector/index.go

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Start and Chat with a Profile (Priority: P1) 🎯 MVP

**Goal**: Preserve chat/profile flows while wiring vector index warnings and embeddings support

**Independent Test**: Run chat with missing/corrupt memory.vec and verify warning + SQLite fallback

### Tests for User Story 1 (REQUIRED) ⚠️

- [X] T008 [P] [US1] Add vector warning/fallback integration test in /home/andy/gitrepos/noto/tests/integration/vector_rebuild_fallback_test.go

### Implementation for User Story 1

- [X] T009 [US1] Emit warning on missing/corrupt vector index in /home/andy/gitrepos/noto/internal/memory/retrieval.go
- [X] T010 [US1] Wire vector search to use cosine-normalized embeddings in /home/andy/gitrepos/noto/internal/vector/retrieval.go

**Checkpoint**: User Story 1 remains functional with vector warning behavior

---

## Phase 4: User Story 2 - Persistent Memory Continuity (Priority: P2)

**Goal**: Add embedding-backed vector sync, rebuild, and retrieval for memory notes

**Independent Test**: Sync notes, rebuild index, and retrieve top‑k results from memory.vec

### Tests for User Story 2 (REQUIRED) ⚠️

- [X] T011 [P] [US2] Add vector sync + retrieval integration test in /home/andy/gitrepos/noto/tests/integration/vector_sync_retrieval_test.go
- [X] T012 [P] [US2] Add vector rebuild integration test in /home/andy/gitrepos/noto/tests/integration/vector_rebuild_fallback_test.go
- [X] T013 [P] [US2] Add vector lookup benchmark asserting p95 < 40ms in /home/andy/gitrepos/noto/tests/integration/performance_bench_test.go

### Implementation for User Story 2

- [X] T014 [US2] Implement vector file read/write and persistence in /home/andy/gitrepos/noto/internal/vector/file/codec.go
- [X] T015 [P] [US2] Implement HNSW insert/search in /home/andy/gitrepos/noto/internal/vector/hnsw/graph.go
- [X] T016 [US2] Implement embedding generation for memory notes in /home/andy/gitrepos/noto/internal/vector/sync.go
- [X] T017 [US2] Implement vector search using HNSW + cosine similarity in /home/andy/gitrepos/noto/internal/vector/index.go
- [X] T018 [US2] Update vector rebuild flow to repopulate memory.vec in /home/andy/gitrepos/noto/internal/vector/rebuild.go
- [X] T019 [US2] Update backup snapshots to include memory.vec consistently in /home/andy/gitrepos/noto/internal/backup/filesystem.go

**Checkpoint**: User Story 2 delivers embedding-backed retrieval with explicit rebuild

---

## Phase 5: User Story 3 - Manage Profiles and Prompts Safely (Priority: P3)

**Goal**: Preserve isolation and manifest metadata rules for vector indexes

**Independent Test**: Verify vector manifest entries remain profile-scoped and correct after profile changes

### Tests for User Story 3 (REQUIRED) ⚠️

- [X] T020 [P] [US3] Add profile isolation test for vector manifest entries in /home/andy/gitrepos/noto/tests/integration/profile_isolation_safety_test.go

### Implementation for User Story 3

- [X] T021 [US3] Enforce manifest metadata usage for memory.vec entries in /home/andy/gitrepos/noto/internal/store/vector_manifest_repo.go
- [X] T022 [US3] Update vector invalidation to mark manifest stale on prompt changes in /home/andy/gitrepos/noto/internal/vector/invalidation.go

**Checkpoint**: User Story 3 maintains strict isolation with vector metadata in SQLite

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [X] T023 [P] Update operations docs for memory.vec handling in /home/andy/gitrepos/noto/docs/operations.md
- [X] T024 Validate quickstart steps in /home/andy/gitrepos/noto/specs/20260328-120000-embedding-index/quickstart.md
- [X] T025 Enforce performance benchmark reporting for vector lookup in /home/andy/gitrepos/noto/tests/integration/performance_bench_test.go

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2)
- **User Story 2 (P2)**: Can start after Foundational (Phase 2); builds on vector primitives
- **User Story 3 (P3)**: Can start after Foundational (Phase 2)

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Models before services
- Services before endpoints
- Core implementation before integration

### Parallel Opportunities

- T002, T004 can run in parallel
- T011–T013 can run in parallel
- T014–T015 can run in parallel

---

## Parallel Example: User Story 2

```bash
Task: "Add vector sync + retrieval integration test in /home/andy/gitrepos/noto/tests/integration/vector_sync_retrieval_test.go"
Task: "Add vector rebuild integration test in /home/andy/gitrepos/noto/tests/integration/vector_rebuild_fallback_test.go"
Task: "Add vector lookup benchmark asserting p95 < 40ms in /home/andy/gitrepos/noto/tests/integration/performance_bench_test.go"

Task: "Implement vector file read/write and persistence in /home/andy/gitrepos/noto/internal/vector/file/codec.go"
Task: "Implement HNSW insert/search in /home/andy/gitrepos/noto/internal/vector/hnsw/graph.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. STOP and validate User Story 1 independently

### Incremental Delivery

1. Setup + Foundational
2. User Story 1 → validate
3. User Story 2 → validate
4. User Story 3 → validate
5. Polish phase
