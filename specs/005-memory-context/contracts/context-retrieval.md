# Contract: Context Retrieval and Extraction

## Inputs

- **profile_id**: string
- **system_prompt**: string (loaded from `<profile>/prompts/system.md`)
- **extractor_prompt**: string (loaded from `<profile>/prompts/extractor.md`)
- **note_budget_tokens**: int (default 1500, configurable)

## Behavior

1. Load profile-local prompt files from `<profile>/prompts/`.
2. Retrieve relevant notes using vector index (when available).
3. Apply token budget to selected notes.
4. If index unavailable, fall back to importance then recency ordering.
5. Assemble prompt with system prompt + session summary + memory block.
6. Cache assembled context for reuse across restarts.
7. If extractor model is missing, use the main model and surface a footer warning.
8. Extractor response MUST be valid JSON and pass note-level validation before persistence.

## Extractor Response Contract (JSON)

```json
{
  "has_new_info": true,
  "confidence": 0.86,
  "notes": [
    {
      "action": "add",
      "category": "fact",
      "content": "User prefers concise responses.",
      "importance": 7
    },
    {
      "action": "update",
      "target_id": "note_123",
      "category": "progress",
      "content": "Project moved from draft to implementation.",
      "importance": 8
    }
  ]
}
```

Validation rules:
- Top-level object MUST include `has_new_info` (boolean), `confidence` (float 0.0..1.0), and `notes` array.
- Each note MUST include `action` with value `add|update`.
- `action=update` MUST include `target_id`.
- Each note MUST include `category` in `fact|progress|blocker|action_item|other`.
- Invalid payloads MUST be rejected and logged.

No-new-info example:

```json
{
  "has_new_info": false,
  "confidence": 0.12,
  "notes": []
}
```

## Output

- **assembled_prompt**: string
- **memory_block**: string
- **cache_hit**: bool
- **accepted_notes_count**: int
- **rejected_payload**: bool
