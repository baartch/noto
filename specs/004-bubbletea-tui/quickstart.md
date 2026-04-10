# Quickstart: Bubble Tea TUI Standard

**Goal**: Validate that all TUI flows use Bubble Tea patterns, prefer Bubbles components, and apply Lip Gloss styling definitions.

## Prerequisites

- A working build of `noto`.
- Knowledge of how to reach each TUI flow (startup, slash commands, pickers).

## Steps

1. **Inventory TUI flows**
   - Start the app and list all TUI interaction flows (startup selection, profile picker, model picker, backup restore, slash suggestions, etc.).

2. **Verify Bubble Tea usage**
   - For each flow, confirm it is backed by a Bubble Tea model and update loop.

3. **Verify Bubbles reuse**
   - For each flow, confirm that Bubbles components are used where applicable (text inputs, lists, viewports).

4. **Verify Lip Gloss styling**
   - For each flow, confirm styling is applied via reusable Lip Gloss style definitions.

5. **Check custom components**
   - If any custom UI remains, confirm rationale is documented.

6. **Verify help UX**
   - Toggle help and confirm keybinding help shows in the footer.
   - Expand help and confirm it renders above the input textarea without shifting the footer/input.
   - Confirm `Ctrl+D` exits and `Ctrl+L` opens the model picker.

## Expected Results

- All TUI flows are implemented as Bubble Tea models.
- Bubbles components are used whenever suitable.
- Lip Gloss styles are reused consistently across the UI.
- Keybinding help appears in the footer and expanded help renders above the input without layout shifts.
- `Ctrl+D` exits and `Ctrl+L` opens the model picker.
- Any remaining custom UI is explicitly justified.
