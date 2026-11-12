package main

import (
	"encoding/json"
	"testing"

	"github.com/jPurin-gg/myfitlog-backend/internal/exercise"
)

// The search API compares equipment and muscle filters for equality, so a
// choice that is spelled differently from the catalog can never match and a
// catalog value without a choice can never be selected.
func TestFilterOptionsMatchExerciseCatalog(t *testing.T) {
	var catalog []exercise.Exercise
	if err := json.Unmarshal(exerciseSeed, &catalog); err != nil {
		t.Fatalf("parse exercise seed: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("exercise seed is empty")
	}

	equipmentCounts := map[string]int{}
	muscleCounts := map[string]int{}
	for _, item := range catalog {
		if item.Equipment != nil {
			equipmentCounts[*item.Equipment]++
		}
		counted := map[string]bool{}
		for _, muscles := range [][]string{item.PrimaryMuscles, item.SecondaryMuscles} {
			for _, muscle := range muscles {
				if !counted[muscle] {
					counted[muscle] = true
					muscleCounts[muscle]++
				}
			}
		}
	}

	options := exercise.AvailableFilterOptions()
	equipmentOptions := map[string]bool{}
	for _, equipment := range options.Equipment {
		if equipmentOptions[equipment] {
			t.Errorf("equipment option %q is listed twice", equipment)
		}
		equipmentOptions[equipment] = true
		if equipmentCounts[equipment] == 0 {
			t.Errorf("equipment option %q matches no catalog exercise", equipment)
		}
	}
	for equipment, count := range equipmentCounts {
		if !equipmentOptions[equipment] {
			t.Errorf("catalog equipment %q (%d exercises) is missing from the options", equipment, count)
		}
	}

	muscleOptions := map[string]bool{}
	for _, category := range options.MuscleCategories {
		if len(category.Muscles) == 0 {
			t.Errorf("muscle category %q has no muscles", category.Name)
		}
		for _, muscle := range category.Muscles {
			if muscleOptions[muscle] {
				t.Errorf("muscle option %q is listed twice", muscle)
			}
			muscleOptions[muscle] = true
			if muscleCounts[muscle] == 0 {
				t.Errorf("muscle option %q matches no catalog exercise", muscle)
			}
		}
	}
	for muscle, count := range muscleCounts {
		if !muscleOptions[muscle] {
			t.Errorf("catalog muscle %q (%d exercises) is missing from the options", muscle, count)
		}
	}
}
