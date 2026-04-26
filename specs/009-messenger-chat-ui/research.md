# Research: Messenger Chat UI History Scrolling

## Decision 1: Use cursor-position-based scroll-zone routing

- **Decision**: Route mouse wheel events to `messages` or `input` zone strictly by mouse cursor location.
- **Rationale**: This directly resolves current UX friction where textarea consumes wheel events before history scrolls. It aligns with user mental model and existing TUI zoning behavior.
- **Alternatives considered**:
  - Focus-based routing only: rejected because cursor hover intent is explicit and was requested.
  - Hybrid overflow routing (current behavior): rejected due to inconsistent interaction.

## Decision 2: Conversation startup load and lazy-load policy

- **Decision**: Load latest 10 conversation messages on startup; lazy-load older messages in fixed batches of 10 when scrolling upward at the top boundary.
- **Rationale**: Preserves continuity while bounding startup work and memory usage; matches current feature requirements and success criteria.
- **Alternatives considered**:
  - Load full conversation on startup: rejected for performance/memory risk on long histories.
  - Smaller startup batch (3 or 5): rejected because user asked to continue from recent context immediately.

## Decision 3: Input-history lazy-load policy (clarified)

- **Decision**: Do not preload input-history into memory. On first input-history scroll, load 3 latest entries; lazy-load older entries in batches of 3; cap loaded input-history entries to 12; clear in-memory input-history cache after send.
- **Rationale**: Input-history scroll is infrequently used, so conservative loading reduces overhead while preserving usability.
- **Alternatives considered**:
  - Preload input history at startup: rejected as unnecessary overhead.
  - Larger batch/cap values: rejected as higher memory cost with little user benefit.

## Decision 4: Preserve viewport anchor when prepending history

- **Decision**: When older messages are prepended, keep the previously visible first message in view (no jump).
- **Rationale**: Prevents disorientation during incremental back-scroll and supports predictable history browsing.
- **Alternatives considered**:
  - Snap-to-top after prepend: rejected due to disruptive UX.
  - Snap-to-bottom after prepend: rejected because it defeats backward browsing.

## Decision 5: Failure handling for history fetches

- **Decision**: Treat conversation/input history load failures as non-fatal; keep UI interactive and surface an inline error indicator.
- **Rationale**: Availability and UX consistency are prioritized for local TUI; failures should degrade gracefully.
- **Alternatives considered**:
  - Hard-fail startup on history read error: rejected as too disruptive.
  - Silent failure without visibility: rejected because it harms diagnosability.
