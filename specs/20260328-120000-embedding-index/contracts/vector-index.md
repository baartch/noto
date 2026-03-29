# Contract: Vector Index

## File Location
- `~/.noto/profiles/<profile>/memory.vec`

## Format (v1)
- Header:
  - magic: `NOTOVEC1`
  - profile_id (string)
  - embedding_model (string)
  - embedding_dim (uint32)
  - entry_count (uint32)
  - offsets for vectors + graph
- Payload:
  - vector table: contiguous float32 embeddings
  - HNSW graph adjacency list

## Operations
- **Upsert**: insert/update by `(source_type, source_id, chunk_hash)`; update manifest entry.
- **Delete**: remove index entry and manifest entry.
- **Search**: return top‑k `Entry` list by cosine similarity.
- **Rebuild**: discard file and recreate from SQLite notes.

## Failure Modes
- Missing file → `ErrIndexNotFound` (warn + fallback)
- Corrupt file → `ErrIndexCorrupted` (warn + fallback, explicit rebuild)

## Performance Budget
- Top‑k search < 40ms p95 on typical corpora (synthetic benchmark).
