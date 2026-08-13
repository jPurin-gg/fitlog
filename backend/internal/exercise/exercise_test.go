package exercise

import "testing"

func TestValidateAlternativesRejectsUnknownIDsAndUsesDictionaryName(t *testing.T) {
	allowed := map[string]AlternativeCandidate{
		"dumbbell-bench": {ID: "dumbbell-bench", Name: "ダンベルベンチプレス"},
	}
	response := AlternativeResponse{Alternatives: []Alternative{{ID: "dumbbell-bench", Name: "hallucinated name"}}}
	if err := validateAlternatives(&response, allowed); err != nil {
		t.Fatalf("validateAlternatives() error = %v", err)
	}
	if response.Alternatives[0].Name != "ダンベルベンチプレス" {
		t.Fatalf("name = %q", response.Alternatives[0].Name)
	}
	response.Alternatives[0].ID = "unknown"
	if err := validateAlternatives(&response, allowed); err == nil {
		t.Fatal("validateAlternatives(unknown) error = nil")
	}
}
