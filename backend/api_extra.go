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
resp.ChartData = []int{40, 70, 45, 90, 65, 80, 55}

// Fetch up to 3 recent workouts
rows, err := app.db.Query(`
SELECT id, started_at, COALESCE(ended_at, started_at), notes
FROM workouts
WHERE user_id = $1
ORDER BY started_at DESC
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
if dur < 0 {
dur = 0
}

resp.RecentWorkouts = append(resp.RecentWorkouts, WorkoutItemData{
ID:       id,
Title    : notes + " Workout",
Type     : "Strength",
Duration : fmt.Sprintf("%.0f min", dur),
Calories : fmt.Sprintf("%.0f kcal", dur*6+50), // silly mock
Time     : start.Format("03:04 PM"),
})
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
	Exercise string `json:"exercise"`
	Reason   string `json:"reason"`
}

type AlternativeExercise struct {
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
		Message:      "機材が埋まっていても大丈夫です。代わりに以下の種目で同じ部位をしっかり追い込みましょう！",
	}

	// 簡易的な文字列マッチングによるモックAI
	ex := req.Exercise
	
	switch {
	case strings.Contains(ex, "ベンチ") || strings.Contains(ex, "胸"):
		resp.Alternatives = []AlternativeExercise{
			{Name: "ダンベルプレス", Description: "ベンチ台が空いていれば、ダンベルを使って同じように大胸筋を効果的に鍛えられます。"},
			{Name: "チェストプレスマシン", Description: "マシンを使えば安全かつピンポイントで胸を追い込めます。軌道が固定されているので初心者にもおすすめです。"},
			{Name: "プッシュアップ (ディップス)", Description: "もし何も空いていなければ、自重やディップススタンドで極限まで追い込みましょう！"},
		}
	case strings.Contains(ex, "スクワット") || strings.Contains(ex, "脚"):
		resp.Alternatives = []AlternativeExercise{
			{Name: "レッグプレスマシン", Description: "フリーウェイトが使えない場合、レッグプレスで脚全体に高負荷をかけるのが最適です。"},
			{Name: "ブルガリアンスクワット", Description: "ダンベルと適当な台があれば、片脚ずつ強烈な刺激を入れられます。"},
			{Name: "レッグエクステンション", Description: "大腿四頭筋に絞って集中的に追い込むのも一つの手です。"},
		}
	case strings.Contains(ex, "デッドリフト") || strings.Contains(ex, "背中") || strings.Contains(ex, "懸垂") || strings.Contains(ex, "ラット"):
		resp.Alternatives = []AlternativeExercise{
			{Name: "ラットプルダウン", Description: "懸垂やデッドリフトの代わりに、広背筋を安全に鍛える王道マシンです。"},
			{Name: "ダンベルロウ", Description: "ダンベル一つで背中の厚みを作ることができます。"},
			{Name: "シーテッドロウ", Description: "マシンのケーブルを使って、背中の中央部を集中的に刺激します。"},
		}
	case strings.Contains(ex, "プレス") || strings.Contains(ex, "肩"):
		resp.Alternatives = []AlternativeExercise{
			{Name: "ダンベルショルダープレス", Description: "バーベルがなくてもダンベルがあれば同等の効果を得られます。"},
			{Name: "サイドレイズ", Description: "マシンの代わりにダンベルやケーブルで肩の側部を集中的に狙いましょう。"},
		}
	default:
		// フォールバック
		resp.Alternatives = []AlternativeExercise{
			{Name: "ダンベルバリエーション", Description: "対象の部位をダンベルを使って鍛える種目に変更しましょう。"},
			{Name: "専用マシン", Description: "同じ部位をターゲットにした空いているマシンを活用しましょう。"},
		}
		resp.Message = fmt.Sprintf("「%s」のフリーウェイトが空いていない場合、ターゲットとする筋肉をマシンやダンベルで代替できます！", ex)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
