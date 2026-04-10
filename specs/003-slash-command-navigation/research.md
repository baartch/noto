# Research: Slash Command Navigation

**Date**: 2026-04-10

## Current State (repo findings)

### Existing slash suggestion implementation

- The TUI already has a dedicated suggestion state in `internal/tui/model.go`:
  - `suggestions []suggest.Suggestion`
  - `suggCursor int`
  - `suggActive bool`
- Suggestions are recomputed during typing by calling the dispatcher with a trailing space:
  - `dispatcher.Dispatch(val+" ", execCtx)`
  - This leverages the existing parser behavior where a trailing space marks the input as “partial”, enabling suggestion mode.
- Navigation keys currently mutate the cursor and write the selected suggestion back into the input:
  - On `Up/Down` (either entering suggestion navigation or while `suggActive`), `input.SetValue("/"+suggestions[cursor].CommandPath)`.

### Identified UX gap

- Suggestion rendering (`renderSuggestions`) prints the full suggestion list with no height constraint and no windowing.
- When the suggestion list is longer than the available terminal space, the user cannot effectively browse beyond what is visible on screen.
- The picker overlay (`internal/tui/picker.go`) already implements list windowing around a cursor (compute `start/end` based on `maxHeight`), which is the desired behavior for slash suggestions.

## Decision

### Decision: Add windowed rendering for slash suggestions

- **Chosen**: Implement suggestion list windowing (scrolling) similar to the picker overlay:
  - Only render the visible subset of suggestions based on available terminal height.
  - Ensure the current selection remains within the visible window by adjusting `start/end` as the cursor changes.

## Rationale

- Matches user expectation (Up/Down traverses full list and the UI scrolls).
- Consistent with existing UI patterns (picker overlay already behaves this way).
- Small, localized change: no need to change command parsing/dispatch, just rendering/window logic.

## Alternatives considered

1. **Keep current rendering and rely on terminal scrollback**
   - Rejected: impractical inside an interactive TUI; user can’t see selection context.

2. **Introduce Page Up/Page Down for suggestion navigation**
   - Rejected for v1: adds extra UX surface; Up/Down scrolling window is sufficient and simpler.

3. **Open a full picker overlay for suggestions**
   - Rejected for v1: more invasive; suggestions already integrate with the input and should remain lightweight.

## Notes / Constraints

- Ensure this does not regress existing Up/Down behavior:
  - When suggestions are visible: Up/Down navigates suggestions.
  - When suggestions are not visible: Up/Down navigates input history; otherwise scrolls viewport.
- Tab behavior is specified in the feature spec; current code should be checked/updated during implementation to match (Tab should autofill selected suggestion, not necessarily require Up/Down first).

## Performance Check

- **Target**: p95 suggestion refresh < 50 ms per keystroke.
- **Status**: Pending measurement (manual verification needed).
- **Notes**: TODO
