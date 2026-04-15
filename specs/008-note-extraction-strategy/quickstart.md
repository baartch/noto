# Quickstart: note-extraction-strategy

## Goal
Validate provider_config persistence for embeddings model and Settings list refresh behavior.

## Steps

1. Configure provider:
   ```bash
   noto provider set --key sk-... --model gpt-4o-mini
   ```
2. Open the TUI settings menu.
3. Change "Model Embeddings" to a different model.
4. Confirm the Settings list immediately shows the new embeddings model.
5. Restart the app and verify the embeddings model remains selected.
6. Validate UX consistency: settings list reflects the updated model without reopening the menu.
