# Quickstart: note-extraction-strategy

## Goal

Validate note extraction, deduplication, footer notifications, and embeddings model selection end-to-end in a local profile.

## Steps

1. Start the app with a test profile.
2. In Settings, select a dedicated embeddings model.
3. Run a chat that includes a clear note-worthy fact.
4. Verify the note is stored once and the footer notification appears for ~3 seconds.
5. Repeat the fact with different wording and confirm no duplicate is stored.
6. Ask a related follow-up and confirm relevant notes are retrieved.

## Validation Notes (2026-04-15)

- Verified dedupe prevented storing repeated facts.
- Footer displayed “note(s) saved” for ~3 seconds after capture.
- Action items were stored as separate notes when present.
