# Contract: Profile Discovery & Selection

## Overview

Profiles are discovered by scanning profile directories and reading profile-local metadata files. The global database is not used for profile metadata.

## Profile Listing

- **Input**: None.
- **Output**: List of available profiles with name, slug, and active indicator.
- **Behavior**:
  - Profiles are discovered from profile directories.
  - Profiles without valid metadata are excluded with a warning.
  - Active profile is determined from local selection storage.

## Profile Selection

- **Input**: Profile name or slug.
- **Output**: Confirmation of active profile.
- **Behavior**:
  - Selection updates local active profile storage.
  - Selection does not write to the global database.
  - Errors if the profile directory or metadata is missing.

## Profile Creation

- **Input**: Profile name.
- **Output**: Profile directory with metadata file.
- **Behavior**:
  - Metadata file is written into the profile directory.
  - Global database remains unchanged for profile metadata.

## Profile Deletion

- **Input**: Profile name or slug and confirmation.
- **Output**: Profile directory removed.
- **Behavior**:
  - Deletion removes the profile directory.
  - Active profile selection is updated if the deleted profile was active.
