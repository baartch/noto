# Feature Specification: Release Publishing & Update Checks

**Feature Branch**: `007-release-pipelines`  
**Created**: 2026-04-13  
**Status**: Draft  
**Input**: User description: "Lets publish releases for linux, windows and macos on github. Therefor we need deployment pipelines (Github actions) and versioning. Ideally noto checks at startup for new versions."

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Publish multi-OS release (Priority: P1)

Release manager publishes a new version with installable packages for Linux, Windows, and macOS on the project’s release page.

**Why this priority**: Core business need; enables distribution to users.

**Independent Test**: Trigger a release and verify three platform artifacts and release notes are visible to users.

**Acceptance Scenarios**:

1. **Given** a new version is ready, **When** the release process runs, **Then** a public release is created with Linux, Windows, and macOS artifacts.
2. **Given** the release completes, **When** a user views the release page, **Then** they see version number and notes for that release.

---

### User Story 2 - User sees update availability on startup (Priority: P2)

User runs the app (CLI or TUI) and is informed if a newer version is available.

**Why this priority**: Improves user awareness and adoption of new releases.

**Independent Test**: Start the app with a newer release available and verify the update notice appears without blocking use.

**Acceptance Scenarios**:

1. **Given** a newer version exists, **When** the app starts, **Then** the user sees a clear update notice.
2. **Given** no newer version exists, **When** the app starts, **Then** no update notice is shown.

---

### User Story 3 - Team verifies versioning consistency (Priority: P3)

Team confirms versions follow a consistent, predictable scheme across releases.

**Why this priority**: Reduces confusion, supports support/debug workflows.

**Independent Test**: Compare consecutive releases and verify version format and ordering rules are consistent.

**Acceptance Scenarios**:

1. **Given** two consecutive releases, **When** reviewing their versions, **Then** the version scheme is consistent and ordered.

---

### Edge Cases

- If any platform build fails, the release is not published.
- How does the app handle update checks when network is unavailable; startup continues and no blocking error.

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: System MUST create a public release on the GitHub Releases page for each new version with artifacts for Linux, Windows, and macOS.
- **FR-002**: System MUST include release notes and version metadata with each release.
- **FR-003**: System MUST use semantic versioning (MAJOR.MINOR.PATCH) across all releases, using tags in the format `v0.0.0`.
- **FR-008**: Update checks MUST ignore pre-release tags (e.g., `-alpha`, `-beta`, `-rc`).
- **FR-004**: App MUST check for newer available versions on startup.
- **FR-005**: App MUST notify the user in a non-blocking way when a newer version is available in both CLI and TUI modes.
- **FR-006**: App MUST continue to start normally if the update check fails or is unavailable.
- **FR-007**: Users MUST be able to identify current installed version within the app or CLI output.
- **FR-009**: Release publishing MUST fail if any platform build fails.

### Non-Functional Requirements _(mandatory)_

- **NFR-001 Code Quality**: Changes MUST pass formatting, linting, and static analysis rules
  defined by the project.
- **NFR-002 Testing Standards**: Changes MUST include automated tests for new/changed behavior,
  including negative/error paths where applicable.
- **NFR-003 UX Consistency**: User-facing changes MUST follow established UX patterns
  (terminology, interaction flows, visual behavior) or document approved deviations.

### Key Entities _(include if feature involves data)_

- **Release**: Public distribution of a version; includes version number, notes, date.
- **Artifact**: Platform-specific package linked to a Release (Linux/Windows/macOS).
- **Version**: Ordered identifier for a Release; used for comparison and update checks.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: Every published release includes three platform artifacts (Linux, Windows, macOS).
- **SC-003**: 90% of users with an outdated version see an update notice on first startup after release.
- **SC-004**: Release notes present for 100% of new releases.
- **SC-005**: 0 lint/format violations in CI for feature scope.
- **SC-006**: 100% of new/changed behaviors have automated test coverage.

## Clarifications

### Session 2026-04-13

- Q: Should the update notice appear in CLI, TUI, or both? → A: Both CLI and TUI modes.
- Q: Which versioning scheme should releases follow? → A: Semantic versioning (MAJOR.MINOR.PATCH).
- Q: Should the update notice block startup? → A: Non-blocking.
- Q: Where should releases be published? → A: Public GitHub Releases page.
- Q: What happens if any platform build fails? → A: Fail release; do not publish.

## Assumptions

- Source repository and releases are hosted on GitHub.
- Semantic versioning is used for all releases. (Confirmed)
- Update checks only inform; no auto-update install in v1.
- Users have intermittent network access; update check failures are non-fatal.
