# Feature Specification: Settings Dialog Navigation

**Feature Branch**: `006-settings-dialog`
**Created**: 2026-04-11
**Status**: Draft
**Input**: User description: "I want ctrl+j opening a Settings dialog (Bubbles list). It shows key/value pairs or submenues ordered alphabetically. value pairs can be edited by selecting and pressing enter. text or numbers editing should open a Bubbles Textarea. Being in a submenu esc should go a level up, otherwise close the settings dialog. The settings should cover all possible App settings. Model/Extractor model selection, provider configuration (Submenu), token budget, System prompt edit, and profile management."

## Clarifications

### Session 2026-04-11

- Q: How are edits in the settings textarea saved vs canceled? → A: enter saves and esc cancels.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Open Settings (Priority: P1)

As a user, I want a dedicated settings dialog so I can find and adjust app settings without leaving the TUI flow.

**Why this priority**: Settings access is foundational for configuring the app across profiles.

**Independent Test**: Press ctrl+j, and verify the settings dialog opens with entries ordered alphabetically. Values are truncated with `...` when long or multi-line.

**Acceptance Scenarios**:

1. **Given** the TUI is active, **When** the user presses ctrl+j, **Then** the settings dialog opens.
2. **Given** the settings dialog is open, **When** entries are rendered, **Then** key/value pairs and submenus are sorted alphabetically.
3. **Given** the settings dialog is open at the top level, **When** the user presses esc, **Then** the dialog closes.

---

### User Story 2 - Edit Settings Values (Priority: P1)

As a user, I want to select a setting and edit its value so I can customize the app to my needs.

**Why this priority**: Editing values is the primary purpose of the settings dialog.

**Independent Test**: Select a value entry, press enter, edit it in a textarea, and confirm the new value is saved.

**Acceptance Scenarios**:

1. **Given** a settings entry represents a value, **When** the user selects it and presses enter, **Then** a textarea editor opens to update the value.
2. **Given** the editor is open for a text or numeric value, **When** the user presses enter, **Then** the updated value is stored and visible in the list.
2a. **Given** the user saves a new system prompt, **When** the value is persisted, **Then** the conversation context cache is invalidated so the new prompt takes effect immediately.
2b. **Given** a multi-line or long value is shown in the list, **Then** it is flattened and truncated with `...` to avoid wrapping.
3. **Given** the editor is open for a numeric value, **When** the user enters a non-numeric value, **Then** the system rejects it and keeps the editor open with an error.
4. **Given** the editor is open for a text or numeric value, **When** the user presses esc, **Then** the edit is canceled and the original value remains.

---

### User Story 3 - Navigate Submenus (Priority: P2)

As a user, I want to navigate into and out of submenus so I can manage grouped settings cleanly.

**Why this priority**: Some settings (like provider configuration and profiles) require grouped options.

**Independent Test**: Enter a submenu, verify the list updates, then press esc to return to the parent list.

---

### User Story 4 - Manage Profiles (Priority: P1)

As a user, I want to manage profiles from the settings dialog so I can create, rename, select, or delete profiles without leaving the TUI.

**Why this priority**: Profile management is a core configuration workflow and should be accessible alongside other settings.

**Independent Test**: Open the Profiles list and use enter/ctrl+n/ctrl+r/ctrl+d to manage profiles.

**Acceptance Scenarios**:

1. **Given** the settings dialog is open, **When** the user selects Profiles and presses enter, **Then** the Profiles list opens.
2. **Given** the Profiles list is open, **When** the user selects a profile and presses enter, **Then** the active profile switches and the UI updates.
3. **Given** the Profiles list is open, **When** the user presses ctrl+n or ctrl+r, **Then** a text editor opens and pressing enter saves the name.
4. **Given** the Profiles list is open, **When** the user presses ctrl+d, **Then** the settings dialog prompts for confirmation and pressing enter deletes the selected profile.

**Acceptance Scenarios**:

1. **Given** a settings entry represents a submenu, **When** the user selects it and presses enter, **Then** the submenu list opens.
2. **Given** the user is inside a submenu, **When** they press esc, **Then** the view returns to the previous menu level.

---

