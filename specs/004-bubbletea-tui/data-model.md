# Data Model: Bubble Tea TUI Standard

This feature standardizes TUI behavior and introduces explicit runtime telemetry entities used by the footer.

## Entities

### TUI Flow

Represents a user-facing terminal UI interaction sequence.

- **name**: Flow identifier (e.g., `model_picker`, `settings_dialog`)
- **entry_point**: Trigger path (startup, keybinding, command)
- **bubbletea_model**: Owning Bubble Tea model type
- **bubbles_components**: Components used (list/help/textarea/viewport/etc.)
- **lipgloss_styles**: Reused style identifiers
- **custom_ui_rationale**: Required when no suitable Bubbles component is used

### SessionUsageAccumulator

In-memory cumulative usage and cost totals displayed in footer.

- **tokens_up** (`int64`): Sum of prompt/input tokens
- **tokens_down** (`int64`): Sum of completion/output tokens
- **cache_read_tokens** (`int64`): Sum of cached prompt tokens
- **cache_write_tokens** (`int64`): Sum of cache write tokens
- **total_cost** (`float64`): Sum of provider-reported cost values
- **sources** (`set` conceptual): Participating model classes (`main`, `extractor`, `embeddings`)

### UsageSnapshot

Parsed usage payload from one provider response.

- **completion_tokens** (`int64`)
- **prompt_tokens** (`int64`)
- **cached_tokens** (`int64`, from `prompt_tokens_details.cached_tokens`)
- **cache_write_tokens** (`int64`, from `prompt_tokens_details.cache_write_tokens`)
- **cost** (`float64`)
- **source_model_class** (`enum`): `main|extractor|embeddings`
- **has_usage** (`bool`): false when provider response omits usage payload

### FooterStatusViewModel

Renderable footer state for current frame.

- **usage** (`SessionUsageAccumulator`)
- **ctx_cache_stats** (`string`): `ctx:<miss>|<hit>`
- **profile_name** (`string`)
- **main_model_name** (`string`)
- **app_version** (`string`)
- **help_keybinding** (`string`, default `Ctrl+H`)

## Relationships

- A **TUI Flow** renders a **FooterStatusViewModel** when applicable.
- **UsageSnapshot** events update one **SessionUsageAccumulator**.
- **SessionUsageAccumulator** is embedded/referenced by **FooterStatusViewModel**.

## Validation Rules / Invariants

- Every TUI flow maps to a Bubble Tea model.
- Every custom UI element includes rationale.
- Footer always renders required fields (usage, ctx cache, profile, model, version, help key).
- `SessionUsageAccumulator` updates only when `UsageSnapshot.has_usage == true`.
- Usage totals include snapshots from `main`, `extractor`, and `embeddings` model classes.
