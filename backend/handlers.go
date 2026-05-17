package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type RecommendRequest struct {
	UserID     int     `json:"user_id"`
	ExerciseID string  `json:"exercise_id"`
	SetOrder   int     `json:"set_order"`
	Weight     float64 `json:"weight"`
	Reps       int     `json:"reps"`
	Feeling    string  `json:"feeling"` // ユーザーの感想
}

type RecommendResponse struct {
	NextAction     string  `json:"next_action"`    // "CONTINUE", "ADJUST", "STOP"
	Recommendation string  `json:"recommendation"` // 具体的なアドバイス
	TargetWeight   float64 `json:"target_weight"`
	TargetReps     int     `json:"target_reps"`
	Reason         string  `json:"reason"`
	MaxWeight      float64 `json:"max_weight"` // 参考用の過去最大重量
}

type StartWorkoutPlanRequest struct {
	UserID int `json:"user_id"`
}

type FinishWorkoutRequest struct {
	UserID    int `json:"user_id"`
	WorkoutID int `json:"workout_id"`
}

type WorkoutPlanExercise struct {
	ExerciseID    string  `json:"exercise_id"`
	Name          string  `json:"name"`
	PlannedSets   int     `json:"planned_sets"`
	TargetWeight  float64 `json:"target_weight"`
	TargetReps    int     `json:"target_reps"`
	LastMaxWeight float64 `json:"last_max_weight"`
}

type WorkoutPlanPayload struct {
	WorkoutTitle         string                `json:"workout_title"`
	Target               string                `json:"target"`
	EstimatedDurationMin int                   `json:"estimated_duration_min"`
	CoachNote            string                `json:"coach_note"`
	Exercises            []WorkoutPlanExercise `json:"exercises"`
}

type WorkoutPlanSessionResponse struct {
	ID        int                `json:"id"`
	WorkoutID int                `json:"workout_id"`
	UserID    int                `json:"user_id"`
	PlanDate  string             `json:"plan_date"`
	Status    string             `json:"status"`
	Plan      WorkoutPlanPayload `json:"plan"`
}

type FinishWorkoutResponse struct {
	WorkoutID int            `json:"workout_id"`
	StartedAt string         `json:"started_at"`
	EndedAt   string         `json:"ended_at"`
	Status    string         `json:"status"`
	Summary   WorkoutSummary `json:"summary"`
}

type WorkoutDetailResponse struct {
	ID        int            `json:"id"`
	UserID    int            `json:"user_id"`
	Title     string         `json:"title"`
	StartedAt string         `json:"started_at"`
	EndedAt   string         `json:"ended_at"`
	Status    string         `json:"status"`
	Summary   WorkoutSummary `json:"summary"`
}

type WorkoutSummary struct {
	TotalSets   int                      `json:"total_sets"`
	TotalReps   int                      `json:"total_reps"`
	TotalVolume float64                  `json:"total_volume"`
	DurationMin int                      `json:"duration_min"`
	PRCount     int                      `json:"pr_count"`
	Exercises   []WorkoutSummaryExercise `json:"exercises"`
}

type WorkoutSummaryExercise struct {
	ExerciseID  string  `json:"exercise_id"`
	Name        string  `json:"name"`
	Sets        int     `json:"sets"`
	TotalReps   int     `json:"total_reps"`
	BestWeight  float64 `json:"best_weight"`
	TotalVolume float64 `json:"total_volume"`
}

