package memory

// extractionSchema returns the strict JSON schema for the extraction contract,
// mirroring the JSON shape of extractionResponse and extractedItem. Task 4
// passes it as provider.ResponseFormat.Schema.
//
// mergeIDs is deliberately absent: it is unexported, set only by
// reconcileAdditions, and never accepted from the LLM. The importance 1-10
// range and the update-requires-target_id rule stay enforced by
// validateExtractionPayload and are not encoded here.
func extractionSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"has_new_info": map[string]any{
				"type":        "boolean",
				"description": "Whether the exchange contains at least one piece of durable information worth remembering.",
			},
			"confidence": map[string]any{
				"type":        "number",
				"description": "Confidence in the extraction, from 0.0 to 1.0.",
			},
			"notes": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"add", "update", "merge"},
							"description": "Action to take: add a new note, update an existing note, or merge duplicates.",
						},
						"target_id": map[string]any{
							"type":        "string",
							"description": "ID of the existing note to update. Required when action is update; empty otherwise.",
						},
						"category": map[string]any{
							"type":        "string",
							"enum":        []string{"fact", "progress", "blocker", "action_item", "other"},
							"description": "The note category.",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "The note content, concise and self-contained.",
						},
						"importance": map[string]any{
							"type":        "integer",
							"description": "Importance from 1 (trivial) to 10 (critical).",
						},
					},
					"required":             []string{"action", "target_id", "category", "content", "importance"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"has_new_info", "confidence", "notes"},
		"additionalProperties": false,
	}
}
