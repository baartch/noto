# Research: Cross-Conversation History Scrolling

## Decision 1: History source spans all conversations in active profile
- **Decision**: Use profile-scoped message history across active + archived conversations when scrolling backward.
- **Rationale**: Matches clarified requirement to scroll through all history regardless of conversation.
- **Alternatives considered**:
  - Current-conversation only: rejected; does not satisfy clarification.
  - Current + previous conversation only: rejected; partial history.

## Decision 2: Conversation boundary rendering model
- **Decision**: Insert synthetic boundary rows between message groups when conversation ID changes, showing conversation start date.
- **Rationale**: Provides clear context transition with minimal visual noise (thin divider + date).
- **Alternatives considered**:
  - No separators: rejected; poor context when crossing conversation boundaries.
  - Full header cards: rejected; too visually heavy for terminal chat flow.

## Decision 3: Pagination strategy across conversations
- **Decision**: Keep a unified chronologically sorted message stream for the active profile and lazy-load in fixed batches of 10 older items.
- **Rationale**: Preserves existing lazy-load expectations while extending to cross-conversation history.
- **Alternatives considered**:
  - Per-conversation paging with nested cursors: rejected as unnecessary complexity.
  - Full preload of profile history: rejected due to startup/memory overhead.

## Decision 4: Failure behavior during back-scroll fetches
- **Decision**: Treat paging failures as non-fatal; keep currently loaded messages and show inline non-blocking error.
- **Rationale**: Aligns with existing resilience requirements and avoids interrupting user workflow.
- **Alternatives considered**:
  - Hard-fail interaction on paging error: rejected as poor UX.

## Decision 5: Boundary date format
- **Decision**: Render boundary date in local readable format `YYYY-MM-DD HH:MM` (conversation start time).
- **Rationale**: Compact and unambiguous in terminal width constraints.
- **Alternatives considered**:
  - Relative time only: rejected; becomes ambiguous for older conversations.
