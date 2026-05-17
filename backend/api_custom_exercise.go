package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type CustomExerciseReq struct {
	Name             string   `json:"name"`
	Category         string   `json:"category"`
	Equipment        string   `json:"equipment"`
	Primary          string   `json:"primary_muscle"`
	PrimaryMuscles   []string `json:"primary_muscles"`
	SecondaryMuscles []string `json:"secondary_muscles"`
	Instructions     []string `json:"instructions"`
}

func (app *App) handleAddCustomExercise(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CustomExerciseReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 必須入力のバリデーション
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		http.Error(w, "種目名（Name）は必須入力です", http.StatusBadRequest)
		return
	}

	// IDを自動生成（被らないようにタイムスタンプを利用）
	id := fmt.Sprintf("custom_%d", time.Now().UnixNano())

	primaryMuscles := cleanStringList(req.PrimaryMuscles)
	if req.Primary != "" && len(primaryMuscles) == 0 {
		primaryMuscles = []string{strings.TrimSpace(req.Primary)}
	}
	secondaryMuscles := cleanStringList(req.SecondaryMuscles)
	instructions := cleanStringList(req.Instructions)

	primaryMusclesJSON, _ := json.Marshal(primaryMuscles)
	secondaryMusclesJSON, _ := json.Marshal(secondaryMuscles)
	instructionsJSON, _ := json.Marshal(instructions)

	// SQLインジェクション対策（プレースホルダー $1, $2... を必ず使用）
	query := `
		INSERT INTO exercises (
			id, name, category, equipment, primary_muscles, 
			force, level, mechanic, instructions, secondary_muscles, images
		)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5::jsonb, NULL, NULL, NULL, $6::jsonb, $7::jsonb, '[]'::jsonb)
	`
	_, err := app.db.Exec(query, id, req.Name, strings.TrimSpace(req.Category), strings.TrimSpace(req.Equipment), string(primaryMusclesJSON), string(instructionsJSON), string(secondaryMusclesJSON))
	if err != nil {
		log.Printf("Error inserting custom exercise: %v\n", err)
		http.Error(w, "Failed to insert custom exercise", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully created custom exercise: %s (%s)\n", req.Name, id)

	// フロントエンドに作成した種目のデータを返す
	category := strings.TrimSpace(req.Category)
	equipment := strings.TrimSpace(req.Equipment)
	res := ExerciseData{
		ID:               id,
		Name:             req.Name,
		Category:         stringPtrOrNil(category),
		Equipment:        stringPtrOrNil(equipment),
		Instructions:     instructions,
		PrimaryMuscles:   primaryMuscles,
		SecondaryMuscles: secondaryMuscles,
		Images:           []string{},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func cleanStringList(values []string) []string {
	cleaned := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
