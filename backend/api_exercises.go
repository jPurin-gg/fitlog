package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func (app *App) handleExercises(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// クエリパラメータ
	muscle := r.URL.Query().Get("muscle")
	// equipment はカンマ区切りで複数指定可能 (例: ?equipment=ダンベル,バーベル)
	equipmentParam := r.URL.Query().Get("equipment")
	nameQuery := r.URL.Query().Get("name")

	query := `SELECT id, name, force, level, mechanic, equipment, category, instructions, primary_muscles, secondary_muscles, images FROM exercises WHERE 1=1`
	args := []interface{}{}
	argId := 1

	// 種目名のキーワード検索（日本語名・英語ID両対応）
	if nameQuery != "" {
		query += ` AND (name ILIKE $` + fmt.Sprint(argId) + ` OR id ILIKE $` + fmt.Sprint(argId) + `)`
		args = append(args, "%"+nameQuery+"%")
		argId++
	}

	// 筋肉名フィルタ（主動筋または補助筋）
	if muscle != "" {
		query += ` AND (primary_muscles::text ILIKE $` + fmt.Sprint(argId) + ` OR secondary_muscles::text ILIKE $` + fmt.Sprint(argId) + `)`
		args = append(args, "%"+muscle+"%")
		argId++
	}

	// 器具フィルタ（複数選択: カンマ区切り）
	if equipmentParam != "" {
		equipments := splitAndTrim(equipmentParam)
		if len(equipments) == 1 {
			query += ` AND equipment = $` + fmt.Sprint(argId)
			args = append(args, equipments[0])
			argId++
		} else if len(equipments) > 1 {
			query += ` AND equipment = ANY($` + fmt.Sprint(argId) + `)`
			args = append(args, equipments)
			argId++
		}
	}

	query += ` ORDER BY name ASC`

	rows, err := app.db.Query(query, args...)
	if err != nil {
		log.Printf("Error querying exercises: %v\n", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var exercises []ExerciseData
	for rows.Next() {
		var ex ExerciseData
		var instJSON, primJSON, secJSON, imgJSON []byte

		err := rows.Scan(
			&ex.ID, &ex.Name, &ex.Force, &ex.Level, &ex.Mechanic, &ex.Equipment, &ex.Category,
			&instJSON, &primJSON, &secJSON, &imgJSON,
		)
		if err != nil {
			log.Printf("Error scanning exercise row: %v\n", err)
			continue
		}

		if instJSON != nil { json.Unmarshal(instJSON, &ex.Instructions) }
		if primJSON != nil { json.Unmarshal(primJSON, &ex.PrimaryMuscles) }
		if secJSON != nil { json.Unmarshal(secJSON, &ex.SecondaryMuscles) }
		if imgJSON != nil { json.Unmarshal(imgJSON, &ex.Images) }

		exercises = append(exercises, ex)
	}

	if exercises == nil {
		exercises = []ExerciseData{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(exercises)
}

// splitAndTrim はカンマ区切り文字列を分割してトリムする
func splitAndTrim(s string) []string {
	var result []string
	for _, part := range splitComma(s) {
		if t := trimSpace(part); t != "" {
			result = append(result, t)
		}
	}
	return result
}

func splitComma(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// handleTargetSets - 種目ごとの目標セット数の取得・更新
func (app *App) handleTargetSets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		exerciseID := r.URL.Query().Get("exercise_id")
		if userID == "" || exerciseID == "" {
			http.Error(w, "user_id and exercise_id are required", http.StatusBadRequest)
			return
		}
		var targetSets int
		err := app.db.QueryRow(
			`SELECT COALESCE(target_sets, 3) FROM user_exercise_stats WHERE user_id = $1 AND exercise_id = $2`,
			userID, exerciseID,
		).Scan(&targetSets)
		if err != nil {
			// レコードなし = デフォルト3セット
			targetSets = 3
		}
		json.NewEncoder(w).Encode(map[string]int{"target_sets": targetSets})

	case http.MethodPost:
		var req struct {
			UserID     int    `json:"user_id"`
			ExerciseID string `json:"exercise_id"`
			TargetSets int    `json:"target_sets"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.ExerciseID == "" || req.TargetSets < 1 {
			http.Error(w, "exercise_id and target_sets (>=1) are required", http.StatusBadRequest)
			return
		}
		_, err := app.db.Exec(`
			INSERT INTO user_exercise_stats (user_id, exercise_id, target_sets)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id, exercise_id)
			DO UPDATE SET target_sets = EXCLUDED.target_sets, updated_at = CURRENT_TIMESTAMP
		`, req.UserID, req.ExerciseID, req.TargetSets)
		if err != nil {
			log.Printf("Failed to update target_sets: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "target_sets": req.TargetSets})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetExercises は handleExercises の別名（main.goの重複登録解消用）
func (app *App) handleGetExercises(w http.ResponseWriter, r *http.Request) {
	app.handleExercises(w, r)
}
