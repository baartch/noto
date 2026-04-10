# Quickstart: Slash Command Navigation

**Goal**: Manually validate slash command suggestion discovery, filtering, full-list navigation (scrolling), tab completion, and execution.

## Prerequisites

- A working build of `noto`.
- A command registry with enough commands to exceed the visible suggestion area (or temporarily reduce terminal height).

## Steps

1. **Show suggestions**
   - Start `noto`.
   - In the input, type `/`.
   - Verify a list of available commands appears.

2. **Filter suggestions**
   - Continue typing characters (e.g. `/m` or `/pro`).
   - Verify the list filters to matching commands.

3. **Scroll through the full list**
   - Ensure the suggestion list is longer than can fit on screen (shrink terminal height if needed).
   - Press Down repeatedly.
   - Verify:
     - selection continues past the initially visible portion,
     - the list window scrolls,
     - the selected item is always visible.
   - Press Up repeatedly.
   - Verify scrolling works in the other direction too.

4. **Tab completion**
   - With a suggestion selected, press Tab.
   - Verify the input is auto-filled to the selected command path.

5. **Execute**
   - With a suggestion selected, press Enter.
   - Verify the command executes (appropriate output appears) and the input clears.

6. **Exit suggestion mode**
   - Clear the input (remove the leading `/`).
   - Verify the suggestion list disappears.

## Validation Notes

- **Status**: Pending manual run
- **Notes**: TODO

## Expected Results

- Users can reach any command in the list using Up/Down, regardless of list length.
- Suggestion updates feel instantaneous while typing.
- Tab and Enter behave consistently with the contract.
