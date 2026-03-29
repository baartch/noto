# Research: Portable Profiles

## Decision 1: Profile discovery source

**Decision**: Discover profiles by scanning the profile directory root and reading per-profile metadata files.
**Rationale**: Scanning keeps profiles self-contained and avoids reliance on a global registry.
**Alternatives considered**: Global DB registry (rejected due to portability requirement).

## Decision 2: Profile metadata storage

**Decision**: Store display name, slug, creation timestamps, and default prompt path in a profile-local metadata file.
**Rationale**: Keeping metadata in the profile directory ensures portability and eliminates cross-instance coupling.
**Alternatives considered**: Global DB metadata (rejected), derived-only metadata from filenames (insufficient for name changes).

## Decision 3: Active profile selection

**Decision**: Persist active profile selection in a local app config file, not in the global DB.
**Rationale**: Active selection is instance-specific and should not require global DB storage.
**Alternatives considered**: Global DB default flag (rejected), environment-only selection (would not persist across restarts).
