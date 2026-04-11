# Data Model: Settings Dialog Navigation

## Entities

### Settings Menu

- **id**: string
- **title**: string
- **entries**: list of Settings Entry
- **parent_id**: string (nullable)

### Settings Entry

- **id**: string
- **label**: string
- **kind**: value | submenu
- **value_type**: text | number | action (nullable)
- **value**: string (nullable)
- **submenu_id**: string (nullable)
- **source**: model | extractor_model | provider_config | token_budget | system_prompt
- **validation**: numeric-only for number entries

### System Prompt Record

- **profile_id**: string
- **prompt**: string
- **updated_at**: timestamp

### Settings Editor State

- **entry_id**: string
- **draft_value**: string
- **status**: editing | saved | canceled

## Relationships

- Settings Menu 1→N Settings Entry
- Settings Entry 0→1 Settings Menu (submenu)
- Settings Entry 1→1 Settings Editor State (during editing)
