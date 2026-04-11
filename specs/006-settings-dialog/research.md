# Research: Settings Dialog Navigation

## Decision: Use Bubbles list + textarea for settings dialog

**Rationale**: The Bubbles list is already used for pickers and supports navigation/filtering, while textarea provides consistent input editing for text and numeric values.

**Alternatives considered**:
- Custom list rendering (more effort, inconsistent with current UI)
- Single-line input only (limits editing for longer values like prompts)

## Decision: Esc navigates up, Esc closes at top level

**Rationale**: Matches established TUI patterns for hierarchical navigation and provides predictable exit behavior.

**Alternatives considered**:
- Dedicated back keybinding (adds cognitive load)
- Separate close key (duplicates Esc behavior)

## Decision: Enter saves edits, Esc cancels

**Rationale**: Consistent with existing modal edit flows and explicit save/abort actions.

**Alternatives considered**:
- Auto-save on blur (hard to reason about in TUI)
- Ctrl+S to save (adds extra keybinding)
