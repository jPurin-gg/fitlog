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
			{DayName: "Day 1", Target: "胸・腹", ExampleExercises: []string{"ベンチプレス", "ダンベルフライ", "クランチ"}},
			{DayName: "Day 2", Target: "背中", ExampleExercises: []string{"デッドリフト", "懸垂", "ラットプルダウン"}},
			{DayName: "Day 3", Target: "脚", ExampleExercises: []string{"スクワット", "レッグプレス", "カーフレイズ"}},
			{DayName: "Day 4", Target: "肩", ExampleExercises: []string{"ショルダープレス", "サイドレイズ", "リアレイズ"}},
			{DayName: "Day 5", Target: "腕", ExampleExercises: []string{"バーベルカール", "トライセプスエクステンション", "ハンマーカール"}},
		}
	} else if strings.Contains(frequency, "週1") || strings.Contains(frequency, "週2") || strings.Contains(motivation, "健康維持") || strings.Contains(motivation, "無理なく") {
		resp.PlanName = "全身法 (Full Body)"
		resp.Frequency = "週1〜2回"
		resp.Description = "少ない日数でも全身の主要な筋肉を同時に鍛えられる、タイムパフォーマンスに優れたルーティンです。"
		resp.Rationale = "多忙なスケジュールや無理のないペースを考慮し、少ない回数で全身を刺激して筋力低下を防ぎ、少しずつ成長できる全身法を選択しました。"
		resp.RecommendedDays = []int{2, 5}
		resp.WeeklyRoutine = []DayRoutine{
			{DayName: "Day 1", Target: "全身 A", ExampleExercises: []string{"スクワット", "ベンチプレス", "ベントオーバーロウ"}},
			{DayName: "Day 2", Target: "全身 B", ExampleExercises: []string{"デッドリフト", "ショルダープレス", "懸垂"}},
		}
	} else {
		resp.PlanName = "PPL法 (Push/Pull/Legs)"
		resp.Frequency = "週3〜4回"
		resp.Description = "押す筋肉、引く筋肉、脚の3グループに分けて鍛える、最もバランスが良く結果が出やすい王道のルーティンです。"
		resp.Rationale = "バランス良く全身を鍛えつつ、各部位に十分な回復期間を与えられるPPL法が最も適していると判断しました。"
		resp.RecommendedDays = []int{1, 3, 5}
		resp.WeeklyRoutine = []DayRoutine{
			{DayName: "Day 1", Target: "Push (胸・肩・三頭)", ExampleExercises: []string{"ベンチプレス", "ショルダープレス", "ディップス"}},
			{DayName: "Day 2", Target: "Pull (背中・二頭)", ExampleExercises: []string{"懸垂", "デッドリフト", "バーベルカール"}},
			{DayName: "Day 3", Target: "Legs (脚・腹)", ExampleExercises: []string{"スクワット", "レッグプレス", "カーフレイズ"}},
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