func (app *App) handleRecommend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RecommendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// ここで本来はDBから過去の履歴を取得し、AI API (OpenAI等) を叩く
	// 今回はロジックの骨組みとして「いい感じの出力」をシミュレートします

	// --- AI API 連携のアドバイス ---
	// 良い出力を得るためのプロンプト例:
	// "以下のユーザーの履歴と今回のセット結果（重量、回数、感想）を分析してください。
	// また、この種目におけるユーザーの過去最大重量(MaxWeight: XX kg)も考慮し、
	// 次の1セットに最適な『重量(kg)』と『回数』を提案してください。
	// 例えば、現在の重量が過去最大に近く、かつ感想に余裕がある場合は重量更新を提案し、
	// 過去最大から遠い場合は、ボリューム（回数）を稼ぐアプローチを提案してください。
	// 理由もあわせて返答してください。"
	// -----------------------------

	// 1. 過去最大重量の取得
	var maxWeight float64
	if app.db != nil {
		app.db.QueryRow("SELECT COALESCE(max_weight, 0) FROM user_exercise_stats WHERE user_id = $1 AND exercise_id = $2", req.UserID, req.ExerciseID).Scan(&maxWeight)
	}

	// 初回利用時のダミー値
	if maxWeight == 0 {
		maxWeight = 10.0 // 何もない場合の基準
	}

	// 2. 本物のDBへの保存処理
	if app.db != nil {
		var workoutID int
		// 今日のワークアウトを探す
		err := app.db.QueryRow(`
			SELECT id FROM workouts 
			WHERE user_id = $1 AND ended_at IS NULL AND DATE(started_at) = CURRENT_DATE 
			LIMIT 1`, req.UserID).Scan(&workoutID)

		if err != nil {
			// 新しく作成
			err = app.db.QueryRow(`
				INSERT INTO workouts (user_id) VALUES ($1) RETURNING id
			`, req.UserID).Scan(&workoutID)
			if err != nil {
				log.Printf("Failed to create workout: %v", err)
			}
		}

		if workoutID > 0 {
			isPR := req.Weight > maxWeight

			// workout_sets に挿入
			_, err = app.db.Exec(`
				INSERT INTO workout_sets (workout_id, exercise_id, weight, reps, set_order, feeling, is_pr)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, workoutID, req.ExerciseID, req.Weight, req.Reps, req.SetOrder, req.Feeling, isPR)
			if err != nil {
				log.Printf("Failed to insert workout_set: %v", err)
			}

			// user_exercise_stats を更新
			_, err = app.db.Exec(`
				INSERT INTO user_exercise_stats (user_id, exercise_id, weight, max_weight)
				VALUES ($1, $2, $3, GREATEST($3, COALESCE((SELECT max_weight FROM user_exercise_stats WHERE user_id = $1 AND exercise_id = $2), 0)))
				ON CONFLICT (user_id, exercise_id) 
				DO UPDATE SET weight = EXCLUDED.weight, max_weight = GREATEST(user_exercise_stats.max_weight, EXCLUDED.weight), updated_at = CURRENT_TIMESTAMP
			`, req.UserID, req.ExerciseID, req.Weight)
			if err != nil {
				log.Printf("Failed to upsert user_exercise_stats: %v", err)
			}
		}
	}
	// 今回の入力が仮に最高重量を上回っていた場合、maxWeightを更新しておく（シミュレーション用ではなく実際のデータとして使う）
	if req.Weight > maxWeight {
		maxWeight = req.Weight
	}

	systemPrompt := `あなたはエリートパーソナルトレーナーAIです。ユーザーの直近の筋トレセットの結果を分析し、次のセットの提案をJSON形式でのみ出力してください。
以下のJSON構造に必ず従ってください（マークダウンブロックを含めず、JSONのみを返してください）:
{
	"next_action": "CONTINUE" または "STOP", // 怪我の恐れがある「痛み」や「違和感」、または過度の疲労がある場合は "STOP"
	"recommendation": "string", // 短く、励ましと具体的なアドバイス
	"target_weight": 80.0, // 次のセットの推奨重量（数値のみ）
	"target_reps": 10, // 次のセットの推奨回数（数値のみ）
	"reason": "string" // なぜこの重量・回数を提案するかの理由
}`

	userPrompt := fmt.Sprintf(`現在の入力データ:
- 今回のセット数: %d
- 今回扱った重量: %.1f kg
- 今回こなした回数: %d
- ユーザーの感想・状態: "%s"
- (参考) この種目の過去最大重量: %.1f kg

上記に基づき、次のセットはどうすべきかJSONで提案してください。`, req.SetOrder, req.Weight, req.Reps, req.Feeling, maxWeight)

	aiJSON, err := callAI(systemPrompt, userPrompt, true)
	if err != nil {
		log.Printf("AI API Error (Recommend): %v\n", err)
		http.Error(w, "AIの呼び出しに失敗しました。しばらくしてからやり直してください。", http.StatusServiceUnavailable)
		return
	}

	// AIレスポンスのパース（マークダウンブロックを除去）
	aiStr := strings.TrimSpace(aiJSON)
	if strings.HasPrefix(aiStr, "```json") {
		aiStr = strings.TrimPrefix(aiStr, "```json")
		aiStr = strings.TrimSuffix(strings.TrimSpace(aiStr), "```")
	}

	var resp RecommendResponse
	if err := json.Unmarshal([]byte(aiStr), &resp); err != nil {
		log.Printf("AI JSON Parse Error: %v. Raw: %s\n", err, aiStr)
		http.Error(w, "AIの返答の解析に失敗しました。もう一度お試しください。", http.StatusInternalServerError)
		return
	}
	resp.MaxWeight = maxWeight

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (app *App) handleStartWorkoutPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartWorkoutPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	userID := req.UserID
	if userID == 0 {
		userID = 1
	}

	if err := app.closeStaleWorkouts(userID); err != nil {
		log.Printf("Failed to close stale workouts: %v", err)
	}

	if plan, ok, err := app.loadActiveWorkoutPlan(userID); err != nil {
		log.Printf("Failed to load active workout plan: %v", err)
		http.Error(w, "Failed to load active workout plan", http.StatusInternalServerError)
		return
	} else if ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(plan)
		return
	}

	payload, err := app.buildTodayWorkoutPlan(userID)
	if err != nil {
		log.Printf("Failed to build workout plan: %v", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	workoutID, err := app.getOrCreateTodayWorkout(userID)
	if err != nil {
		log.Printf("Failed to start workout: %v", err)
		http.Error(w, "Failed to start workout", http.StatusInternalServerError)
		return
	}

	planJSON, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Failed to serialize workout plan", http.StatusInternalServerError)
		return
	}

	var resp WorkoutPlanSessionResponse
	err = app.db.QueryRow(`
		INSERT INTO workout_plans (workout_id, user_id, plan_date, title, estimated_duration_min, status, plan)
		VALUES ($1, $2, CURRENT_DATE, $3, $4, 'active', $5::jsonb)
		ON CONFLICT (user_id, plan_date, status) DO UPDATE SET
			workout_id = EXCLUDED.workout_id,
			title = EXCLUDED.title,
			estimated_duration_min = EXCLUDED.estimated_duration_min,
			plan = EXCLUDED.plan,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, workout_id, user_id, plan_date::text, status, plan
	`, workoutID, userID, payload.WorkoutTitle, payload.EstimatedDurationMin, string(planJSON)).Scan(
		&resp.ID,
		&resp.WorkoutID,
		&resp.UserID,
		&resp.PlanDate,
		&resp.Status,
		&planJSON,
	)
	if err != nil {
		log.Printf("Failed to save workout plan: %v", err)
		http.Error(w, "Failed to save workout plan", http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal(planJSON, &resp.Plan); err != nil {
		http.Error(w, "Failed to parse saved workout plan", http.StatusInternalServerError)
		return
	}

	_, _ = app.db.Exec("UPDATE workouts SET notes = $1 WHERE id = $2", payload.WorkoutTitle, workoutID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (app *App) handleFinishWorkout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FinishWorkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	userID := req.UserID
	if userID == 0 {
		userID = 1
	}

	var resp FinishWorkoutResponse
	var err error
	if req.WorkoutID > 0 {
		err = app.db.QueryRow(`
			UPDATE workouts
			SET ended_at = COALESCE(
				(SELECT MAX(created_at) FROM workout_sets WHERE workout_id = workouts.id),
				CURRENT_TIMESTAMP
			)
			WHERE id = $1 AND user_id = $2 AND ended_at IS NULL
			RETURNING id, started_at::text, ended_at::text
		`, req.WorkoutID, userID).Scan(&resp.WorkoutID, &resp.StartedAt, &resp.EndedAt)
	} else {
		err = app.db.QueryRow(`
			UPDATE workouts
			SET ended_at = COALESCE(
				(SELECT MAX(created_at) FROM workout_sets WHERE workout_id = workouts.id),
				CURRENT_TIMESTAMP
			)
			WHERE id = (
				SELECT id FROM workouts
				WHERE user_id = $1 AND ended_at IS NULL
				ORDER BY started_at DESC
				LIMIT 1
			)
			RETURNING id, started_at::text, ended_at::text
		`, userID).Scan(&resp.WorkoutID, &resp.StartedAt, &resp.EndedAt)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Active workout not found", http.StatusNotFound)
			return
		}
		log.Printf("Failed to finish workout: %v", err)
		http.Error(w, "Failed to finish workout", http.StatusInternalServerError)
		return
	}

	_, _ = app.db.Exec(`
		UPDATE workout_plans
		SET status = 'completed', updated_at = CURRENT_TIMESTAMP
		WHERE workout_id = $1 AND user_id = $2 AND status = 'active'
	`, resp.WorkoutID, userID)
	resp.Status = "completed"
	summary, err := app.loadWorkoutSummary(userID, resp.WorkoutID)
	if err != nil {
		log.Printf("Failed to load workout summary: %v", err)
		http.Error(w, "Failed to load workout summary", http.StatusInternalServerError)
		return
	}
	resp.Summary = summary

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (app *App) handleWorkoutDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idText := strings.TrimPrefix(r.URL.Path, "/api/workouts/")
	workoutID, err := strconv.Atoi(strings.Trim(idText, "/"))
	if err != nil || workoutID <= 0 {
		http.Error(w, "Invalid workout id", http.StatusBadRequest)
		return
	}

	userID, _ := strconv.Atoi(r.URL.Query().Get("user_id"))
	if userID == 0 {
		userID = 1
	}
	if err := app.closeStaleWorkouts(userID); err != nil {
		log.Printf("Failed to close stale workouts: %v", err)
	}

	var resp WorkoutDetailResponse
	err = app.db.QueryRow(`
		SELECT
			id,
			user_id,
			COALESCE(notes, 'ワークアウト'),
			started_at::text,
			COALESCE(ended_at::text, ''),
			CASE WHEN ended_at IS NULL THEN 'active' ELSE 'completed' END
		FROM workouts
		WHERE id = $1 AND user_id = $2
	`, workoutID, userID).Scan(
		&resp.ID,
		&resp.UserID,
		&resp.Title,
		&resp.StartedAt,
		&resp.EndedAt,
		&resp.Status,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Workout not found", http.StatusNotFound)
			return
		}
		log.Printf("Failed to load workout detail: %v", err)
		http.Error(w, "Failed to load workout detail", http.StatusInternalServerError)
		return
	}

	summary, err := app.loadWorkoutSummary(userID, workoutID)
	if err != nil {
		log.Printf("Failed to load workout summary: %v", err)
		http.Error(w, "Failed to load workout summary", http.StatusInternalServerError)
		return
	}
	resp.Title = displayWorkoutTitle(resp.Title)
	resp.Summary = summary

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (app *App) loadWorkoutSummary(userID, workoutID int) (WorkoutSummary, error) {
	summary := WorkoutSummary{Exercises: []WorkoutSummaryExercise{}}
	err := app.db.QueryRow(`
		SELECT GREATEST(1, CEIL(EXTRACT(EPOCH FROM (COALESCE(ended_at, CURRENT_TIMESTAMP) - started_at)) / 60))::int
		FROM workouts
		WHERE id = $1 AND user_id = $2
	`, workoutID, userID).Scan(&summary.DurationMin)
	if err != nil {
		return summary, err
	}

	rows, err := app.db.Query(`
		SELECT
			ws.exercise_id,
			COALESCE(e.name, ws.exercise_id) AS exercise_name,
			COUNT(*)::int AS sets,
			COALESCE(SUM(ws.reps), 0)::int AS total_reps,
			COALESCE(MAX(ws.weight), 0) AS best_weight,
			COALESCE(SUM(ws.weight * ws.reps), 0) AS total_volume,
			COALESCE(SUM(CASE WHEN ws.is_pr THEN 1 ELSE 0 END), 0)::int AS pr_count
		FROM workout_sets ws
		LEFT JOIN exercises e ON e.id = ws.exercise_id
		WHERE ws.workout_id = $1
		GROUP BY ws.exercise_id, e.name
		ORDER BY MIN(ws.created_at), MIN(ws.id)
	`, workoutID)
	if err != nil {
		return summary, err
	}
	defer rows.Close()

	for rows.Next() {
		var exercise WorkoutSummaryExercise
		var prCount int
		if err := rows.Scan(
			&exercise.ExerciseID,
			&exercise.Name,
			&exercise.Sets,
			&exercise.TotalReps,
			&exercise.BestWeight,
			&exercise.TotalVolume,
			&prCount,
		); err != nil {
			return summary, err
		}
		summary.TotalSets += exercise.Sets
		summary.TotalReps += exercise.TotalReps
		summary.TotalVolume += exercise.TotalVolume
		summary.PRCount += prCount
		summary.Exercises = append(summary.Exercises, exercise)
	}
	if err := rows.Err(); err != nil {
		return summary, err
	}
	return summary, nil
}

func displayWorkoutTitle(value string) string {
	switch value {
	case "Strength":
		return "筋トレ"
	case "Workout":
		return "ワークアウト"
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

func (app *App) closeStaleWorkouts(userID int) error {
	_, err := app.db.Exec(`
		UPDATE workouts
		SET ended_at = COALESCE(
			(SELECT MAX(created_at) FROM workout_sets WHERE workout_id = workouts.id),
			started_at
		)
		WHERE user_id = $1 AND ended_at IS NULL AND DATE(started_at) < CURRENT_DATE
	`, userID)
	if err != nil {
		return err
	}
	_, err = app.db.Exec(`
		UPDATE workout_plans
		SET status = 'abandoned', updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND status = 'active' AND plan_date < CURRENT_DATE
	`, userID)
	return err
}

func (app *App) getOrCreateTodayWorkout(userID int) (int, error) {
	var workoutID int
	err := app.db.QueryRow(`
		SELECT id FROM workouts
		WHERE user_id = $1 AND ended_at IS NULL AND DATE(started_at) = CURRENT_DATE
		ORDER BY started_at DESC
		LIMIT 1
	`, userID).Scan(&workoutID)
	if err == nil {
		return workoutID, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	err = app.db.QueryRow("INSERT INTO workouts (user_id) VALUES ($1) RETURNING id", userID).Scan(&workoutID)
	return workoutID, err
}

func (app *App) loadActiveWorkoutPlan(userID int) (WorkoutPlanSessionResponse, bool, error) {
	var resp WorkoutPlanSessionResponse
	var planJSON []byte
	err := app.db.QueryRow(`
		SELECT id, workout_id, user_id, plan_date::text, status, plan
		FROM workout_plans
		WHERE user_id = $1 AND plan_date = CURRENT_DATE AND status = 'active'
		ORDER BY updated_at DESC
		LIMIT 1
	`, userID).Scan(&resp.ID, &resp.WorkoutID, &resp.UserID, &resp.PlanDate, &resp.Status, &planJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return WorkoutPlanSessionResponse{}, false, nil
		}
		return WorkoutPlanSessionResponse{}, false, err
	}
	if err := json.Unmarshal(planJSON, &resp.Plan); err != nil {
		return WorkoutPlanSessionResponse{}, false, err
	}
	return resp, true, nil
}

func (app *App) buildTodayWorkoutPlan(userID int) (WorkoutPlanPayload, error) {
	today := time.Now()
	monthly, ok, err := app.loadMonthlyPlan(userID, today.Format("2006-01"))
	if err != nil {
		return WorkoutPlanPayload{}, err
	}
	if !ok || len(monthly.WeeklyRoutine) == 0 {
		return WorkoutPlanPayload{}, fmt.Errorf("今月の月間プランがまだありません。先にホームで月間プランを作成してください。")
	}

	weekday := int(today.Weekday())
	idx := -1
	for i, d := range monthly.RecommendedDays {
		if d == weekday {
			idx = i
			break
		}
	}
	routine := monthly.WeeklyRoutine[0]
	coachNote := "今日は月間プラン上は休息日ですが、実施する場合に備えてAIが軽めに調整します。"
	if idx >= 0 && idx < len(monthly.WeeklyRoutine) {
		routine = monthly.WeeklyRoutine[idx]
		coachNote = "月間プランの今日のメニューをもとに、直近の重量と目標セット数を反映します。"
	}

	exercises := []WorkoutPlanExercise{}
	for _, name := range routine.ExampleExercises {
		exerciseID, displayName, err := app.findExerciseByName(name)
		if err != nil {
			log.Printf("Exercise lookup failed for %q: %v", name, err)
			continue
		}
		stats := app.loadExercisePlanStats(userID, exerciseID)
		exercises = append(exercises, WorkoutPlanExercise{
			ExerciseID:    exerciseID,
			Name:          displayName,
			PlannedSets:   stats.TargetSets,
			TargetWeight:  stats.TargetWeight,
			TargetReps:    stats.TargetReps,
			LastMaxWeight: stats.MaxWeight,
		})
	}
	if len(exercises) == 0 {
		return WorkoutPlanPayload{}, fmt.Errorf("月間プラン内の種目を辞書から見つけられませんでした。")
	}

	estimated := len(exercises) * 12
	if estimated < 30 {
		estimated = 30
	}
	payload := WorkoutPlanPayload{
		WorkoutTitle:         routine.Target,
		Target:               routine.Target,
		EstimatedDurationMin: estimated,
		CoachNote:            coachNote,
		Exercises:            exercises,
	}
	return refineWorkoutPlanWithAI(payload)
}

func refineWorkoutPlanWithAI(base WorkoutPlanPayload) (WorkoutPlanPayload, error) {
	baseJSON, err := json.Marshal(base)
	if err != nil {
		return WorkoutPlanPayload{}, err
	}

	systemPrompt := `あなたは安全重視の筋トレAIコーチです。
ユーザーの今日のワークアウト計画をJSONで微調整してください。
以下の制約を守ってください:
- 返答はJSONのみ
- exercise_id と name は入力された候補から変更しない
- planned_sets は1〜6の整数
- target_weight と target_reps は無理のない範囲にする
- 疲労や痛みが出たら短縮できる前提の coach_note にする`

	userPrompt := fmt.Sprintf(`以下のベース計画を、今日実施しやすいワークアウト計画に調整してください。
同じJSON構造で返してください。

%s`, string(baseJSON))

	aiJSON, err := callAI(systemPrompt, userPrompt, true)
	if err != nil {
		return WorkoutPlanPayload{}, fmt.Errorf("AIによる今日の計画作成に失敗しました。APIキーや接続状況を確認してください。")
	}

	aiStr := strings.TrimSpace(aiJSON)
	if strings.HasPrefix(aiStr, "```json") {
		aiStr = strings.TrimPrefix(aiStr, "```json")
		aiStr = strings.TrimSuffix(strings.TrimSpace(aiStr), "```")
	}

	var refined WorkoutPlanPayload
	if err := json.Unmarshal([]byte(aiStr), &refined); err != nil {
		return WorkoutPlanPayload{}, fmt.Errorf("AIの計画レスポンスを解析できませんでした。もう一度お試しください。")
	}
	if len(refined.Exercises) != len(base.Exercises) || refined.WorkoutTitle == "" {
		return WorkoutPlanPayload{}, fmt.Errorf("AIの計画レスポンスが不正でした。もう一度お試しください。")
	}
	for i := range refined.Exercises {
		refined.Exercises[i].ExerciseID = base.Exercises[i].ExerciseID
		refined.Exercises[i].Name = base.Exercises[i].Name
		refined.Exercises[i].LastMaxWeight = base.Exercises[i].LastMaxWeight
		if refined.Exercises[i].PlannedSets < 1 {
			refined.Exercises[i].PlannedSets = base.Exercises[i].PlannedSets
		}
		if refined.Exercises[i].PlannedSets > 6 {
			refined.Exercises[i].PlannedSets = 6
		}
		if refined.Exercises[i].TargetWeight <= 0 {
			refined.Exercises[i].TargetWeight = base.Exercises[i].TargetWeight
		}
		if refined.Exercises[i].TargetReps <= 0 {
			refined.Exercises[i].TargetReps = base.Exercises[i].TargetReps
		}
	}
	if refined.EstimatedDurationMin <= 0 {
		refined.EstimatedDurationMin = base.EstimatedDurationMin
	}
	if refined.CoachNote == "" {
		refined.CoachNote = base.CoachNote
	}
	return refined, nil
}

type exercisePlanStats struct {
	TargetSets   int
	TargetWeight float64
	TargetReps   int
	MaxWeight    float64
}

func (app *App) loadExercisePlanStats(userID int, exerciseID string) exercisePlanStats {
	stats := exercisePlanStats{TargetSets: 3, TargetWeight: 20, TargetReps: 10}
	_ = app.db.QueryRow(`
		SELECT COALESCE(weight, 0), COALESCE(max_weight, 0), COALESCE(target_sets, 3)
		FROM user_exercise_stats
		WHERE user_id = $1 AND exercise_id = $2
	`, userID, exerciseID).Scan(&stats.TargetWeight, &stats.MaxWeight, &stats.TargetSets)
	if stats.TargetWeight == 0 && stats.MaxWeight > 0 {
		stats.TargetWeight = stats.MaxWeight * 0.8
	}
	if stats.TargetWeight == 0 {
		stats.TargetWeight = 20
	}
	return stats
}

func (app *App) findExerciseByName(name string) (string, string, error) {
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
	if err == nil {
		return id, displayName, nil
	}
	err = app.db.QueryRow(`
		SELECT id, name FROM exercises
		WHERE id = 'Barbell_Bench_Press_-_Medium_Grip'
		LIMIT 1
	`).Scan(&id, &displayName)
	return id, displayName, err
}

type MonthlyPlanRequest struct {
	UserID     int    `json:"user_id"`
	PlanMonth  string `json:"plan_month"`
	Motivation string `json:"motivation"`
	Frequency  string `json:"frequency"`
}

type DayRoutine struct {
	DayName          string   `json:"day_name"`
	Target           string   `json:"target"`
	ExampleExercises []string `json:"example_exercises"`
}

type MonthlyPlanResponse struct {
	ID              int          `json:"id,omitempty"`
	UserID          int          `json:"user_id,omitempty"`
	PlanMonth       string       `json:"plan_month,omitempty"`
	PlanName        string       `json:"plan_name"`
	Frequency       string       `json:"frequency"`
	Description     string       `json:"description"`
	Rationale       string       `json:"rationale"`
	RecommendedDays []int        `json:"recommended_days"`
	WeeklyRoutine   []DayRoutine `json:"weekly_routine"`
}

func (app *App) handleMonthlyPlan(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		app.getMonthlyPlan(w, r)
	case http.MethodPost:
		app.createMonthlyPlan(w, r)
	case http.MethodPut:
		app.saveMonthlyPlan(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (app *App) handleMonthlyPlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := parseUserID(r.URL.Query().Get("user_id"))
	plans, err := app.loadMonthlyPlans(userID)
	if err != nil {
		log.Printf("Failed to load monthly plans: %v", err)
		http.Error(w, "Failed to load monthly plans", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plans)
}

func (app *App) getMonthlyPlan(w http.ResponseWriter, r *http.Request) {
	userID := parseUserID(r.URL.Query().Get("user_id"))
	planMonth := normalizePlanMonth(r.URL.Query().Get("month"))

	plan, ok, err := app.loadMonthlyPlan(userID, planMonth)
	if err != nil {
		log.Printf("Failed to load monthly plan: %v", err)
		http.Error(w, "Failed to load monthly plan", http.StatusInternalServerError)
		return
	}
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "monthly plan not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

func (app *App) createMonthlyPlan(w http.ResponseWriter, r *http.Request) {
	var req MonthlyPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	userID := req.UserID
	if userID == 0 {
		userID = 1
	}
	planMonth := normalizePlanMonth(req.PlanMonth)
	motivation := req.Motivation
	frequency := req.Frequency
	resp := generateMonthlyPlan(motivation, frequency)
	resp.UserID = userID
	resp.PlanMonth = planMonth

	if err := app.upsertMonthlyPlan(userID, planMonth, &resp); err != nil {
		log.Printf("Failed to save monthly plan: %v", err)
		http.Error(w, "Failed to save monthly plan", http.StatusInternalServerError)
		return
	}

	plan, ok, err := app.loadMonthlyPlan(userID, planMonth)
	if err != nil || !ok {
		log.Printf("Failed to reload monthly plan: ok=%v err=%v", ok, err)
		http.Error(w, "Failed to load saved monthly plan", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

func (app *App) saveMonthlyPlan(w http.ResponseWriter, r *http.Request) {
	var req MonthlyPlanResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	userID := req.UserID
	if userID == 0 {
		userID = 1
	}
	planMonth := normalizePlanMonth(req.PlanMonth)
	req.UserID = userID
	req.PlanMonth = planMonth

	if req.PlanName == "" || len(req.WeeklyRoutine) == 0 {
		http.Error(w, "Invalid monthly plan", http.StatusBadRequest)
		return
	}

	if err := app.upsertMonthlyPlan(userID, planMonth, &req); err != nil {
		log.Printf("Failed to update monthly plan: %v", err)
		http.Error(w, "Failed to update monthly plan", http.StatusInternalServerError)
		return
	}

	plan, ok, err := app.loadMonthlyPlan(userID, planMonth)
	if err != nil || !ok {
		log.Printf("Failed to reload monthly plan: ok=%v err=%v", ok, err)
		http.Error(w, "Failed to load saved monthly plan", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

func generateMonthlyPlan(motivation, frequency string) MonthlyPlanResponse {
	var resp MonthlyPlanResponse
	if strings.Contains(frequency, "週5") || strings.Contains(motivation, "本気") {
		resp.PlanName = "ブロ割 (部位別特化)"
		resp.Frequency = "週5〜6回"
		resp.Description = "各筋肉群を日ごとに徹底的に追い込む、高頻度・高強度のルーティンです。"
		resp.Rationale = "非常に高いモチベーションと頻度を考慮し、1回あたりの部位を絞って限界まで追い込める分割法を選択しました。"
		resp.RecommendedDays = []int{1, 2, 3, 5, 6}
		resp.WeeklyRoutine = []DayRoutine{
			{DayName: "1日目", Target: "胸・腹", ExampleExercises: []string{"ベンチプレス", "ダンベルフライ", "クランチ"}},
			{DayName: "2日目", Target: "背中", ExampleExercises: []string{"デッドリフト", "懸垂", "ラットプルダウン"}},
			{DayName: "3日目", Target: "脚", ExampleExercises: []string{"スクワット", "レッグプレス", "カーフレイズ"}},
			{DayName: "4日目", Target: "肩", ExampleExercises: []string{"ショルダープレス", "サイドレイズ", "リアレイズ"}},
			{DayName: "5日目", Target: "腕", ExampleExercises: []string{"バーベルカール", "トライセプスエクステンション", "ハンマーカール"}},
		}
	} else if strings.Contains(frequency, "週1") || strings.Contains(frequency, "週2") || strings.Contains(motivation, "健康維持") || strings.Contains(motivation, "無理なく") {
		resp.PlanName = "全身法"
		resp.Frequency = "週1〜2回"
		resp.Description = "少ない日数でも全身の主要な筋肉を同時に鍛えられる、タイムパフォーマンスに優れたルーティンです。"
		resp.Rationale = "多忙なスケジュールや無理のないペースを考慮し、少ない回数で全身を刺激して筋力低下を防ぎ、少しずつ成長できる全身法を選択しました。"
		resp.RecommendedDays = []int{2, 5}
		resp.WeeklyRoutine = []DayRoutine{
			{DayName: "1日目", Target: "全身その1", ExampleExercises: []string{"スクワット", "ベンチプレス", "ベントオーバーロウ"}},
			{DayName: "2日目", Target: "全身その2", ExampleExercises: []string{"デッドリフト", "ショルダープレス", "懸垂"}},
		}
	} else {
		resp.PlanName = "PPL法（押す・引く・脚）"
		resp.Frequency = "週3〜4回"
		resp.Description = "押す筋肉、引く筋肉、脚の3グループに分けて鍛える、最もバランスが良く結果が出やすい王道のルーティンです。"
		resp.Rationale = "バランス良く全身を鍛えつつ、各部位に十分な回復期間を与えられるPPL法が最も適していると判断しました。"
		resp.RecommendedDays = []int{1, 3, 5}
		resp.WeeklyRoutine = []DayRoutine{
			{DayName: "1日目", Target: "押す日（胸・肩・三頭）", ExampleExercises: []string{"ベンチプレス", "ショルダープレス", "ディップス"}},
			{DayName: "2日目", Target: "引く日（背中・二頭）", ExampleExercises: []string{"懸垂", "デッドリフト", "バーベルカール"}},
			{DayName: "3日目", Target: "脚の日（脚・腹）", ExampleExercises: []string{"スクワット", "レッグプレス", "カーフレイズ"}},
		}
	}
	return resp
}

func (app *App) loadMonthlyPlan(userID int, planMonth string) (MonthlyPlanResponse, bool, error) {
	var plan MonthlyPlanResponse
	var recommendedDaysJSON []byte
	var weeklyRoutineJSON []byte

	err := app.db.QueryRow(`
		SELECT id, user_id, plan_month, plan_name, frequency, COALESCE(description, ''), COALESCE(rationale, ''), recommended_days, weekly_routine
		FROM monthly_plans
		WHERE user_id = $1 AND plan_month = $2
	`, userID, planMonth).Scan(
		&plan.ID,
		&plan.UserID,
		&plan.PlanMonth,
		&plan.PlanName,
		&plan.Frequency,
		&plan.Description,
		&plan.Rationale,
		&recommendedDaysJSON,
		&weeklyRoutineJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return MonthlyPlanResponse{}, false, nil
		}
		return MonthlyPlanResponse{}, false, err
	}

	if err := json.Unmarshal(recommendedDaysJSON, &plan.RecommendedDays); err != nil {
		return MonthlyPlanResponse{}, false, err
	}
	if err := json.Unmarshal(weeklyRoutineJSON, &plan.WeeklyRoutine); err != nil {
		return MonthlyPlanResponse{}, false, err
	}
	return plan, true, nil
}

func (app *App) loadMonthlyPlans(userID int) ([]MonthlyPlanResponse, error) {
	rows, err := app.db.Query(`
		SELECT id, user_id, plan_month, plan_name, frequency, COALESCE(description, ''), COALESCE(rationale, ''), recommended_days, weekly_routine
		FROM monthly_plans
		WHERE user_id = $1
		ORDER BY plan_month DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	plans := []MonthlyPlanResponse{}
	for rows.Next() {
		var plan MonthlyPlanResponse
		var recommendedDaysJSON []byte
		var weeklyRoutineJSON []byte
		if err := rows.Scan(
			&plan.ID,
			&plan.UserID,
			&plan.PlanMonth,
			&plan.PlanName,
			&plan.Frequency,
			&plan.Description,
			&plan.Rationale,
			&recommendedDaysJSON,
			&weeklyRoutineJSON,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(recommendedDaysJSON, &plan.RecommendedDays); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(weeklyRoutineJSON, &plan.WeeklyRoutine); err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, rows.Err()
}

func (app *App) upsertMonthlyPlan(userID int, planMonth string, plan *MonthlyPlanResponse) error {
	recommendedDaysJSON, err := json.Marshal(plan.RecommendedDays)
	if err != nil {
		return err
	}
	weeklyRoutineJSON, err := json.Marshal(plan.WeeklyRoutine)
	if err != nil {
		return err
	}

	return app.db.QueryRow(`
		INSERT INTO monthly_plans (
			user_id, plan_month, plan_name, frequency, description, rationale, recommended_days, weekly_routine
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb)
		ON CONFLICT (user_id, plan_month) DO UPDATE SET
			plan_name = EXCLUDED.plan_name,
			frequency = EXCLUDED.frequency,
			description = EXCLUDED.description,
			rationale = EXCLUDED.rationale,
			recommended_days = EXCLUDED.recommended_days,
			weekly_routine = EXCLUDED.weekly_routine,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`, userID, planMonth, plan.PlanName, plan.Frequency, plan.Description, plan.Rationale, string(recommendedDaysJSON), string(weeklyRoutineJSON)).Scan(&plan.ID)
}

func parseUserID(raw string) int {
	userID, err := strconv.Atoi(raw)
	if err != nil || userID == 0 {
		return 1
	}
	return userID
}

func normalizePlanMonth(raw string) string {
	if raw == "" {
		return time.Now().Format("2006-01")
	}
	if _, err := time.Parse("2006-01", raw); err != nil {
		return time.Now().Format("2006-01")
	}
	return raw
}
