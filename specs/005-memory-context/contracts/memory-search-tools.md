# Contract: Memory Search Tools via OpenRouter Tool Calling

## Tool Calling Model

The chat provider request MUST support OpenRouter-compatible tool-calling flow:

1. Send the normal conversation request with `tools` definitions included.
2. If the model emits `tool_calls`, execute the requested tool locally.
3. Send a follow-up provider request containing:
   - the original conversation turns,
   - the assistant tool-call request,
   - the corresponding tool result messages,
   - the same `tools` definitions.
4. Return the final assistant answer after the provider processes tool results.

## Tool 1: Keyword Memory Search

### Tool Name

`search_memory_keywords`

### Purpose

Retrieve memory records relevant to a keyword/topic request using the existing vector-backed memory retrieval path, with graceful fallback if vector retrieval is unavailable.

### Input Schema

- **query**: string
- **limit**: integer (optional)

### Output Shape

Ordered list of result items containing:

- **record_type**: `raw_note | weekly_summary | monthly_summary`
- **record_id**: string
- **content**: string
- **category**: string (nullable)
- **time_start**: timestamp
- **time_end**: timestamp
- **relevance_score**: number (nullable when fallback path cannot score)

### Behavior Rules

1. Empty query MUST return an empty result set or a validation error payload, not a provider failure.
2. If vector search is available, keyword search SHOULD return relevance-ranked results.
3. If vector search is unavailable, the tool MUST degrade gracefully with a deterministic fallback path.
4. Results MUST be scoped to the active profile.

## Tool 2: Time-Range Memory Search

### Tool Name

`search_memory_time_range`

### Purpose

Retrieve raw notes and summary artifacts whose timestamps fall within a requested time range.

### Input Schema

- **start_time**: timestamp/string
- **end_time**: timestamp/string
- **limit**: integer (optional)

### Output Shape

Ordered list of result items containing:

- **record_type**: `raw_note | weekly_summary | monthly_summary`
- **record_id**: string
- **content**: string
- **category**: string (nullable)
- **time_start**: timestamp
- **time_end**: timestamp

### Behavior Rules

1. Only records within the requested range may be returned.
2. Both raw notes and summary records may appear when both are in-range.
3. Invalid ranges must return a structured validation error or empty result, not break the conversation.
4. Results MUST be scoped to the active profile.

## Tool Availability Rules

1. Tools MUST only be sent to the provider when the active model/provider supports tool calling.
2. If tool calling is unavailable for the active model, the system MUST continue basic chat without failing the interaction.
3. Tool schemas MUST be included on both the initial request and the follow-up request containing tool results.

## Traceability

- Spec §FR-047 through §FR-050
- Spec §FR-056
- Spec §SC-006 through §SC-007
