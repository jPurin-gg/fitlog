package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type RecommendRequest struct {
	UserID     int     `json:"user_id"`
	ExerciseID int     `json:"exercise_id"`
	SetOrder   int     `json:"set_order"`
	Weight     float64 `json:"weight"`
	Reps       int     `json:"reps"`
	Feeling    string  `json:"feeling"` // ユーザーの感想
}

type RecommendResponse struct {
	NextAction     string  `json:"next_action"`     // "CONTINUE", "ADJUST", "STOP"
	Recommendation string  `json:"recommendation"`  // 具体的なアドバイス
	TargetWeight   float64 `json:"target_weight"`
	TargetReps     int     `json:"target_reps"`
	Reason         string  `json:"reason"`
	MaxWeight      float64 `json:"max_weight"` // 参考用の過去最大重量
}

func handleRecommend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RecommendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	
	// 1. 本来はここでDBに保存
	// INSERT INTO workout_sets (user_id, exercise_id, weight, reps, feeling, set_order) VALUES (...)
	
	// 2. 過去最大重量の取得
	var maxWeight float64
	if db != nil {
		// 実際のDBから過去の最大重量を取得します
		db.QueryRow("SELECT COALESCE(MAX(weight), 0) FROM workout_sets WHERE user_id = $1 AND exercise_id = $2", req.UserID, req.ExerciseID).Scan(&maxWeight)
	}
	
	// 初回利用時、もしくは開発環境でDBに繋がっていない場合のダミー値
	if maxWeight == 0 {
		maxWeight = 100.0
	}

	// 今回の入力が仮に最高重量を上回っていた場合、maxWeightを更新しておく（シミュレーション用）
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

	var resp RecommendResponse
	resp.MaxWeight = maxWeight

	aiJSON, err := callAI(systemPrompt, userPrompt, true)
	if err != nil {
		fmt.Printf("AI API Error (Recommend): %v. Using fallback logic.\n", err)
		// APIコール失敗時のフォールバックロジック
		if req.Feeling == "痛い" || req.Feeling == "違和感" {
			resp.NextAction = "STOP"
			resp.Recommendation = "本日はここで中止しましょう。怪我のリスクがあります。"
			resp.Reason = "体の違和感は何より優先して対処すべきです。最高重量を目指すのはコンディションが良い日にしましょう。"
		} else if req.SetOrder >= 2 && (req.Feeling == "限界" || req.Feeling == "かなりきつい") {
			resp.NextAction = "STOP"
			resp.Recommendation = "本日のこの種目はここで終了し、十分な回復を図りましょう。"
			resp.Reason = "すでに強い疲労感があります。過去最大重量(" + fmt.Sprintf("%.0f", maxWeight) + "kg)に挑戦するためのベース作りとしては今日で十分な刺激です。"
		} else if req.Weight >= maxWeight-5 && (req.Feeling == "軽い" || req.Feeling == "余裕あり") {
			resp.NextAction = "CONTINUE"
			resp.Recommendation = "調子が良さそうです。自己ベスト更新を狙って、少し重量を上げてみましょう。"
			resp.TargetWeight = req.Weight + 2.5
			resp.TargetReps = req.Reps - 2
			if resp.TargetReps < 1 { resp.TargetReps = 1 }
			resp.Reason = "過去最大重量に近い重量にも関わらず余裕があるため、神経系の適応を引き出すチャンスです。"
		} else {
			resp.NextAction = "CONTINUE"
			resp.Recommendation = "フォームを意識して、今の重量でしっかりと回数をこなしましょう。"
			resp.TargetWeight = req.Weight
			resp.TargetReps = req.Reps
			resp.Reason = "最大重量に向けての筋力基盤を作るため、現在の重量で十分なボリューム（回数）を積むことが有効です。"
		}
	} else {
		// AIレスポンスのパース
		// もしマークダウンブロックが含まれていたら除去
		aiStr := strings.TrimSpace(aiJSON)
		if strings.HasPrefix(aiStr, "```json") {
			aiStr = strings.TrimPrefix(aiStr, "```json")
			aiStr = strings.TrimSuffix(strings.TrimSpace(aiStr), "```")
		}

		if err := json.Unmarshal([]byte(aiStr), &resp); err != nil {
			fmt.Printf("AI JSON Parse Error: %v. Raw String: %s\n", err, aiStr)
			// エラー時の適当なデフォルト値
			resp.NextAction = "CONTINUE"
			resp.Recommendation = "通信エラーが発生しましたが、調子に合わせて継続しましょう。"
			resp.TargetWeight = req.Weight
			resp.TargetReps = req.Reps
			resp.Reason = "AIからの応答パースに失敗しました。"
		}
		resp.MaxWeight = maxWeight // 再セット
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type MonthlyPlanRequest struct {
	UserID     int    `json:"user_id"`
	Motivation string `json:"motivation"`
	Frequency  string `json:"frequency"`
}

type DayRoutine struct {
	DayName          string   `json:"day_name"`
	Target           string   `json:"target"`
	ExampleExercises []string `json:"example_exercises"`
}

type MonthlyPlanResponse struct {
	PlanName      string       `json:"plan_name"`
	Frequency     string       `json:"frequency"`
	Description   string       `json:"description"`
	Rationale     string       `json:"rationale"`
	WeeklyRoutine []DayRoutine `json:"weekly_routine"`
}

func handleMonthlyPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// OPTIONS method for CORS if needed is usually handled centrally, but we might just respond OK
	var req MonthlyPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	motivation := req.Motivation
	frequency := req.Frequency
	var resp MonthlyPlanResponse

	if strings.Contains(frequency, "週5") || strings.Contains(motivation, "本気") {
		resp.PlanName = "ブロ割 (部位別特化)"
		resp.Frequency = "週5〜6回"
		resp.Description = "各筋肉群を日ごとに徹底的に追い込む、高頻度・高強度のルーティンです。"
		resp.Rationale = "非常に高いモチベーションと頻度を考慮し、1回あたりの部位を絞って限界まで追い込める分割法を選択しました。"
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
		resp.WeeklyRoutine = []DayRoutine{
			{DayName: "Day 1", Target: "全身 A", ExampleExercises: []string{"スクワット", "ベンチプレス", "ベントオーバーロウ"}},
			{DayName: "Day 2", Target: "全身 B", ExampleExercises: []string{"デッドリフト", "ショルダープレス", "懸垂"}},
		}
	} else {
		resp.PlanName = "PPL法 (Push/Pull/Legs)"
		resp.Frequency = "週3〜4回"
		resp.Description = "押す筋肉、引く筋肉、脚の3グループに分けて鍛える、最もバランスが良く結果が出やすい王道のルーティンです。"
		resp.Rationale = "バランス良く全身を鍛えつつ、各部位に十分な回復期間を与えられるPPL法が最も適していると判断しました。"
		resp.WeeklyRoutine = []DayRoutine{
			{DayName: "Day 1", Target: "Push (胸・肩・三頭)", ExampleExercises: []string{"ベンチプレス", "ショルダープレス", "ディップス"}},
			{DayName: "Day 2", Target: "Pull (背中・二頭)", ExampleExercises: []string{"懸垂", "デッドリフト", "バーベルカール"}},
			{DayName: "Day 3", Target: "Legs (脚・腹)", ExampleExercises: []string{"スクワット", "レッグプレス", "カーフレイズ"}},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
