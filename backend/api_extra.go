package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type StatCardData struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Unit  string `json:"unit"`
	Trend string `json:"trend"`
}

type WorkoutItemData struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	Duration string `json:"duration"`
	Calories string `json:"calories"`
	Time     string `json:"time"`
}

type DashboardResponse struct {
	Stats          []StatCardData    `json:"stats"`
	ChartData      []int             `json:"chart_data"`
	RecentWorkouts []WorkoutItemData `json:"recent_workouts"`
}

func (app *App) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := 1

	var resp DashboardResponse

	// Fake Stats but derived a bit
	resp.Stats = []StatCardData{
		{Label: "Calories", Value: "2,450", Unit: "kcal", Trend: "+12%"},
		{Label: "Heart Rate", Value: "72", Unit: "bpm", Trend: "-2%"},
		{Label: "Active Time", Value: "45", Unit: "min", Trend: "+5min"},
		{Label: "Weight", Value: "68.5", Unit: "kg", Trend: "-0.5kg"},
	}

	// ChartData (Last 7 days mock data, just static for now as simple DB fetch isn't fully comprehensive)
	resp.ChartData = []int{0, 0, 0, 0, 0, 0, 0}
	if app.db != nil {
		rows, err := app.db.Query(`
			SELECT generate_series(CURRENT_DATE - INTERVAL '6 days', CURRENT_DATE, '1 day')::date AS d
		`)
		if err == nil {
			defer rows.Close()
			var dates []time.Time
			for rows.Next() {
				var d time.Time
				rows.Scan(&d)
				dates = append(dates, d)
			}
			
			for i, d := range dates {
				var count int
				app.db.QueryRow(`
					SELECT COUNT(*) FROM workout_sets ws
					JOIN workouts w ON ws.workout_id = w.id
					WHERE w.user_id = $1 AND DATE(ws.created_at) = $2
				`, userID, d).Scan(&count)
				if i < len(resp.ChartData) {
					resp.ChartData[i] = count
				}
			}
		}
	}

	// Fetch up to 3 recent workouts
	if app.db != nil {
		rows, err := app.db.Query(`
			SELECT w.id, w.started_at, COALESCE(w.ended_at, w.started_at), COALESCE(w.notes, 'Strength')
			FROM workouts w
			WHERE w.user_id = $1
			ORDER BY w.started_at DESC
			LIMIT 3
		`, userID)

		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int
				var start time.Time
				var end time.Time
				var notes string
				if err := rows.Scan(&id, &start, &end, &notes); err == nil {
					
					// もし終了時間が同じ（記録中）なら、最後のセットの時間を取得
					var lastSetTime time.Time
					err = app.db.QueryRow("SELECT MAX(created_at) FROM workout_sets WHERE workout_id = $1", id).Scan(&lastSetTime)
					if err == nil && !lastSetTime.IsZero() && lastSetTime.After(start) {
						end = lastSetTime
					}

					dur := end.Sub(start).Minutes()
					if dur < 1 {
						dur = 1 // 少なくとも1分
					}

					title := notes + " Workout"
					if notes == "Strength" {
						title = "Workout Session"
					}

					resp.RecentWorkouts = append(resp.RecentWorkouts, WorkoutItemData{
						ID:       id,
						Title:    title,
						Type:     notes,
						Duration: fmt.Sprintf("%.0f min", dur),
						Calories: fmt.Sprintf("%.0f kcal", dur*5), // mock
						Time:     start.Format("03:04 PM"),
					})
				}
			}
		}
	}

if len(resp.RecentWorkouts) == 0 {
resp.RecentWorkouts = []WorkoutItemData{
{ID: 1, Title: "Morning Yoga", Type: "Flexibility", Duration: "30 min", Calories: "120 kcal", Time: "08:00 AM"},
}
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)
}

type WorkedOutDay struct {
	Date int    `json:"date"`
	Type string `json:"type"`
}

type CalendarResponse struct {
	WorkedOutDates []int          `json:"worked_out_dates"`
	WorkedOutDays  []WorkedOutDay `json:"worked_out_days"`
}

func (app *App) handleCalendar(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodGet {
http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
return
}

userID := 1
yearStr := r.URL.Query().Get("year")
monthStr := r.URL.Query().Get("month")

year, _ := strconv.Atoi(yearStr)
month, _ := strconv.Atoi(monthStr)

if year == 0 { year = time.Now().Year() }
if month == 0 { month = int(time.Now().Month()) }

	var resp CalendarResponse
	resp.WorkedOutDates = []int{}
	resp.WorkedOutDays = []WorkedOutDay{}

	// get unique days in that month the user worked out
	rows, err := app.db.Query(`
		SELECT EXTRACT(DAY FROM started_at), MAX(COALESCE(notes, 'Workout'))
		FROM workouts
		WHERE user_id = $1
		  AND EXTRACT(YEAR FROM started_at) = $2
		  AND EXTRACT(MONTH FROM started_at) = $3
		GROUP BY EXTRACT(DAY FROM started_at)
	`, userID, year, month)

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var d float64
			var t string
			if err := rows.Scan(&d, &t); err == nil {
				resp.WorkedOutDates = append(resp.WorkedOutDates, int(d))
				resp.WorkedOutDays = append(resp.WorkedOutDays, WorkedOutDay{
					Date: int(d),
					Type: t,
				})
			}
		}
	}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)
}

