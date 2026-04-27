# Data Model: Memory Context Indexing

## Entities

### Memory Note

- **id**: string
- **profile_id**: string
- **conversation_id**: string (nullable)
- **category**: `fact | progress | blocker | action_item | other`
- **content**: string
- **importance**: int (1–10)
- **created_at**: timestamp
- **updated_at**: timestamp

Validation:
- `category` MUST be one of the constrained values.
- `content` MUST be non-empty.

### Prompt File

- **profile_id**: string
- **name**: `system | extractor`
- **path**: string (`<profile>/prompts/<name>.md`)
- **content**: string (markdown)
- **updated_at**: timestamp

Validation:
- Prompt files MUST exist or be lazily created with defaults.
- Prompt storage MUST NOT use SQLite for system/extractor prompt content.

### Extraction Payload

- **notes**: array of `ExtractedNote`

Validation:
- Payload MUST be valid JSON object with top-level `notes` array.

### ExtractedNote

- **action**: `add | update`
- **target_id**: string (required iff `action=update`)
- **category**: `fact | progress | blocker | action_item | other`
- **content**: string
- **importance**: int (optional if model omits; system may default)

Validation:
- Each note MUST include its own `action`.
- `target_id` MUST be present for `update`, absent/ignored for `add`.
- `category` MUST be in allowed set.
- Invalid notes invalidate the payload.

### Vector Index Entry

- **id**: string
- **profile_id**: string
- **source_type**: `memory_note | session_summary | message`
- **source_id**: string
- **chunk_hash**: string
- **embedding_model**: string
- **embedding_dim**: int
- **vector_ref**: string
- **updated_at**: timestamp

### Vector Index Manifest

- **profile_id**: string
- **index_path**: string
- **index_format_version**: string
- **embedding_model**: string
- **embedding_dim**: int
- **last_rebuild_at**: timestamp
- **last_sync_at**: timestamp
- **source_state_version**: string
- **status**: `ready | stale | rebuilding | failed`

### Context Cache Entry

- **id**: string
- **profile_id**: string
- **cache_key**: string
- **payload**: string
- **created_at**: timestamp
- **expires_at**: timestamp (nullable)

## Relationships

- Memory Note 1→N Vector Index Entry (source_id)
- Prompt File N→1 Profile
- Vector Index Manifest 1→1 per profile
- Context Cache Entry 1→1 per cache key

## State/Flow Notes

- Extraction: input message(s) → extractor JSON payload → validated extracted notes → add/update note writes → incremental index updates.
- Prompt edits: settings/UI edit → write `<profile>/prompts/*.md` → invalidate context cache when system prompt changes.
