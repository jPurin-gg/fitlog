package main

import (
	"strings"
	"testing"
)

func TestNormalizeUserPreferencesRemovesAvoidedFromPreferred(t *testing.T) {
	preferences := normalizeUserPreferences(UserPreferences{
		UserID:              1,
		PreferredEquipment:  []string{" ダンベル ", "マシン"},
		AvoidedEquipment:    []string{"マシン"},
		TrainingEnvironment: " ジム ",
		Notes:               " 懸垂バーなし ",
	})

	if len(preferences.PreferredEquipment) != 1 || preferences.PreferredEquipment[0] != "ダンベル" {
		t.Fatalf("preferred_equipment = %v, want [ダンベル]", preferences.PreferredEquipment)
	}
	if len(preferences.AvoidedEquipment) != 1 || preferences.AvoidedEquipment[0] != "マシン" {
		t.Fatalf("avoided_equipment = %v, want [マシン]", preferences.AvoidedEquipment)
	}
	if preferences.TrainingEnvironment != "ジム" {
		t.Fatalf("training_environment = %q, want ジム", preferences.TrainingEnvironment)
	}
	if preferences.Notes != "懸垂バーなし" {
		t.Fatalf("notes = %q, want 懸垂バーなし", preferences.Notes)
	}
}

func TestUserPreferencesPromptText(t *testing.T) {
	text := userPreferencesPromptText(UserPreferences{
		PreferredEquipment:  []string{"ダンベル"},
		AvoidedEquipment:    []string{"バーベル"},
		TrainingEnvironment: "家",
		Notes:               "懸垂バーなし",
	})

	for _, want := range []string{"環境: 家", "優先する器具: ダンベル", "避けたい器具: バーベル", "メモ: 懸垂バーなし"} {
		if !strings.Contains(text, want) {
			t.Fatalf("userPreferencesPromptText() = %q, want to contain %q", text, want)
		}
	}
}
