# Implementation Plan: Noto Profile Memory CLI

**Branch**: `001-build-profile-memory-cli` | **Date**: 2026-03-27 | **Spec**: [/home/andy/gitrepos/noto/specs/001-build-profile-memory-cli/spec.md](/home/andy/gitrepos/noto/specs/001-build-profile-memory-cli/spec.md)
**Input**: Feature specification from `/specs/001-build-profile-memory-cli/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Build Noto as a local-first Go terminal chatbot with consistent TUI UX, provider-agnostic chat,
strict profile isolation, SQLite-backed memory continuity, and full CLI-to-chat slash command
parity. Slash mode supports canonical hierarchical syntax, explicit disambiguation, unknown-command
error + suggestions, and live suggestions while typing after `/`.

## Technical Context

**Language/Version**: Go 1.23+  
**Primary Dependencies**: Cobra (command definitions), Bubble Tea + Bubbles + Lip Gloss (TUI),
provider adapter layer for OpenAI-compatible APIs, `modernc.org/sqlite`  
**Storage**: Local SQLite per profile (`~/.noto/profiles/<profile>/memory.db`) + profile-local
backup snapshots + local prompt files + local context cache  
**Testing**: `go test` (unit, integration, contract tests)  
**Target Platform**: Local terminal on macOS/Linux  
**Project Type**: Single-binary CLI/TUI application  
**Performance Goals**: p95 startup < 1.5s (warm), p95 first contextual response assembly < 700ms
(cache hit) / < 2.0s (cache miss), p95 profile command feedback < 150ms, p95 slash suggestion
refresh < 50ms per keystroke  
**Constraints**: Local-first only; encrypted provider credentials; profile DB/prompt/cache local;
strict per-profile isolation; no cloud persistence; no provider lock-in; destructive confirmations;
slash command parity with CLI  
**Scale/Scope**: Single-user workstation; 1–50 profiles; up to 100k memory notes/profile;
out-of-scope: multi-user sync, cloud backup, vector as source-of-truth

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Code Quality Gate**: PASS
  - Single command registry shared by CLI and slash execution paths.
  - Lint/static analysis + explicit error handling in parser/dispatcher.
- **Testing Standards Gate**: PASS
  - Tests cover parity, disambiguation, unknown command suggestions, destructive confirmation.
  - Regression tests for parser edge cases and suggestion visibility rules.
- **UX Consistency Gate**: PASS
  - Canonical hierarchical command syntax in both CLI and chat.
  - Slash suggestions only in slash mode; explicit selection for ambiguity.
- **Performance Gate**: PASS
  - Suggestion latency budget defined and benchmarked.
  - Existing startup/retrieval/cache/recovery budgets preserved.

**Post-Design Re-check**: PASS
- `research.md` defines slash parity architecture and parser behavior.
- `data-model.md` includes slash command and suggestion event entities.
- `contracts/cli-commands.md` codifies slash grammar + suggestion contract.
- `quickstart.md` validates slash UX and parity scenarios.

## Project Structure

### Documentation (this feature)

```text
specs/001-build-profile-memory-cli/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── cli-commands.md
└── tasks.md
```

### Source Code (repository root)

```text
cmd/
└── noto/
    └── main.go

internal/
├── app/                 # startup orchestration, command registry wiring
├── tui/                 # Bubble Tea models/views/update loop
├── profile/             # profile lifecycle + prompt management
├── chat/                # chat turn pipeline + slash dispatch integration
├── commands/            # canonical command specs + shared handlers
├── parser/              # slash lexer/parser and argument handling
├── suggest/             # command suggestion ranking and filtering
├── provider/            # provider adapters + normalized errors
├── memory/              # note extraction, retrieval, summary generation
├── cache/               # context cache build/invalidate/load
├── backup/              # periodic/session-end snapshot + restore orchestration
├── security/            # credential encryption/decryption helpers
├── store/               # sqlite repositories, migrations
├── observe/             # structured logs + local metrics emission
└── config/              # ~/.noto path and active profile config

tests/
├── unit/
├── integration/
└── contract/
```

**Structure Decision**: Domain packages with dedicated `commands/parser/suggest` layers to ensure
clear separation between command definition, parsing, suggestion UX, and execution.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
