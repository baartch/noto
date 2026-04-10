# Data Model: Memory Context Indexing

## Entities

### Memory Note

- **id**: string
- **profile_id**: string
- **conversation_id**: string (nullable)
- **category**: fact | progress | blocker | action_item | other
- **content**: string
- **importance**: int (1–10)
- **created_at**: timestamp
- **updated_at**: timestamp

### Vector Index Entry

- **id**: string
- **profile_id**: string
- **source_type**: memory_note | session_summary | message
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
- **status**: ready | stale | rebuilding | failed

### Context Cache Entry

- **id**: string
- **profile_id**: string
- **cache_key**: string
- **payload**: string
- **created_at**: timestamp
- **expires_at**: timestamp (nullable)

## Relationships

- Memory Note 1→N Vector Index Entry (source_id)
- Vector Index Manifest 1→1 per profile
- Context Cache Entry 1→1 per cache key
