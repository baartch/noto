# Contract: Slash Suggestion Interaction

**Feature**: [Slash Command Navigation](../spec.md)

This contract defines the expected user-visible interaction behavior for slash command suggestions.

## Triggering and Visibility

1. When the input begins with `/`, the UI shows a suggestion list of matching commands.
2. As the user types additional characters after `/`, the suggestion list filters to matching commands.
3. When the input no longer begins with `/`, the suggestion list is hidden.

## Navigation

1. Up/Down moves selection through the **entire** suggestion list.
2. If the list is longer than the available visible area, the suggestion list window scrolls so the selected item remains visible.
3. Navigation does not move focus away from the input.

## Completion and Execution

1. **Tab** inserts (auto-fills) the currently selected suggestion into the input.
2. **Enter** executes the currently selected suggestion.
3. If there are no suggestions (or no selection), Tab does not change the input.
4. If there are no suggestions (or no selection), Enter behaves like normal message send.

## Empty / No Match State

- If there are no matching commands for the current prefix, the UI clearly indicates “no matches”.

## Determinism

- Suggestion ordering is deterministic for a given command registry and prefix.
