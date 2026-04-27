<!--
Sync Impact Report
- Version change: 1.1.0 → 1.1.1
- Modified principles:
  - I. Code Quality Is Enforced (clarified required local verification commands)
- Added sections:
  - None
- Removed sections:
  - None
- Templates requiring updates:
  - ✅ no changes required: .specify/templates/plan-template.md
  - ✅ no changes required: .specify/templates/spec-template.md
  - ✅ no changes required: .specify/templates/tasks-template.md
  - ✅ no command templates present: .specify/templates/commands/*.md
- Follow-up TODOs:
  - None
-->

# Noto Constitution

## Core Principles

### I. Code Quality Is Enforced

All production code MUST pass formatter, linter, and static analysis checks before merge.
Local verification MUST run `make fmt`, `make lint`, and `make test` before opening or updating a PR.
Code MUST keep clear naming, small focused modules, and explicit error handling. Reviews MUST
reject unclear or unmaintainable code even if functionally correct.

Rationale: consistent quality reduces defects, review time, and long-term maintenance cost.

### II. Testing Standards Are Non-Negotiable

Every behavior change MUST include automated tests at the right level (unit, integration,
contract, end-to-end as applicable). Tests MUST cover success and failure paths. Regressions
MUST add a failing test first, then fix.

Rationale: test discipline prevents silent regressions and enables safe iteration.

### III. User Experience Consistency First

User-facing changes MUST follow existing UX patterns: terminology, interaction flow, visual
states, accessibility behavior, and error messaging. Any deviation MUST be documented in the
spec/plan and explicitly approved during review.

Rationale: consistent UX lowers cognitive load, improves trust, and reduces support overhead.

## Quality Standards

- Specifications MUST include non-functional requirements for code quality, testing, and UX
  consistency.
- Plans MUST include constitution gates for all three principles before implementation begins.
- Tasks MUST include required test work before implementation and explicit UX validation work.

## Delivery Workflow & Review Gates

- PRs MUST show: passing quality checks, passing tests, and UX consistency validation.
- Standard local validation command set is: `make fmt`, `make lint`, `make test`.
- Reviewers MUST block merges on any missing gate evidence.
- Release notes MUST include notable UX changes when user-visible.

## Governance

This constitution overrides conflicting local practice for planning, implementation, and review.
Amendments require: (1) documented proposal, (2) approval by maintainers, (3) template sync
across `.specify/templates/*`, and (4) migration notes for in-flight work.

Versioning policy (semantic):

- MAJOR: remove or redefine a principle/policy in a backward-incompatible way.
- MINOR: add principle/section or materially expand mandatory guidance.
- PATCH: clarifications, wording improvements, typo/non-semantic fixes.

Compliance review expectations:

- Every plan: Constitution Check before research/design and re-check before implementation.
- Every PR: explicit compliance confirmation for all core principles.
- Quarterly (or milestone) audit: spot-check recent specs/plans/tasks for alignment.

**Version**: 1.1.1 | **Ratified**: 2026-03-27 | **Last Amended**: 2026-04-27
