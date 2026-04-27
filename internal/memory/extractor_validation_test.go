package memory

import "testing"

func TestValidateExtractionPayload_Valid(t *testing.T) {
	p := extractionResponse{
		HasNewInfo: true,
		Confidence: 0.8,
		Notes: []extractedItem{
			{Action: "add", Category: "fact", Content: "A", Importance: 5},
			{Action: "update", TargetID: "note_1", Category: "progress", Content: "B", Importance: 6},
		},
	}
	if err := validateExtractionPayload(p); err != nil {
		t.Fatalf("expected valid payload, got %v", err)
	}
}

func TestValidateExtractionPayload_HasNewInfoTrueEmptyNotes(t *testing.T) {
	p := extractionResponse{HasNewInfo: true, Confidence: 0.7, Notes: []extractedItem{}}
	if err := validateExtractionPayload(p); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateExtractionPayload_ConfidenceRange(t *testing.T) {
	p := extractionResponse{HasNewInfo: true, Confidence: 1.2}
	if err := validateExtractionPayload(p); err == nil {
		t.Fatal("expected confidence range error")
	}
}

func TestValidateExtractionPayload_UpdateRequiresTargetID(t *testing.T) {
	p := extractionResponse{
		HasNewInfo: true,
		Confidence: 0.7,
		Notes: []extractedItem{{Action: "update", Category: "progress", Content: "A", Importance: 5}},
	}
	if err := validateExtractionPayload(p); err == nil {
		t.Fatal("expected update target_id error")
	}
}
