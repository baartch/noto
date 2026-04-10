# Data Model: Slash Command Navigation

This feature is primarily interactive behavior. The "data model" here describes the UI state needed to deliver correct suggestion list scrolling and selection.

## Entities

### Slash Suggestion

Represents an available command the user can select.

- **CommandPath**: Canonical command path (without leading `/`), e.g. `"profile list"`
- **Hint**: Short description shown in the UI

### Suggestion List State

Ephemeral state maintained while the user is typing a slash command.

- **Suggestions**: Ordered list of `Slash Suggestion` items (filtered for the current prefix)
- **CursorIndex**: Integer index into the full `Suggestions` list indicating current selection
- **NavigationActive**: Boolean indicating whether the user is actively navigating suggestions (vs. typing)

### Windowing (derived)

Values computed during rendering based on terminal height.

- **VisibleStartIndex**: Start index into `Suggestions` for currently visible window
- **VisibleEndIndex**: End index (exclusive) for visible window
- **MaxVisibleRows**: Maximum number of suggestion rows that can be drawn given available terminal height

## Invariants

- `CursorIndex` is always clamped to `[-1 .. len(Suggestions)-1]`.
- If `CursorIndex >= 0`, the visible window must be chosen so that `CursorIndex` is within `[VisibleStartIndex .. VisibleEndIndex)`.
