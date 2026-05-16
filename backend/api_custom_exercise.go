package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type CustomExerciseReq struct {
	Name      string `json:"name"`
	Category  string `json:"category"`
	Equipment string `json:"equipment"`
	Primary   string `json:"primary_muscle"`
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
	if req.Name == "" {
		http.Error(w, "種目名（Name）は必須入力です", http.StatusBadRequest)
		return
	}

	// IDを自動生成（被らないようにタイムスタンプを利用）
	id := fmt.Sprintf("custom_%d", time.Now().UnixNano())

	// jsonb配列として筋肉を保存するための処理
	var primaryMusclesJSON string
	if req.Primary != "" {
		primaryArr := []string{req.Primary}
		bytes, _ := json.Marshal(primaryArr)
		primaryMusclesJSON = string(bytes)
	} else {
		primaryMusclesJSON = "[]" // 入力がなければ空のJSON配列
	}

	// SQLインジェクション対策（プレースホルダー $1, $2... を必ず使用）
	query := `
		INSERT INTO exercises (
			id, name, category, equipment, primary_muscles, 
			force, level, mechanic, instructions, secondary_muscles, images
		)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, NULL, NULL, NULL, '[]'::jsonb, '[]'::jsonb, '[]'::jsonb)
	`
	_, err := app.db.Exec(query, id, req.Name, req.Category, req.Equipment, primaryMusclesJSON)
	if err != nil {
		log.Printf("Error inserting custom exercise: %v\n", err)
		http.Error(w, "Failed to insert custom exercise", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully created custom exercise: %s (%s)\n", req.Name, id)

	// フロントエンドに作成した種目のデータを返す
	res := map[string]string{
		"id":        id,
		"name":      req.Name,
		"category":  req.Category,
		"equipment": req.Equipment,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
