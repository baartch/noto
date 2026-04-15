# Research: note-extraction-strategy

## Decision 1: Embeddings model persistence
**Decision**: Move embeddings model persistence to provider_config in the profile SQLite DB.
**Rationale**: Aligns with provider-scoped settings and enables immediate Settings menu refresh from DB.
**Alternatives considered**: Keep in profile.json (current approach) — rejected because it diverges from provider_config usage and makes settings refresh indirect.

## Decision 2: Provider_config schema cleanup
**Decision**: Remove columns that are not referenced in code paths (after verification).
**Rationale**: Simplifies configuration storage and reduces unused schema surface.
**Alternatives considered**: Leave columns in place — rejected due to explicit requirement to drop unused columns.

## Decision 3: Settings list refresh behavior
**Decision**: Refresh Settings list immediately after embeddings model selection.
**Rationale**: UX requirement for instant feedback; aligns with existing model/extractor flows.
**Alternatives considered**: Deferred refresh on menu reopen — rejected for stale display.
