# Quickstart: Settings Dialog Navigation

## Build & Test Status

```
go test ./...  → all packages pass
make lint      → 0 issues
```

## How it works

1. Press **Ctrl+J** to open the settings dialog.
2. Use **↑/↓** to navigate entries (sorted alphabetically).
3. Press **Enter** on a value entry to open the inline textarea editor.
   - Edit the value. **Enter** saves; **Esc** cancels.
   - Numeric values (e.g. Memory Token Budget) are validated — non-numeric input shows an error.
   - The API Key is displayed obfuscated in the list but shown in full inside the editor.
4. Press **Enter** on a submenu entry (e.g. Provider) to navigate into it.
   - **Esc** goes back to the parent menu.
5. Press **Esc** at the root menu to close the dialog.
6. Press **Enter** on **Model** or **Model Extractor** to open the model picker.

## Persistence

- **System Prompt** — stored in profile SQLite (`system_prompts` table). Saving invalidates the context cache.
- **Memory Token Budget** — stored in `profile.json`.
- **Provider Endpoint / Key** — stored in `provider_config` table. Key is encrypted with `security.MachinePassphrase`.

## Performance

Settings dialog opens in <1s (manual verification). All reads happen synchronously on open; no background work required.

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
