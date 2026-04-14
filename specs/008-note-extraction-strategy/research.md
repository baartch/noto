# Research: note-extraction-strategy

## Decision: Use existing profile SQLite + vector index for note storage and deduplication

**Rationale**: The project already stores profile memory in a local SQLite database with a vector index file; leveraging these keeps data local and enables fast similarity checks for deduplication and retrieval.

**Alternatives considered**:
- Store notes in flat files with in-memory search (rejected: slower deduplication, higher memory usage).
- Rely solely on keyword search without vectors (rejected: lower relevance for semantic matching).