type AlternativeRequest struct {
	ExerciseID string `json:"exercise_id"`
	Exercise   string `json:"exercise"`
	Reason     string `json:"reason"`
}

type AlternativeExercise struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AlternativeResponse struct {
	Alternatives []AlternativeExercise `json:"alternatives"`
	Message      string                `json:"message"`
}

func (app *App) handleAlternative(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AlternativeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	resp := AlternativeResponse{
		Alternatives: []AlternativeExercise{},
		Message:      "機材が埋まっていても大丈夫です。代わりに以下の種目でしっかり追い込みましょう！",
	}

	if app.db != nil && req.ExerciseID != "" {
		// 1. 元の種目の対象筋肉を取得
		var pMusclesJSON []byte
		err := app.db.QueryRow("SELECT primary_muscles FROM exercises WHERE id = $1 LIMIT 1", req.ExerciseID).Scan(&pMusclesJSON)
		
		if err == nil {
			var pMuscles []string
			json.Unmarshal(pMusclesJSON, &pMuscles)
			
			if len(pMuscles) > 0 {
				var arr []string
				for _, m := range pMuscles {
					arr = append(arr, fmt.Sprintf("'%s'", strings.ReplaceAll(m, "'", "''")))
				}
				arrStr := "ARRAY[" + strings.Join(arr, ",") + "]"
				
				// 同じ筋肉を鍛えられる他の種目を最大30件取得
				rows, err := app.db.Query(fmt.Sprintf(`
					SELECT id, name, equipment FROM exercises 
					WHERE primary_muscles ?| %s 
					  AND id != $1 
					LIMIT 30
				`, arrStr), req.ExerciseID)
				
				if err == nil {
					defer rows.Close()
					var exList []string
					for rows.Next() {
						var idStr, nStr, eqStr string
						rows.Scan(&idStr, &nStr, &eqStr)
						exList = append(exList, fmt.Sprintf("- ID: %s, Name: %s (器具: %s)", idStr, nStr, eqStr))
					}
					
					if len(exList) > 0 {
						dbContext := "以下の種目が同じ筋肉( " + strings.Join(pMuscles, ", ") + " )を鍛えられるデータベース内の候補です:\n" + strings.Join(exList, "\n")
						
						systemPrompt := `あなたは優秀なパーソナルトレーナーAIです。ユーザーが現在行おうとしている種目を別の種目に変更したいと考えています。
データベース内の候補リストの中から、ユーザーの理由（例: マシンが空いていない等）に最も適した代替種目を2〜3個選び、JSON形式で出力してください。
選んだ種目のIDとNameは必ず候補リストにあるものをそのまま使用してください。
以下のJSONフォーマットに必ず従ってください:
{
  "message": "励ましのメッセージやアドバイス",
  "alternatives": [
    {
      "id": "候補リストにあるID",
      "name": "候補リストにあるName",
      "description": "なぜこの種目がおすすめなのか、理由や簡単なやり方"
    }
  ]
}`
						userPrompt := fmt.Sprintf("変更したい元の種目: %s\n変更したい理由: %s\n\n%s", req.Exercise, req.Reason, dbContext)
						
						aiJSON, err := callAI(systemPrompt, userPrompt, true)
						if err == nil {
							aiStr := strings.TrimSpace(aiJSON)
							if strings.HasPrefix(aiStr, "```json") {
								aiStr = strings.TrimPrefix(aiStr, "```json")
								aiStr = strings.TrimSuffix(strings.TrimSpace(aiStr), "```")
							}
							json.Unmarshal([]byte(aiStr), &resp)
						} else {
							fmt.Printf("AI Alternative Error: %v\n", err)
						}
					}
				}
			}
		}
	}

	// AI失敗時のフォールバック
	if len(resp.Alternatives) == 0 {
		ex := req.Exercise
		if strings.Contains(ex, "ベンチ") || strings.Contains(ex, "胸") {
			resp.Alternatives = []AlternativeExercise{
				{ID: "Dumbbell_Bench_Press", Name: "ダンベルプレス", Description: "ベンチ台が空いていればダンベルで同等の効果を得られます。"},
			}
		} else {
			resp.Alternatives = []AlternativeExercise{
				{ID: "custom_alt_" + fmt.Sprintf("%d", time.Now().Unix()), Name: ex + " (代替)", Description: "現在AIが代替種目を取得できませんでした。"},
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
