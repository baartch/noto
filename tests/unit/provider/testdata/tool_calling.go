package testdata

const ResponsesRequestWithTools = `{
  "model": "openrouter/test-model",
  "input": [
    {
      "role": "user",
      "content": [{"type": "input_text", "text": "Find notes about launch planning"}]
    }
  ],
  "tools": [
    {
      "type": "function",
      "name": "search_memory_keywords",
      "description": "Search memory by topic keywords",
      "parameters": {
        "type": "object",
        "properties": {
          "query": {"type": "string"},
          "limit": {"type": "integer"}
        },
        "required": ["query"]
      }
    }
  ]
}`

const ResponsesToolCallOutput = `{
  "id": "resp_1",
  "model": "openrouter/test-model",
  "output": [
    {
      "type": "function_call",
      "id": "fc_1",
      "call_id": "call_1",
      "name": "search_memory_keywords",
      "arguments": "{\"query\":\"launch planning\",\"limit\":3}"
    }
  ]
}`

const ResponsesFollowupWithToolResult = `{
  "model": "openrouter/test-model",
  "input": [
    {
      "role": "assistant",
      "content": [{"type": "output_text", "text": ""}]
    },
    {
      "type": "function_call_output",
      "call_id": "call_1",
      "output": "[{\"record_type\":\"raw_note\",\"record_id\":\"n1\",\"content\":\"Launch planning starts Monday\"}]"
    }
  ],
  "tools": [
    {
      "type": "function",
      "name": "search_memory_keywords"
    }
  ]
}`
