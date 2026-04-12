# Data Model

## Entities

### Profiles List Entry
- **Description**: A selectable entry representing a profile within the settings Profiles list.
- **Fields**:
  - `ID` (string): Profile ID (UUID or slug key from storage).
  - `Name` (string): Display name for the profile.
  - `Slug` (string): Unique profile slug used for persistence and switching.
  - `Active` (bool): Whether this profile is currently active.

### Profiles Editor State
- **Description**: Tracks the current Profiles list interaction state in settings.
- **Fields**:
  - `Mode` (enum: `view`, `create`, `rename`): Current edit mode.
  - `SelectedProfileSlug` (string): Slug for currently selected profile.
  - `InputValue` (string): Current text entered in the settings textarea.

## Validation Rules
- Profile names are required and must satisfy existing profile service validation.
- Delete cannot remove the last remaining profile (existing profile service guard).

## Relationships
- Profiles List Entry corresponds to existing profile metadata stored in profile.json and SQLite files in the profile directory.
