package main

import (
	"database/sql"
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

	userID := parseUserID(r.URL.Query().Get("user_id"))
	_ = app.closeStaleWorkouts(userID)

	var resp DashboardResponse

	// Fake Stats but derived a bit
	resp.Stats = []StatCardData{
		{Label: "消費カロリー", Value: "2,450", Unit: "kcal", Trend: "+12%"},
		{Label: "心拍数", Value: "72", Unit: "bpm", Trend: "-2%"},
		{Label: "活動時間", Value: "45", Unit: "分", Trend: "+5分"},
		{Label: "体重", Value: "68.5", Unit: "kg", Trend: "-0.5kg"},
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

	// Fetch up to 3 completed recent workouts
	if app.db != nil {
		rows, err := app.db.Query(`
			SELECT w.id, w.started_at, w.ended_at, COALESCE(w.notes, '筋トレ')
			FROM workouts w
			WHERE w.user_id = $1 AND w.ended_at IS NOT NULL
			ORDER BY w.ended_at DESC, w.started_at DESC
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

					dur := end.Sub(start).Minutes()
					if dur < 1 {
						dur = 1 // 少なくとも1分
					}

					workoutType := displayWorkoutType(notes)
					title := workoutType + "の記録"

					resp.RecentWorkouts = append(resp.RecentWorkouts, WorkoutItemData{
						ID:       id,
						Title:    title,
						Type:     workoutType,
						Duration: fmt.Sprintf("%.0f分", dur),
						Calories: fmt.Sprintf("%.0f kcal", dur*5), // mock
						Time:     start.Format("15:04"),
					})
				}
			}
		}
	}

	if len(resp.RecentWorkouts) == 0 {
		resp.RecentWorkouts = []WorkoutItemData{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type WorkedOutDay struct {
	Date      int    `json:"date"`
	WorkoutID int    `json:"workout_id"`
	Type      string `json:"type"`
}

type CalendarResponse struct {
	WorkedOutDates []int          `json:"worked_out_dates"`
	WorkedOutDays  []WorkedOutDay `json:"worked_out_days"`
}

type CalendarWorkoutSet struct {
	ID           int     `json:"id,omitempty"`
	ExerciseID   string  `json:"exercise_id"`
	ExerciseName string  `json:"exercise_name,omitempty"`
	Weight       float64 `json:"weight"`
	Reps         int     `json:"reps"`
	SetOrder     int     `json:"set_order"`
	Feeling      string  `json:"feeling"`
}

type CalendarWorkoutResponse struct {
	Exists    bool                 `json:"exists"`
	WorkoutID int                  `json:"workout_id,omitempty"`
	UserID    int                  `json:"user_id"`
	Date      string               `json:"date"`
	Title     string               `json:"title"`
	Sets      []CalendarWorkoutSet `json:"sets"`
}

type SaveCalendarWorkoutRequest struct {
	UserID int                  `json:"user_id"`
	Date   string               `json:"date"`
	Title  string               `json:"title"`
	Sets   []CalendarWorkoutSet `json:"sets"`
}

func (app *App) handleCalendar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := parseUserID(r.URL.Query().Get("user_id"))
	_ = app.closeStaleWorkouts(userID)
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")

	year, _ := strconv.Atoi(yearStr)
	month, _ := strconv.Atoi(monthStr)

	if year == 0 {
		year = time.Now().Year()
	}
	if month == 0 {
		month = int(time.Now().Month())
	}

	var resp CalendarResponse
	resp.WorkedOutDates = []int{}
	resp.WorkedOutDays = []WorkedOutDay{}

	// get the latest completed workout for each day in that month
	rows, err := app.db.Query(`
		SELECT DISTINCT ON (EXTRACT(DAY FROM started_at))
			EXTRACT(DAY FROM started_at),
			id,
			COALESCE(notes, 'ワークアウト')
		FROM workouts
		WHERE user_id = $1
			AND ended_at IS NOT NULL
			AND EXTRACT(YEAR FROM started_at) = $2
			AND EXTRACT(MONTH FROM started_at) = $3
		ORDER BY EXTRACT(DAY FROM started_at), started_at DESC, id DESC
	`, userID, year, month)

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var d float64
			var workoutID int
			var t string
			if err := rows.Scan(&d, &workoutID, &t); err == nil {
				resp.WorkedOutDates = append(resp.WorkedOutDates, int(d))
				resp.WorkedOutDays = append(resp.WorkedOutDays, WorkedOutDay{
					Date:      int(d),
					WorkoutID: workoutID,
					Type:      displayWorkoutType(t),
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (app *App) handleCalendarWorkout(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		app.getCalendarWorkout(w, r)
	case http.MethodPut:
		app.saveCalendarWorkout(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (app *App) getCalendarWorkout(w http.ResponseWriter, r *http.Request) {
	userID := parseUserID(r.URL.Query().Get("user_id"))
	dateText := strings.TrimSpace(r.URL.Query().Get("date"))
	day, err := time.Parse("2006-01-02", dateText)
	if err != nil {
		http.Error(w, "日付の形式が正しくありません。", http.StatusBadRequest)
		return
	}

	resp := CalendarWorkoutResponse{
		Exists: false,
		UserID: userID,
		Date:   day.Format("2006-01-02"),
		Title:  "筋トレ",
		Sets:   []CalendarWorkoutSet{},
	}

	err = app.db.QueryRow(`
		SELECT id, COALESCE(notes, '筋トレ')
		FROM workouts
		WHERE user_id = $1 AND DATE(started_at) = $2
		ORDER BY started_at DESC, id DESC
		LIMIT 1
	`, userID, day).Scan(&resp.WorkoutID, &resp.Title)
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		http.Error(w, "記録の取得に失敗しました。", http.StatusInternalServerError)
		return
	}
	resp.Exists = true
	resp.Sets, err = app.loadCalendarWorkoutSets(userID, resp.WorkoutID)
	if err != nil {
		http.Error(w, "セットの取得に失敗しました。", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (app *App) saveCalendarWorkout(w http.ResponseWriter, r *http.Request) {
	var req SaveCalendarWorkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	userID := req.UserID
	if userID <= 0 {
		userID = 1
	}
	day, err := time.Parse("2006-01-02", strings.TrimSpace(req.Date))
	if err != nil {
		http.Error(w, "日付の形式が正しくありません。", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "筋トレ"
	}
	if len(req.Sets) == 0 {
		http.Error(w, "少なくとも1セットは入力してください。", http.StatusBadRequest)
		return
	}

	exerciseIDs := make([]string, 0, len(req.Sets))
	for i := range req.Sets {
		req.Sets[i].ExerciseID = strings.TrimSpace(req.Sets[i].ExerciseID)
		req.Sets[i].Feeling = strings.TrimSpace(req.Sets[i].Feeling)
		if req.Sets[i].ExerciseID == "" {
			http.Error(w, "種目を選択してください。", http.StatusBadRequest)
			return
		}
		if req.Sets[i].Reps <= 0 {
			http.Error(w, "回数は1以上で入力してください。", http.StatusBadRequest)
			return
		}
		if req.Sets[i].Weight < 0 {
			http.Error(w, "重量は0以上で入力してください。", http.StatusBadRequest)
			return
		}
		if req.Sets[i].SetOrder <= 0 {
			req.Sets[i].SetOrder = i + 1
		}
		exerciseIDs = append(exerciseIDs, req.Sets[i].ExerciseID)
	}

	tx, err := app.db.Begin()
	if err != nil {
		http.Error(w, "保存に失敗しました。", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var workoutID int
	err = tx.QueryRow(`
		SELECT id
		FROM workouts
		WHERE user_id = $1 AND DATE(started_at) = $2
		ORDER BY started_at DESC, id DESC
		LIMIT 1
	`, userID, day).Scan(&workoutID)
	startedAt := day.Add(12 * time.Hour)
	endedAt := startedAt.Add(time.Duration(15+len(req.Sets)*3) * time.Minute)
	if err == sql.ErrNoRows {
		err = tx.QueryRow(`
			INSERT INTO workouts (user_id, started_at, ended_at, notes, summary_comment)
			VALUES ($1, $2, $3, $4, NULL)
			RETURNING id
		`, userID, startedAt, endedAt, title).Scan(&workoutID)
	} else if err == nil {
		_, err = tx.Exec(`
			UPDATE workouts
			SET started_at = $1, ended_at = $2, notes = $3, summary_comment = NULL
			WHERE id = $4 AND user_id = $5
		`, startedAt, endedAt, title, workoutID, userID)
	}
	if err != nil {
		http.Error(w, "ワークアウトの保存に失敗しました。", http.StatusInternalServerError)
		return
	}

	rows, err := tx.Query(`SELECT DISTINCT exercise_id FROM workout_sets WHERE workout_id = $1`, workoutID)
	if err != nil {
		http.Error(w, "既存セットの確認に失敗しました。", http.StatusInternalServerError)
		return
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil && id != "" {
			exerciseIDs = append(exerciseIDs, id)
		}
	}
	rows.Close()

	if _, err := tx.Exec(`DELETE FROM workout_sets WHERE workout_id = $1`, workoutID); err != nil {
		http.Error(w, "既存セットの更新に失敗しました。", http.StatusInternalServerError)
		return
	}

	for i, set := range req.Sets {
		var maxWeight float64
		_ = tx.QueryRow(`
			SELECT COALESCE(MAX(ws.weight), 0)
			FROM workout_sets ws
			JOIN workouts w ON w.id = ws.workout_id
			WHERE w.user_id = $1 AND ws.exercise_id = $2
		`, userID, set.ExerciseID).Scan(&maxWeight)
		isPR := set.Weight > maxWeight
		createdAt := startedAt.Add(time.Duration(i) * time.Minute)
		if _, err := tx.Exec(`
			INSERT INTO workout_sets (workout_id, exercise_id, weight, reps, set_order, feeling, is_pr, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, workoutID, set.ExerciseID, set.Weight, set.Reps, set.SetOrder, set.Feeling, isPR, createdAt); err != nil {
			http.Error(w, "セットの保存に失敗しました。", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "保存に失敗しました。", http.StatusInternalServerError)
		return
	}

	app.recalculateUserExerciseStats(userID, exerciseIDs)
	sets, err := app.loadCalendarWorkoutSets(userID, workoutID)
	if err != nil {
		http.Error(w, "保存後の記録取得に失敗しました。", http.StatusInternalServerError)
		return
	}
	resp := CalendarWorkoutResponse{
		Exists:    true,
		WorkoutID: workoutID,
		UserID:    userID,
		Date:      day.Format("2006-01-02"),
		Title:     title,
		Sets:      sets,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (app *App) loadCalendarWorkoutSets(userID, workoutID int) ([]CalendarWorkoutSet, error) {
	rows, err := app.db.Query(`
		SELECT
			ws.id,
			ws.exercise_id,
			COALESCE(e.name, ws.exercise_id),
			ws.weight,
			ws.reps,
			ws.set_order,
			COALESCE(ws.feeling, '')
		FROM workout_sets ws
		JOIN workouts w ON w.id = ws.workout_id
		LEFT JOIN exercises e ON e.id = ws.exercise_id
		WHERE w.user_id = $1 AND ws.workout_id = $2
		ORDER BY ws.created_at, ws.id
	`, userID, workoutID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sets := []CalendarWorkoutSet{}
	for rows.Next() {
		var set CalendarWorkoutSet
		if err := rows.Scan(&set.ID, &set.ExerciseID, &set.ExerciseName, &set.Weight, &set.Reps, &set.SetOrder, &set.Feeling); err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}
	return sets, rows.Err()
}

func (app *App) recalculateUserExerciseStats(userID int, exerciseIDs []string) {
	seen := map[string]bool{}
	for _, exerciseID := range exerciseIDs {
		exerciseID = strings.TrimSpace(exerciseID)
		if exerciseID == "" || seen[exerciseID] {
			continue
		}
		seen[exerciseID] = true

		var latestWeight, maxWeight float64
		err := app.db.QueryRow(`
			SELECT latest.weight, stats.max_weight
			FROM (
				SELECT ws.weight
				FROM workout_sets ws
				JOIN workouts w ON w.id = ws.workout_id
				WHERE w.user_id = $1 AND ws.exercise_id = $2
				ORDER BY w.started_at DESC, ws.created_at DESC, ws.id DESC
				LIMIT 1
			) latest
			CROSS JOIN (
				SELECT COALESCE(MAX(ws.weight), 0) AS max_weight
				FROM workout_sets ws
				JOIN workouts w ON w.id = ws.workout_id
				WHERE w.user_id = $1 AND ws.exercise_id = $2
			) stats
		`, userID, exerciseID).Scan(&latestWeight, &maxWeight)
		if err == sql.ErrNoRows {
			_, _ = app.db.Exec(`DELETE FROM user_exercise_stats WHERE user_id = $1 AND exercise_id = $2`, userID, exerciseID)
			continue
		}
		if err != nil {
			continue
		}
		_, _ = app.db.Exec(`
			INSERT INTO user_exercise_stats (user_id, exercise_id, weight, max_weight)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, exercise_id)
			DO UPDATE SET weight = EXCLUDED.weight, max_weight = EXCLUDED.max_weight, updated_at = CURRENT_TIMESTAMP
		`, userID, exerciseID, latestWeight, maxWeight)
	}
}

func displayWorkoutType(value string) string {
	switch value {
	case "Strength":
		return "筋トレ"
	case "Workout":
		return "ワークアウト"
	case "Flexibility":
		return "柔軟性"
	case "Morning Yoga":
		return "朝のストレッチ"
	case "Push (胸・肩・三頭)":
		return "押す日（胸・肩・三頭）"
	case "Pull (背中・二頭)":
		return "引く日（背中・二頭）"
	case "Legs (脚・腹)":
		return "脚の日（脚・腹）"
	default:
		return value
	}
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

	if app.db == nil {
		http.Error(w, "代替種目の取得に失敗しました。しばらくしてからやり直してください。", http.StatusServiceUnavailable)
		return
	}
	if req.ExerciseID == "" {
		id, name, err := app.resolveExerciseForAlternative(req.Exercise)
		if err != nil {
			fmt.Printf("Alternative exercise name lookup error: %v\n", err)
			http.Error(w, "種目を辞書から見つけられませんでした。手動で種目を変更してください。", http.StatusNotFound)
			return
		}
		req.ExerciseID = id
		if req.Exercise == "" {
			req.Exercise = name
		}
	}

	// 1. 元の種目の対象筋肉を取得
	var pMusclesJSON []byte
	if err := app.db.QueryRow("SELECT primary_muscles FROM exercises WHERE id = $1 LIMIT 1", req.ExerciseID).Scan(&pMusclesJSON); err != nil {
		fmt.Printf("Alternative muscle lookup error: %v\n", err)
		http.Error(w, "代替種目の候補を取得できませんでした。", http.StatusInternalServerError)
		return
	}

	var pMuscles []string
	if err := json.Unmarshal(pMusclesJSON, &pMuscles); err != nil {
		fmt.Printf("Alternative muscle parse error: %v\n", err)
		http.Error(w, "代替種目の候補を解析できませんでした。", http.StatusInternalServerError)
		return
	}
	if len(pMuscles) == 0 {
		http.Error(w, "代替種目の候補が見つかりませんでした。手動で種目を変更してください。", http.StatusNotFound)
		return
	}

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
	if err != nil {
		fmt.Printf("Alternative candidate query error: %v\n", err)
		http.Error(w, "代替種目の候補を取得できませんでした。", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var exList []string
	for rows.Next() {
		var idStr, nStr, eqStr string
		if err := rows.Scan(&idStr, &nStr, &eqStr); err != nil {
			fmt.Printf("Alternative candidate scan error: %v\n", err)
			http.Error(w, "代替種目の候補を読み込めませんでした。", http.StatusInternalServerError)
			return
		}
		exList = append(exList, fmt.Sprintf("- ID: %s, Name: %s (器具: %s)", idStr, nStr, eqStr))
	}
	if err := rows.Err(); err != nil {
		fmt.Printf("Alternative candidate rows error: %v\n", err)
		http.Error(w, "代替種目の候補を読み込めませんでした。", http.StatusInternalServerError)
		return
	}
	if len(exList) == 0 {
		http.Error(w, "代替種目の候補が見つかりませんでした。手動で種目を変更してください。", http.StatusNotFound)
		return
	}

	dbContext := "以下の種目が同じ筋肉( " + strings.Join(pMuscles, ", ") + " )を鍛えられるデータベース内の候補です:\n" + strings.Join(exList, "\n")

	systemPrompt, userPrompt, err := renderPromptPair("alternative_system.txt", "alternative_user.txt", map[string]any{
		"Exercise":  req.Exercise,
		"Reason":    req.Reason,
		"DBContext": dbContext,
	})
	if err != nil {
		fmt.Printf("Alternative prompt render error: %v\n", err)
		http.Error(w, "AIプロンプトの読み込みに失敗しました。", http.StatusInternalServerError)
		return
	}

	aiJSON, err := callAI(systemPrompt, userPrompt, true)
	if err != nil {
		fmt.Printf("AI Alternative Error: %v\n", err)
		http.Error(w, "AIによる代替種目の提案に失敗しました。しばらくしてからやり直してください。", http.StatusServiceUnavailable)
		return
	}

	aiStr := strings.TrimSpace(aiJSON)
	if strings.HasPrefix(aiStr, "```json") {
		aiStr = strings.TrimPrefix(aiStr, "```json")
		aiStr = strings.TrimSuffix(strings.TrimSpace(aiStr), "```")
	}
	if err := json.Unmarshal([]byte(aiStr), &resp); err != nil {
		fmt.Printf("AI Alternative parse error: %v. Raw: %s\n", err, aiStr)
		http.Error(w, "AIの返答の解析に失敗しました。もう一度お試しください。", http.StatusInternalServerError)
		return
	}
	if len(resp.Alternatives) == 0 {
		http.Error(w, "AIから代替種目を取得できませんでした。もう一度お試しください。", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (app *App) resolveExerciseForAlternative(name string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", fmt.Errorf("exercise name is empty")
	}

	var id string
	var displayName string
	err := app.db.QueryRow(`
		SELECT id, name FROM exercises
		WHERE name = $1 OR id = $1
		LIMIT 1
	`, name).Scan(&id, &displayName)
	if err == nil {
		return id, displayName, nil
	}

	err = app.db.QueryRow(`
		SELECT id, name FROM exercises
		WHERE name ILIKE $1 OR id ILIKE $1
		ORDER BY name
		LIMIT 1
	`, "%"+name+"%").Scan(&id, &displayName)
	if err != nil {
		return "", "", err
	}
	return id, displayName, nil
}
