# Implementation Plan: Noto Profile Memory CLI

**Branch**: `001-build-profile-memory-cli` | **Date**: 2026-03-27 | **Spec**: [/home/andy/gitrepos/noto/specs/001-build-profile-memory-cli/spec.md](/home/andy/gitrepos/noto/specs/001-build-profile-memory-cli/spec.md)
**Input**: Feature specification from `/specs/001-build-profile-memory-cli/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Build Noto as a local-first Go terminal chatbot with consistent TUI UX, provider-agnostic chat,
strict per-profile isolation, and SQLite-backed persistent memory. Add session-handoff summaries,
per-profile context cache, encrypted credential storage, deterministic corruption recovery
(auto-repair then backup restore), periodic plus session-end backups, and local structured
observability.

## Technical Context

**Language/Version**: Go 1.23+  
**Primary Dependencies**: Cobra (CLI command surface), Bubble Tea + Bubbles + Lip Gloss (TUI),
provider adapter layer for OpenAI-compatible APIs, `modernc.org/sqlite` (embedded SQLite)  
**Storage**: Local SQLite per profile (`~/.noto/profiles/<profile>/memory.db`) + profile-local
backup snapshots + local prompt files + local cache artifacts  
**Testing**: `go test` (unit, integration, contract-style CLI/TUI behavior tests)  
**Target Platform**: Local terminal on macOS/Linux  
**Project Type**: Single-binary CLI/TUI application  
**Performance Goals**: p95 startup < 1.5s (warm), p95 first contextual response assembly < 700ms
(cache hit) / < 2.0s (cache miss), p95 profile command feedback < 150ms  
**Constraints**: Local-first only; credentials encrypted at rest; non-credential profile artifacts
local plaintext under OS permissions; strict profile isolation; no cloud persistence; no provider
lock-in; explicit deletion confirmation; low dependency footprint  
**Scale/Scope**: Single-user workstation; 1–50 profiles; up to 100k memory notes/profile;
out-of-scope: multi-user sync, cloud backup, vector as source-of-truth

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Code Quality Gate**: PASS
  - Enforce `go fmt` + lint + static analysis in CI/local checks.
  - Package boundaries keep modules focused (`profile`, `chat`, `memory`, `cache`, `store`).
- **Testing Standards Gate**: PASS
  - Unit: extraction, cache invalidation, encryption/decryption, backup policy decisions.
  - Integration: startup flows, profile lifecycle, continuity across sessions, recovery flow.
  - Contract: CLI/TUI behavior, confirmation prompts, error-path handling.
- **UX Consistency Gate**: PASS
  - One interaction model for startup/profile selection/chat/recovery prompts.
  - Consistent terminology: profile, memory note, session summary, context cache.
- **Performance Gate**: PASS
  - Budgets defined above.
  - Benchmarks planned for cache hit/miss, retrieval latency, startup, recovery overhead.

**Post-Design Re-check**: PASS
- `research.md` resolves architecture choices and risk tradeoffs.
- `data-model.md` defines entities, isolation, backup, and observability records.
- `contracts/cli-commands.md` defines runtime behavior and failure handling guarantees.
- `quickstart.md` includes validation scenarios for performance, reliability, and observability.

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
├── app/                 # startup orchestration, active profile flow
├── tui/                 # Bubble Tea models/views/update loop
├── profile/             # create/list/select/rename/delete, prompt management
├── chat/                # turn pipeline, session lifecycle
├── provider/            # provider adapters + normalized errors
├── memory/              # note extraction, retrieval, summary generation
├── cache/               # context cache build/invalidate/load
├── backup/              # periodic/session-end snapshot + restore orchestration
├── security/            # credential encryption/decryption helpers
├── store/               # sqlite repositories, migrations, transactional boundaries
├── observe/             # structured logs + local metrics emission
└── config/              # ~/.noto path and active profile config

tests/
├── unit/
├── integration/
└── contract/
```

**Structure Decision**: Single project, domain-split packages. Isolation and safety-critical logic
(backup/security/cache/profile boundaries) separated for testability and clearer failure handling.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
