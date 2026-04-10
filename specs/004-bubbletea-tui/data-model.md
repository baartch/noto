# Data Model: Bubble Tea TUI Standard

This feature is a refactor/standardization effort rather than a data model change. The relevant "entities" are the TUI flows and their component usage.

## Entities

### TUI Flow

Represents a user-facing terminal UI interaction sequence.

- **Name**: Human-readable identifier for the flow (e.g., "Model Picker")
- **Entry Point**: Where the flow is initiated (command, keybinding, startup path)
- **Bubble Tea Model**: The model implementing the flow
- **Bubbles Components Used**: List of Bubbles components used in the flow (if any)
- **Lip Gloss Styles**: List of style definitions used by the flow
- **Custom UI**: Notes on any custom UI and rationale for not using Bubbles

### Component Usage Record

Captures the choice between Bubbles component and custom UI.

- **Component Name**: Name of the Bubbles component considered/used
- **Decision**: Used / Not Used
- **Rationale**: Short explanation when not used

### Style Definition Record

Captures standard styling definitions applied across the TUI.

- **Style Name**: Identifier for the Lip Gloss style
- **Usage**: Where the style is applied (headers, lists, prompts)
- **Notes**: Purpose/intent of the style

## Invariants

- Every TUI flow must map to a Bubble Tea model.
- Any custom UI must include a rationale for not using a Bubbles component.
- Styling is defined via Lip Gloss in reusable blocks.