### Edge Cases

- If a setting has no current value, the list shows it as blank but still editable.
- If a user cancels out of an editor with esc, the original value remains unchanged.
- If a submenu has no entries, it shows an empty state instead of crashing.
- Active list entries are highlighted with color and a leading dot indicator instead of text labels.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST open the settings dialog when the user presses ctrl+j
- **FR-002**: The settings list MUST display key/value entries and submenu entries sorted alphabetically.
- **FR-003**: Users MUST be able to select a value entry and press enter to edit it.
- **FR-004**: The system MUST allow editing of text and numeric values through a dedicated textarea editor flow.
- **FR-004a**: The system MUST validate numeric values and reject invalid entries with an error state.
- **FR-004b**: The editor MUST support word-jump navigation via ctrl+←/→ and multi-line input via alt+enter.
- **FR-005**: Users MUST be able to enter submenus by selecting them and pressing enter.
- **FR-006**: When inside a submenu, pressing esc MUST return to the previous menu level.
- **FR-007**: When at the top-level settings menu, pressing esc MUST close the settings dialog.
- **FR-008**: The settings dialog MUST cover all app settings, including model selection, model extractor selection, provider configuration (submenu), profile management (list), token budget, and system prompt editing.
- **FR-008d**: The Profiles list MUST support enter to select, ctrl+n to create, ctrl+r to rename, and ctrl+d to delete profiles using the settings text editor.
- **FR-008f**: Deleting a profile from the Profiles list MUST require an in-dialog confirmation prompt, and pressing enter confirms deletion.
- **FR-008e**: Profile management MUST not rely on slash commands.
- **FR-008a**: The system MUST store the system prompt in the profile database, not a standalone file, defaulting to "You are Noto. A buddy who takes notes." when missing.
- **FR-008b**: The system MUST invalidate the conversation context cache whenever the system prompt is saved via the settings dialog.
- **FR-008c**: Provider settings MUST include Endpoint and Key fields; Key values MUST be obfuscated in the list view.
- **FR-009**: After a value is edited, the updated value MUST be persisted and displayed in the list.
- **FR-009a**: Long or multi-line values MUST be flattened and truncated with `...` in the list view.
- **FR-010**: The editor MUST save changes on enter and cancel changes on esc.
- **FR-011**: Lists MUST indicate the current selection using a colored style and a leading dot indicator instead of text labels, but only within the submenu where the selection is made.

### Non-Functional Requirements *(mandatory)*

- **NFR-001 Code Quality**: Changes MUST pass formatting, linting, and static analysis rules
  defined by the project.
- **NFR-002 Testing Standards**: Changes MUST include automated tests for new/changed behavior,
  including negative/error paths where applicable.
- **NFR-003 UX Consistency**: User-facing changes MUST follow established UX patterns
  (terminology, interaction flows, visual behavior) or document approved deviations.
- **NFR-004 Performance**: Settings interactions MUST feel instantaneous for typical user actions.

### Key Entities *(include if feature involves data)*

- **Settings Entry**: A named item representing a value or submenu.
- **Settings Menu**: A collection of settings entries at a given level.
- **Setting Value**: The editable value associated with a settings entry.
- **System Prompt**: The stored prompt value persisted in the profile database.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of settings entries appear in alphabetical order in the dialog.
- **SC-002**: Users can open the settings dialog with ctrl+j in under 1 second.
- **SC-003**: Users can edit a value entry and see the updated value reflected immediately.
- **SC-004**: esc returns to the previous menu level in submenus and closes the dialog at the top level.
- **SC-005**: All listed settings (model, model extractor, provider config, profile management, token budget, system prompt) are reachable within the dialog.
- **SC-006**: 0 lint/format violations in CI for the feature scope.
- **SC-007**: All new/changed behavior is covered by automated tests.

## Assumptions

- Settings are profile-scoped unless explicitly noted otherwise.
- Existing configuration flows (model selection, provider setup, system prompt edit) can be invoked from settings.
- The dialog can display blank values for unset settings without blocking edits.
- The system prompt is stored in the profile database (no separate prompt file) and defaults to "You are Noto. A buddy who takes notes." when missing.
