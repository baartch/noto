package memory

import "testing"

func TestExtractionSchemaStrictShape(t *testing.T) {
	s := extractionSchema()
	if s["type"] != "object" {
		t.Fatalf("type = %v, want object", s["type"])
	}
	if s["additionalProperties"] != false {
		t.Fatalf("additionalProperties = %v, want false", s["additionalProperties"])
	}
	req, ok := s["required"].([]string)
	if !ok || len(req) != 3 {
		t.Fatalf("required = %v, want [has_new_info confidence notes]", s["required"])
	}
	props := s["properties"].(map[string]any)
	if len(props) != 3 {
		t.Fatalf("properties = %v, want exactly has_new_info confidence notes", props)
	}
	notes := props["notes"].(map[string]any)
	items := notes["items"].(map[string]any)
	if items["additionalProperties"] != false {
		t.Fatalf("items additionalProperties = %v, want false", items["additionalProperties"])
	}
	itemProps := items["properties"].(map[string]any)
	if len(itemProps) != 5 {
		t.Fatalf("item properties = %v, want exactly action target_id category content importance", itemProps)
	}
	if got := itemProps["action"].(map[string]any)["enum"].([]string); len(got) != 3 {
		t.Fatalf("action enum = %v, want [add update merge]", got)
	}
	if got := itemProps["category"].(map[string]any)["enum"].([]string); len(got) != 5 {
		t.Fatalf("category enum = %v, want [fact progress blocker action_item other]", got)
	}
	itemReq, ok := items["required"].([]string)
	if !ok || len(itemReq) != 5 {
		t.Fatalf("items required = %v, want all five item keys", items["required"])
	}
}
