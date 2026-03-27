# Phase 0 Research: Noto Profile Memory CLI

## Decision: Go + Cobra + Bubble Tea stack

- **Decision**: Use Go 1.23+, Cobra for command layer, Bubble Tea/Bubbles/Lip Gloss for TUI.
- **Rationale**: Mature ecosystem, strong terminal UX patterns, low runtime footprint, fast single
  binary distribution.
- **Alternatives considered**:
  - Pure stdlib terminal handling: lower deps, but higher UX and state-management complexity.
  - Other TUI libs: viable, but less composable update/view model for chat-heavy flows.

## Decision: Provider abstraction around OpenAI-compatible protocol

- **Decision**: Define provider interface with adapters for GitHub Models, OpenRouter, local
  OpenAI-compatible endpoints, and generic OpenAI-compatible APIs.
- **Rationale**: Satisfies no-lock-in requirement, centralizes retries/error normalization,
  minimizes chat-layer branching.
- **Alternatives considered**:
  - Single provider SDK only: violates product requirement.
  - Provider logic inside chat package: increases coupling and test complexity.

## Decision: SQLite per profile as source of truth

- **Decision**: One `memory.db` per profile; profile data not shared across profiles.
- **Rationale**: Strong isolation boundary, local durability, operational simplicity.
- **Alternatives considered**:
  - Shared DB for all profiles: increases accidental cross-profile query risk.
  - Files-only memory: weaker query/retrieval capabilities and consistency guarantees.

## Decision: Continuity model = notes + session summary + cache

- **Decision**: Persist memory notes and session summary, plus per-profile context cache for
  performance. Cache is secondary, never source of truth.
- **Rationale**: Balances continuity quality, prompt size control, and latency/cost stability.
- **Alternatives considered**:
  - Full transcript replay each session: prompt bloat, latency growth, token-cost growth.
  - Cache-only memory: weak recoverability and correctness.

## Decision: Data protection scope

- **Decision**: Encrypt provider credentials at rest; keep profile DB/prompts/cache local
  plaintext files under OS file permissions.
- **Rationale**: Protects highest-risk secrets while preserving SQLite and retrieval performance
  with low implementation overhead.
- **Alternatives considered**:
  - Full-dataset encryption: stronger protection but higher complexity/perf overhead.
  - No encryption at rest: insufficient credential protection.

## Decision: Reliability and backup policy

- **Decision**: On DB corruption, attempt automatic repair first; if unsuccessful, restore latest
  profile-local backup. Backup cadence = periodic + session-end snapshots.
- **Rationale**: Deterministic recovery behavior with bounded work loss and minimal user friction.
- **Alternatives considered**:
  - Manual restore only: slower recovery, higher user burden.
  - Daily-only backup cadence: too large potential data loss window.

## Decision: Observability policy (local)

- **Decision**: Emit structured local logs and profile-scoped local metrics for startup,
  retrieval, cache, provider calls, and recovery flows.
- **Rationale**: Debuggability and validation without external telemetry dependency.
- **Alternatives considered**:
  - Error-only logging: insufficient for performance/reliability analysis.
  - Unstructured logs only: weaker machine analysis and contract test assertions.

## Dependency health check (quick)

- Cobra: high adoption, active maintenance.
- Bubble Tea ecosystem: active releases, broad TUI usage.
- modernc SQLite driver: avoids CGO dependency for simpler distribution.

No unresolved clarifications remain.
