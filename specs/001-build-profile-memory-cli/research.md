# Phase 0 Research: Noto Profile Memory CLI

## Decision: Full CLI-to-slash parity via canonical command registry

- **Decision**: Define commands once in shared registry; execute via both Cobra CLI and chat slash
  dispatcher.
- **Rationale**: Ensures FR-022 parity and consistent side effects.
- **Alternatives considered**:
  - Duplicate CLI/slash handlers: drift risk and test duplication.

## Decision: Canonical slash syntax + explicit ambiguity handling

- **Decision**: Use hierarchical syntax (`/profile list`, `/prompt show`) and require explicit
  selection for ambiguous matches.
- **Rationale**: Predictable UX, safer behavior, no accidental command execution.
- **Alternatives considered**:
  - Flat aliases only: weak CLI consistency.
  - Auto-execute first match: unsafe for destructive commands.

## Decision: Suggestion behavior and visibility

- **Decision**: Suggestions visible only when input starts with `/`; show top matches with usage
  hints; unknown slash command returns explicit error + top suggestions.
- **Rationale**: Keeps normal chat uncluttered while making slash mode discoverable.
- **Alternatives considered**:
  - Always-on suggestions: noisy for normal chat.
  - Error-only unknown handling: slower recovery.

## Decision: Existing architecture and constraints retained

- **Decision**: Keep Go/Cobra/Bubble Tea stack, profile-local SQLite source-of-truth, cache as
  performance layer, encrypted credentials only, deterministic recovery and backups.
- **Rationale**: New slash behavior composes with existing architecture.
- **Alternatives considered**:
  - No structural changes required beyond parser/suggest layers.

No unresolved clarifications remain.
