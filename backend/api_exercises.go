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

	// クエリパラメータの取得（例: ?muscle=胸&equipment=ダンベル）
	muscle := r.URL.Query().Get("muscle")
	equipment := r.URL.Query().Get("equipment")

	query := `SELECT id, name, force, level, mechanic, equipment, category, instructions, primary_muscles, secondary_muscles, images FROM exercises WHERE 1=1`
	args := []interface{}{}
	argId := 1

	if muscle != "" {
		// JSON配列の中に特定の筋肉名が含まれているかテキスト検索（英語・日本語両対応のためLIKEを使用）
		query += ` AND (primary_muscles::text LIKE $` + fmt.Sprint(argId) + ` OR secondary_muscles::text LIKE $` + fmt.Sprint(argId) + `)`
		args = append(args, "%"+muscle+"%")
		argId++
	}

	if equipment != "" {
		query += ` AND equipment = $` + fmt.Sprint(argId)
		args = append(args, equipment)
		argId++
	}

	query += ` ORDER BY name ASC LIMIT 100`

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

		// DBに保存されているJSON文字列をGoの配列に変換
		if instJSON != nil { json.Unmarshal(instJSON, &ex.Instructions) }
		if primJSON != nil { json.Unmarshal(primJSON, &ex.PrimaryMuscles) }
		if secJSON != nil { json.Unmarshal(secJSON, &ex.SecondaryMuscles) }
		if imgJSON != nil { json.Unmarshal(imgJSON, &ex.Images) }

		exercises = append(exercises, ex)
	}

	// nilスライスを空配列にして返すための対策
	if exercises == nil {
		exercises = []ExerciseData{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(exercises)
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
