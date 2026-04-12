# Data Model: Profile Management in Settings

## Settings Entry (Profiles)

Add a Profiles submenu with action entries:

- `profile_select` (action)
- `profile_create` (action)
- `profile_rename` (action)
- `profile_delete` (action)

## Validation

- Reuse existing profile validation in `internal/profile`.
- Delete requires explicit confirmation (`profile.ConfirmDeletion`).

## State Transitions

- **Select** → active profile changes, settings reload.
- **Create** → new profile directory + metadata + sqlite.
- **Rename** → slug and paths updated.
- **Delete** → profile directory removed after confirmation.
