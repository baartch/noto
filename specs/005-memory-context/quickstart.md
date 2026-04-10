# Quickstart: Memory Context Indexing

**Goal**: Validate relevance-based memory context with persistent vector indexing and configurable token budget.

## Prerequisites

- A working build of `noto` with memory extraction enabled.
- A profile with enough memory notes (seed at least 100 short notes).

## Steps

1. **Verify indexing persists**
   - Start Noto, run a chat turn, then exit and restart.
   - Confirm the existing index is reused (no full rebuild).

2. **Verify relevance selection**
   - Trigger a chat turn and inspect the assembled prompt.
   - Confirm only notes within the configured token budget are injected.

3. **Verify fallback ranking**
   - Temporarily disable or remove the vector index.
   - Confirm notes are selected by importance then recency.

4. **Verify maintenance**
   - Add new notes and confirm incremental index updates.
   - Trigger compaction/rebuild and ensure it completes without blocking chat.

5. **Verify extractor fallback**
   - Clear the extractor model configuration.
   - Trigger a chat turn and confirm extraction uses the main model.
   - Confirm a footer warning is shown while fallback is active.

6. **Verify settings**
   - Open settings (Ctrl+J) and adjust the token budget.
   - Confirm subsequent chat turns use the updated budget.

## Expected Results

- Relevant notes are injected within the token budget.
- Vector index is reused across restarts.
- Index maintenance runs automatically without blocking chat.
- Fallback ranking is deterministic.
- Settings changes take effect for context retrieval.
- Extractor fallback uses the main model with a visible footer warning.
