# Implementation Plan: Portable Profiles

**Branch**: `002-portable-profiles` | **Date**: 2026-03-29 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-portable-profiles/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Make profiles portable by ensuring all profile metadata lives inside each profile directory, removing any reliance on the global database for profile discovery or selection. Profiles are discovered via filesystem scanning, and active profile selection is stored outside the global DB.

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: Cobra (CLI), Bubble Tea/Bubbles/Lip Gloss (TUI)  
**Storage**: SQLite (per-profile DB), profile directory metadata files  
**Testing**: Go test (integration/contract tests)  
**Target Platform**: Local CLI/TUI on desktop platforms  
**Project Type**: CLI/TUI application  
**Performance Goals**: Profile discovery within 200 ms for up to 100 profiles  
**Constraints**: No profile metadata persisted in the global SQLite database  
**Scale/Scope**: Single-user local profiles, up to 100 profile directories

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

- **Code Quality Gate**: Continue to enforce gofmt/goimports and golangci-lint; no changes merge without clean lint runs.
- **Testing Standards Gate**: Add/adjust integration tests covering profile discovery, selection, and portability, including negative paths (missing metadata, duplicate names).
- **UX Consistency Gate**: Preserve existing CLI/TUI commands and messaging; any new error messages should follow current style and wording patterns.
- **Performance Gate**: Verify directory scan performance for up to 100 profiles with a benchmark or timed test.

## Project Structure

### Documentation (this feature)

```text
specs/002-portable-profiles/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
src/

tests/
├── contract/
└── integration/
```

**Structure Decision**: The project is a single CLI/TUI codebase under `src/` with tests under `tests/`.

## Complexity Tracking

No constitution violations anticipated.

## Phase 0: Outline & Research

### Research Tasks

- Validate current profile discovery workflow and identify all global DB dependencies.
- Identify best practice for portable profile metadata storage within the profile directory.
- Confirm active profile selection storage can move outside the global DB without breaking flows.

### Research Output

- **Output**: `specs/002-portable-profiles/research.md`

## Phase 1: Design & Contracts

### Data Model

- Define profile metadata file contents and required fields.
- Define how active profile selection is stored without the global DB.

**Output**: `specs/002-portable-profiles/data-model.md`

### Contracts

- Document CLI/TUI profile discovery and selection behaviors.

**Output**: `specs/002-portable-profiles/contracts/profile-discovery.md`

### Quickstart

- Provide a short guide for creating, moving, and selecting profiles with the new portable layout.

**Output**: `specs/002-portable-profiles/quickstart.md`

### Agent Context Update

- Run `/home/andy/gitrepos/noto/.specify/scripts/bash/update-agent-context.sh pi`.

## Phase 2: Planning

- Phase 2 planning will be produced by `/speckit.tasks`.
