# Quickstart: Portable Profiles

## Create a Profile

1. Create a profile using the CLI or TUI.
2. Verify the profile directory contains the profile metadata file (`profile.json`).

## Move a Profile

1. Copy the profile directory to another instance's profile root.
2. List profiles to confirm it appears and is selectable.
3. If the metadata file is missing, expect the profile to be flagged as invalid.

## Select a Profile

1. Select a profile by name or slug.
2. Restart the app and confirm the active profile remains selected via the local `active_profile` file.

## Validate No Global DB Usage

1. Inspect the global database (if present).
2. Confirm no profile metadata entries are stored (no `profiles` table).

## Validation Notes

- 2026-03-29: Pending manual validation.
