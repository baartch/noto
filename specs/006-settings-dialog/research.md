# Research

## Decisions

- **Decision**: Reuse existing profile service and store repositories for CRUD, but drop slash command pathways for profile management in the TUI.
  - **Rationale**: Keeps persistence rules centralized while aligning UX with new settings-only management flow.
  - **Alternatives considered**: Keep slash commands and dispatch from TUI. Rejected because FR-008e forbids slash-command reliance.

- **Decision**: Implement Profiles management as a dedicated list view inside Settings with keyboard shortcuts (Enter/Ctrl+N/Ctrl+R/Ctrl+D).
  - **Rationale**: Matches updated UX requirement for single list with in-place actions.
  - **Alternatives considered**: Separate submenu with action entries. Rejected per updated spec.
