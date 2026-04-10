# Contract: Context Retrieval

## Inputs

- **profile_id**: string
- **system_prompt**: string
- **note_budget_tokens**: int (default 1500, configurable)

## Behavior

1. Retrieve relevant notes using vector index (when available).
2. Apply token budget to selected notes.
3. If index unavailable, fall back to importance then recency ordering.
4. Assemble prompt with system prompt + session summary + memory block.
5. Cache assembled context for reuse across restarts.
6. If extractor model is missing, use the main model and surface a footer warning.

## Output

- **assembled_prompt**: string
- **memory_block**: string
- **cache_hit**: bool
