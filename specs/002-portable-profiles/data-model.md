# Data Model: Portable Profiles

## Profile Metadata File

**Purpose**: Store profile metadata inside the profile directory for portability.

**Fields**:
- **Profile ID**: Unique identifier for the profile.
- **Profile Name**: Human-friendly display name.
- **Profile Slug**: Stable filesystem-safe identifier.
- **Created At**: Creation timestamp.
- **Updated At**: Last update timestamp.
- **System Prompt Path**: Path to the profile prompt file within the profile directory.

**Validation Rules**:
- Profile ID must be non-empty and unique within the profile directory.
- Profile slug must be filesystem-safe and stable once created.
- Profile name must be non-empty.

## Active Profile Selection

**Purpose**: Track which profile is active without storing data in the global DB.

**Fields**:
- **Active Profile Slug**: Identifier of the active profile.
- **Last Selected At**: Timestamp of the last selection.

**Validation Rules**:
- Active profile slug must reference an existing profile directory.

## Global Database

**Purpose**: Must not store any profile metadata after this change.

**Constraints**:
- No profile name, slug, or default/active flags stored in the global DB.
