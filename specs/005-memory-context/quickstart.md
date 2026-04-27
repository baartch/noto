# Quickstart: Memory Context Indexing

**Goal**: Validate relevance-based memory context with persistent vector indexing, profile-local prompt files, and strict extractor JSON contracts.

## Prerequisites

- A working build of `noto` with memory extraction enabled.
- A profile with at least 100 notes for relevance checks.

## Steps

1. **Verify prompt file storage**
   - Open or create a profile.
   - Confirm prompt files exist at:
     - `<profile>/prompts/system.md`
     - `<profile>/prompts/extractor.md`
   - Confirm prompt content is Markdown and not persisted in SQLite.

2. **Verify relevance selection**
   - Trigger a chat turn and inspect assembled context.
   - Confirm only notes within configured token budget are injected.

3. **Verify strict extractor JSON contract**
   - Trigger extraction and capture raw model output.
   - Confirm output is valid JSON with top-level `notes` array.
   - Confirm each note has per-note `action` (`add|update`).
   - Confirm `update` notes include `target_id`.
   - Confirm each note category is one of `fact|progress|blocker|action_item|other`.

4. **Verify malformed payload handling**
   - Provide invalid JSON or missing required fields.
   - Confirm extraction payload is rejected and warning is logged.

5. **Verify index fallback and maintenance**
   - Disable/remove vector index and run chat turn.
   - Confirm fallback to importance then recency.
   - Add/update notes and confirm incremental index updates.

6. **Verify extractor model fallback**
   - Clear extractor model configuration.
   - Confirm main model is used and footer warning appears.

7. **Verify missing prompt-file warning UX**
   - Delete `<profile>/prompts/system.md` and restart/open chat for that profile.
   - Confirm the file is auto-created with defaults.
   - Confirm the footer shows: `Prompt files missing — bootstrapped defaults.`
   - Repeat for `<profile>/prompts/extractor.md`.

## Expected Results

- Prompt files are profile-local Markdown files.
- Missing prompt files are auto-bootstrapped with defaults and show a visible footer warning.
- Extraction returns contract-compliant JSON with note-level actions.
- `update` actions always include `target_id`.
- Category taxonomy remains constrained and consistent.
- Invalid payloads are rejected with observability signals.
- Updated notes participate in incremental vector sync.
- Relevance and persistence behavior remain unchanged from baseline.

## Validation Commands

- `go test ./...` ✅
- `make lint` ✅
- `make fmt` ✅

## UX Validation Checklist (Prompt Bootstrap Warning)

- [ ] Footer warning appears when `system.md` is missing and auto-created.
- [ ] Footer warning appears when `extractor.md` is missing and auto-created.
- [ ] Warning text matches expected copy and does not overlap token/cache indicators.
- [ ] Warning is non-fatal (chat flow continues).

## Wiring Notes

- Integration fixture constants for extractor payloads live in `tests/integration/memory/extractor_payload_fixtures.go`.
- Prompt file test helpers for profile-local markdown prompts live in `tests/integration/testutil/prompt_files.go`.
- Use these helpers when validating `<profile>/prompts/system.md` and `<profile>/prompts/extractor.md` persistence and bootstrap behavior.
