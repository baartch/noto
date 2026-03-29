# Feature Specification: Portable Profiles

**Feature Branch**: `002-portable-profiles`  
**Created**: 2026-03-29  
**Status**: Draft  
**Input**: User description: "I want profiles being portable between instances. So no information about a profile should be stored in the global database."

## Clarifications

### Session 2026-03-29

- Q: Where should active profile selection be persisted? → A: Store active profile in a local app config file per instance.
- Q: Where should profile metadata be stored within a profile directory? → A: Store a metadata file directly under each profile directory.
- Q: How should missing profile metadata be handled during discovery? → A: List the profile as invalid with a warning and require repair.
- Q: How should duplicate profile names be handled? → A: Allow duplicates and disambiguate by slug in listings and selection.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Profile Data Stays Local (Priority: P1)

As a user, I want all profile-specific data to live within the profile itself so I can move a profile between instances without losing or conflicting metadata.

**Why this priority**: Portability depends on keeping all profile metadata within the profile; this is the core requirement.

**Independent Test**: Create a profile, move only the profile directory to a fresh instance, and confirm the profile loads with the same name, settings, and data without referencing global records.

**Acceptance Scenarios**:

1. **Given** a profile with metadata and data, **When** the profile directory is moved to another instance, **Then** the profile is recognized and usable without needing a global record.
2. **Given** a profile exists, **When** profile data is queried, **Then** no profile metadata is read from the global database.

---

### User Story 2 - Global DB Excludes Profile Metadata (Priority: P2)

As a user, I want the global database to avoid storing profile-specific details so global state does not block portability.

**Why this priority**: Global metadata could cause conflicts or mismatches when a profile is moved; removing it keeps instances independent.

**Independent Test**: Inspect the global database after profile operations and verify no profile metadata is stored or required for profile access.

**Acceptance Scenarios**:

1. **Given** profiles are created or updated, **When** the global database is inspected, **Then** it contains no profile-specific records.
2. **Given** a profile is deleted, **When** the global database is inspected, **Then** no profile metadata remains or is required.

---

### User Story 3 - Multiple Profiles Remain Usable (Priority: P3)

As a user, I want to continue managing multiple profiles locally without relying on a global registry.

**Why this priority**: Users still need multi-profile support even when global metadata is removed.

**Independent Test**: Create multiple profiles and verify listing, selection, and deletion operate without global profile storage.

**Acceptance Scenarios**:

1. **Given** multiple profile directories exist, **When** profiles are listed, **Then** all profiles are discovered without global profile metadata.
2. **Given** a profile is selected as active, **When** the application restarts, **Then** the selection is restored without global profile metadata.

---

### Edge Cases

- If a profile directory lacks metadata, it is listed as invalid with a warning and requires repair before use.
- If two profiles share the same name, list and select them by slug to disambiguate.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST store all profile metadata within a metadata file directly under the profile directory.
- **FR-002**: The system MUST NOT store profile metadata in the global database.
- **FR-003**: The system MUST discover profiles by scanning profile directories rather than reading a global registry.
- **FR-004**: Users MUST be able to list available profiles without relying on global profile metadata.
- **FR-005**: Users MUST be able to select an active profile without global profile metadata.
- **FR-007**: The system MUST persist the active profile selection in a local app config file per instance.
- **FR-006**: The system MUST preserve profile portability by allowing a profile directory to be moved between instances without additional steps.

### Non-Functional Requirements *(mandatory)*

- **NFR-001 Code Quality**: Changes MUST pass formatting, linting, and static analysis rules
  defined by the project.
- **NFR-002 Testing Standards**: Changes MUST include automated tests for new/changed behavior,
  including negative/error paths where applicable.
- **NFR-003 UX Consistency**: User-facing changes MUST follow established UX patterns
  (terminology, interaction flows, visual behavior) or document approved deviations.
- **NFR-004 Performance**: Profile discovery MUST complete within 200 ms for up to 100 profiles.

### Key Entities *(include if feature involves data)*

- **Profile**: A user-specific workspace containing metadata, settings, memory data, and any configuration files.
- **Profile Directory**: The filesystem location that encapsulates all profile-specific data.
- **Global Database**: Shared storage that MUST exclude profile metadata after this change.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A profile directory moved to a fresh instance loads successfully without manual migration steps.
- **SC-002**: Profile listing completes within 200 ms for up to 100 profiles.
- **SC-003**: 100% of profile metadata is stored within the profile directory (0% in global DB).
- **SC-004**: Users can select and use profiles after restart without global profile metadata.
- **SC-005**: 0 lint/format violations in CI for the feature scope.
- **SC-006**: All new/changed behavior covered by automated tests.

## Assumptions

- Profiles are stored in a predictable directory path per instance.
- Profile directories include or can be extended with metadata files needed for discovery.
- Existing profile operations can be updated to use directory scanning without breaking CLI/TUI flows.
- Users do not expect cross-instance sharing of global settings unrelated to profiles.
