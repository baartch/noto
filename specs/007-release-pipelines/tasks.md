---
description: "Task list for Release Publishing & Update Checks"
---

# Tasks: Release Publishing & Update Checks

**Input**: Design documents from `/specs/007-release-pipelines/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED for every user story and must be created before implementation tasks.

## Phase 1: Setup (Shared Infrastructure)

- [ ] T001 Create release workflow skeleton in /home/andy/gitrepos/noto/.github/workflows/release.yml
- [ ] T002 [P] Add version package scaffold in /home/andy/gitrepos/noto/internal/version/version.go (Version, Commit, Date defaults)

---

## Phase 2: Foundational (Blocking Prerequisites)

- [ ] T003 Define GitHub repo constants/config in /home/andy/gitrepos/noto/internal/config/release.go (owner, repo, base URL)
- [ ] T004 Implement update check client in /home/andy/gitrepos/noto/internal/update/checker.go (GitHub Releases API, semver compare, ignore pre-releases, timeout; depends on T003 release config)
- [ ] T005 [P] Add helper for async startup check in /home/andy/gitrepos/noto/internal/app/update_notice.go (non-blocking invocation)

---

## Phase 3: User Story 1 - Publish multi-OS release (Priority: P1) 🎯 MVP

**Goal**: GitHub Actions publishes releases with Linux/Windows/macOS artifacts and notes.

**Independent Test**: Push a tag and verify three artifacts + release notes are published.

### Tests for User Story 1 (REQUIRED) ⚠️

- [ ] T006 [P] [US1] Add actionlint check for release workflow in /home/andy/gitrepos/noto/.github/workflows/ci.yml

### Implementation for User Story 1

- [ ] T007 [US1] Implement build matrix + artifact packaging in /home/andy/gitrepos/noto/.github/workflows/release.yml (linux/windows/macos)
- [ ] T008 [US1] Add `make tidy fmt vet lint test` gate before publish in /home/andy/gitrepos/noto/.github/workflows/release.yml
- [ ] T009 [US1] Add fail-fast on build failure in /home/andy/gitrepos/noto/.github/workflows/release.yml
- [ ] T010 [US1] Add ldflags version injection in /home/andy/gitrepos/noto/.github/workflows/release.yml
- [ ] T011 [US1] Configure GitHub Release publish step in /home/andy/gitrepos/noto/.github/workflows/release.yml (notes + artifacts)
- [ ] T028 [US1] Add semantic tag validation step in /home/andy/gitrepos/noto/.github/workflows/release.yml (fail publish unless tag matches `vMAJOR.MINOR.PATCH`)

**Checkpoint**: User Story 1 release pipeline publishes multi-OS artifacts with release notes.

---

## Phase 4: User Story 2 - User sees update availability on startup (Priority: P2)

**Goal**: Non-blocking update notice shown in CLI and TUI when newer version exists.

**Independent Test**: Start app with newer GitHub Release; observe notice in CLI/TUI without delaying startup.

### Tests for User Story 2 (REQUIRED) ⚠️

- [ ] T012 [P] [US2] Add update checker unit tests in /home/andy/gitrepos/noto/internal/update/checker_test.go (newer/same/error/ignore pre-release)
- [ ] T013 [P] [US2] Add CLI startup notice test in /home/andy/gitrepos/noto/internal/app/update_notice_test.go (non-blocking)
- [ ] T014 [P] [US2] Add TUI notice message test in /home/andy/gitrepos/noto/internal/tui/update_notice_test.go

### Implementation for User Story 2

- [ ] T015 [US2] Wire update check at startup in /home/andy/gitrepos/noto/internal/app/root.go (trigger async check)
- [ ] T016 [US2] Add CLI notice output in /home/andy/gitrepos/noto/internal/app/update_notice.go (stdout/stderr messaging)
- [ ] T017 [US2] Add TUI update notice message + rendering in /home/andy/gitrepos/noto/internal/tui/model.go
- [ ] T018 [US2] Ensure timeout + error suppression in /home/andy/gitrepos/noto/internal/update/checker.go

**Checkpoint**: Update notice appears in CLI/TUI without blocking startup.

---

## Phase 5: User Story 3 - Team verifies versioning consistency (Priority: P3)

**Goal**: Semantic versioning exposed and consistent across releases.

**Independent Test**: Run `noto version` and verify semver output aligns with release tags.

### Tests for User Story 3 (REQUIRED) ⚠️

- [ ] T019 [P] [US3] Add contract test for `noto version` in /home/andy/gitrepos/noto/tests/contract/version_cmd_test.go

### Implementation for User Story 3

- [ ] T020 [US3] Implement `version` Cobra command in /home/andy/gitrepos/noto/internal/app/version_cmd.go
- [ ] T021 [US3] Register version command in /home/andy/gitrepos/noto/internal/app/root.go
- [ ] T022 [US3] Use version package in /home/andy/gitrepos/noto/internal/version/version.go for output formatting (dev fallback)
- [ ] T029 [US3] Add TUI footer version rendering test in /home/andy/gitrepos/noto/internal/tui/footer_version_test.go (bottom-right placement)
- [ ] T030 [US3] Render current version in TUI footer (bottom-right) in /home/andy/gitrepos/noto/internal/tui/model.go

**Checkpoint**: `noto version` outputs semantic version string.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T023 [P] Update docs for release/version usage in /home/andy/gitrepos/noto/README.md
- [ ] T024 Validate quickstart steps in /home/andy/gitrepos/noto/specs/007-release-pipelines/quickstart.md
- [ ] T025 Add UX consistency validation note for CLI/TUI update notice in /home/andy/gitrepos/noto/specs/007-release-pipelines/quickstart.md
- [ ] T026 Add workflow validation checklist entry in /home/andy/gitrepos/noto/specs/007-release-pipelines/quickstart.md
- [ ] T031 Add PR requirement-to-test traceability checklist section in /home/andy/gitrepos/noto/.github/pull_request_template.md (FR-to-test mapping table for changed FRs)
- [ ] T032 Document traceability usage in /home/andy/gitrepos/noto/specs/007-release-pipelines/quickstart.md (how to map FR-001–FR-009 to tests in PR)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **Foundational (Phase 2)**: Depends on Setup
- **User Stories (Phase 3+)**: Depend on Foundational
- **Polish (Phase 6)**: Depends on desired user stories complete

### User Story Dependencies

- **US1**: Can start after Phase 2
- **US2**: Can start after Phase 2; uses version/update client
- **US3**: Can start after Phase 2; uses version package

### Parallel Opportunities

- T001/T002 in parallel
- T003/T005 in parallel; T004 starts after T003 (requires release config constants)
- US1 tasks T007-T011, T028 are sequential in same file (no [P])
- US2 tests T012-T014 in parallel; implementation tasks T015-T018 mostly sequential (shared files)
- US3 test T019 can run in parallel with US2 tests

---

## Parallel Example: User Story 2

```bash
Task: "Add update checker unit tests in /home/andy/gitrepos/noto/internal/update/checker_test.go"
Task: "Add CLI startup notice test in /home/andy/gitrepos/noto/internal/app/update_notice_test.go"
Task: "Add TUI notice message test in /home/andy/gitrepos/noto/internal/tui/update_notice_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1: Setup
2. Phase 2: Foundational
3. Phase 3: US1
4. Validate release pipeline

### Incremental Delivery

1. Setup + Foundational
2. US1 → publish pipeline
3. US2 → update notice
4. US3 → version command
5. Polish
