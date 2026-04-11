# Quickstart: Settings Dialog Navigation

**Goal**: Validate the settings dialog workflow and editing behaviors.

## Prerequisites

- Build of `noto` with TUI enabled.
- An active profile with provider configuration available.

## Steps

1. **Open settings**
   - Press Ctrl+, and confirm the settings dialog opens.
   - Verify entries are sorted alphabetically.

2. **Navigate submenus**
   - Enter the provider configuration submenu.
   - Press Esc and confirm you return to the top-level list.

3. **Edit a value**
   - Select token budget and press Enter.
   - Update the value in the textarea and press Enter to save.
   - Reopen settings and verify the value persists.

4. **Validate numeric input**
   - Select token budget and press Enter.
   - Enter a non-numeric value and confirm an error appears and the editor stays open.

5. **Cancel edit**
   - Select system prompt and press Enter.
   - Press Esc to cancel and confirm the prompt is unchanged.

5. **Close settings**
   - From top-level, press Esc to close the dialog.

## Expected Results

- Ctrl+, opens the settings dialog in under 1 second (manual measurement acceptable).
- Entries and submenus are sorted alphabetically.
- Enter saves edits; Esc cancels edits.
- Esc navigates up from submenus and closes at the top level.
- Model/extractor model, provider configuration, token budget, and system prompt are reachable.
