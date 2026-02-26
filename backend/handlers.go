package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	// シミュレーションロジック
	var resp RecommendResponse
	resp.MaxWeight = maxWeight
	
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
		resp.Reason = "過去最大重量(" + fmt.Sprintf("%.0f", maxWeight) + "kg)に近い重量にも関わらず余裕があるため、神経系の適応を引き出すチャンスです。"
	} else {
		resp.NextAction = "CONTINUE"
		resp.Recommendation = "フォームを意識して、今の重量でしっかりと回数をこなしましょう。"
		resp.TargetWeight = req.Weight
		resp.TargetReps = req.Reps
		resp.Reason = "最大重量(" + fmt.Sprintf("%.0f", maxWeight) + "kg)に向けての筋力基盤を作るため、現在の重量で十分なボリューム（回数）を積むことが有効です。"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
