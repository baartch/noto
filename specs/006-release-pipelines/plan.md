# Implementation Plan: Release Publishing & Update Checks

**Branch**: `006-release-pipelines` | **Date**: 2026-04-13 | **Spec**: /home/andy/gitrepos/noto/specs/006-release-pipelines/spec.md
**Input**: Feature specification from `/specs/006-release-pipelines/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

GitHub Actions release pipeline builds Linux/Windows/macOS artifacts, runs `make tidy fmt vet lint test` before publishing, and creates GitHub Releases with notes/version metadata. Add semantic versioning support in the binary (embedded build version), startup update check against GitHub Releases API with async/non-blocking notice in CLI/TUI, and expose current version via CLI.

## Technical Context

**Language/Version**: Go 1.26  
**Primary Dependencies**: Cobra, Bubble Tea/Bubbles, Lip Gloss, modernc.org/sqlite, golang.org/x/mod/semver (new)  
**Storage**: SQLite per profile (`~/.noto/profiles/<profile>/memory.db`), profile metadata files  
**Testing**: `go test ./...`, `go test ./tests/integration/...`, `go test ./tests/contract/...`  
**Target Platform**: Linux/macOS/Windows CLI + TUI  
**Project Type**: CLI/TUI desktop app  
**Performance Goals**: Update check completes <1s p95 without blocking startup  
**Constraints**: Offline-capable, non-blocking startup, GitHub-hosted releases, semantic versioning  
**Scale/Scope**: Single-user local app; single binary per OS

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Code Quality Gate**: Release workflow runs `make tidy fmt vet lint test` before publish; CI stays on PRs. Go format (`gofmt`), `go vet`, `golangci-lint` enforced.
- **Testing Standards Gate**: Add tests for update check (success/no update/error) and CLI/TUI notice paths; release workflow runs full test suite; contract coverage for version output.
- **UX Consistency Gate**: Reuse existing CLI/TUI notification styles; update notice is non-blocking and uses established messaging pattern.
- **Performance Gate**: Update check async with timeout budget; measure via unit test timing + manual timing check to ensure <1s p95 startup impact.

## Project Structure

### Documentation (this feature)

```text
specs/006-release-pipelines/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
cmd/
└── noto/

internal/
├── app/
├── chat/
├── commands/
├── config/
├── observe/
├── profile/
├── provider/
├── store/
├── tui/
├── update/
└── vector/

tests/
├── contract/
└── integration/
```

**Structure Decision**: Single Go module with `cmd/` entrypoint, `internal/` packages, and integration/contract tests.

## Constitution Check (Post-Design)

- **Code Quality Gate**: PASS — workflow includes `make tidy fmt vet lint test` before release; CI unchanged.
- **Testing Standards Gate**: PASS — plan includes unit/contract tests for update check + version output.
- **UX Consistency Gate**: PASS — update notice uses existing CLI/TUI messaging patterns.
- **Performance Gate**: PASS — async check with timeout + validation steps.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
