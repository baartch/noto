# Data Model: note-extraction-strategy

## provider_config (profile DB)

| Field | Type | Notes |
| --- | --- | --- |
| id | TEXT | Primary key |
| profile_id | TEXT | Profile owner |
| provider_type | TEXT | Provider identifier (e.g., openai_compatible) |
| endpoint | TEXT | Provider endpoint URL |
| model | TEXT | Default/fallback model |
| active_model | TEXT | Selected chat model |
| extractor_model | TEXT | Model for note extraction |
| embeddings_model | TEXT | Selected embeddings model |
| credential_ref | TEXT | Encrypted API key |
| is_active | INTEGER | 0/1 |
| created_at | DATETIME | Created timestamp |
| updated_at | DATETIME | Updated timestamp |

## Derived constraints

- Only one active provider_config per profile (is_active = 1).
- embeddings_model must be explicitly set before vector indexing operations.

## Schema cleanup

- Remove provider_config columns that are no longer referenced in application code.
