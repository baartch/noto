# Data Model: note-extraction-strategy

## Note

- **id**: Unique identifier
- **content**: Canonical note text
- **value_score**: Importance score used to decide storage
- **source_context**: Conversation excerpt or message reference
- **created_at**: Timestamp
- **updated_at**: Timestamp

## NoteCandidate

- **content**: Extracted candidate text
- **value_score**: Computed importance score
- **duplicate_of**: Reference to existing Note id if duplicate detected
- **evidence**: Supporting context snippets

## NoteRetrievalResult

- **note_id**: Referenced Note id
- **relevance_score**: Similarity/relevance score
- **rank**: Position in ranked results
