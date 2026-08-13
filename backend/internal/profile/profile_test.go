package profile

import "testing"

func TestNormalizeRemovesAvoidedEquipmentFromPreferred(t *testing.T) {
	preferences := Normalize(Preferences{
		PreferredEquipment:  []string{" ダンベル ", "マシン"},
		AvoidedEquipment:    []string{"マシン"},
		TrainingEnvironment: " ジム ",
		Notes:               " 懸垂バーなし ",
	})
	if len(preferences.PreferredEquipment) != 1 || preferences.PreferredEquipment[0] != "ダンベル" {
		t.Fatalf("preferred_equipment = %v", preferences.PreferredEquipment)
	}
	if preferences.TrainingEnvironment != "ジム" || preferences.Notes != "懸垂バーなし" {
		t.Fatalf("normalized text = %q, %q", preferences.TrainingEnvironment, preferences.Notes)
	}
}
