package integration

// ValidExtractorPayload is a baseline valid extractor JSON payload fixture.
const ValidExtractorPayload = `{
  "has_new_info": true,
  "confidence": 0.86,
  "notes": [
    {
      "action": "add",
      "category": "fact",
      "content": "User prefers concise responses.",
      "importance": 7
    },
    {
      "action": "update",
      "target_id": "note_123",
      "category": "progress",
      "content": "Project moved from draft to implementation.",
      "importance": 8
    }
  ]
}`

// InvalidExtractorPayloadMissingTopLevel is invalid because required top-level fields are missing.
const InvalidExtractorPayloadMissingTopLevel = `{
  "notes": [
    {
      "action": "add",
      "category": "fact",
      "content": "Missing has_new_info and confidence"
    }
  ]
}`

// InvalidExtractorPayloadBadConfidence is invalid because confidence is out of range.
const InvalidExtractorPayloadBadConfidence = `{
  "has_new_info": true,
  "confidence": 1.5,
  "notes": []
}`

// InvalidExtractorPayloadUpdateMissingTarget is invalid because update note lacks target_id.
const InvalidExtractorPayloadUpdateMissingTarget = `{
  "has_new_info": true,
  "confidence": 0.66,
  "notes": [
    {
      "action": "update",
      "category": "progress",
      "content": "Updated status"
    }
  ]
}`
