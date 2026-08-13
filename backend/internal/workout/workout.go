package workout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/ai"
	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
	"github.com/jPurin-gg/myfitlog-backend/internal/prompt"
)

var (
	ErrNotFound = errors.New("workout not found")
	ErrConflict = errors.New("workout conflict")
)

var idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

type SetInput struct {
	ExerciseID string  `json:"exercise_id"`
	SetOrder   int     `json:"set_order"`
	Weight     float64 `json:"weight"`
	Reps       int     `json:"reps"`
	Feeling    string  `json:"feeling"`
}

type Set struct {
	ID         int     `json:"id"`
	WorkoutID  int     `json:"workout_id"`
	ExerciseID string  `json:"exercise_id"`
	SetOrder   int     `json:"set_order"`
	Weight     float64 `json:"weight"`
	Reps       int     `json:"reps"`
	Feeling    string  `json:"feeling"`
	IsPR       bool    `json:"is_pr"`
	CreatedAt  string  `json:"created_at"`
}

type Recommendation struct {
	NextAction     string  `json:"next_action"`
	Recommendation string  `json:"recommendation"`
	TargetWeight   float64 `json:"target_weight"`
	TargetReps     int     `json:"target_reps"`
	Reason         string  `json:"reason"`
	RecordTemplate string  `json:"record_template"`
	MaxWeight      float64 `json:"max_weight"`
}

type HistorySet struct {
	Date         string
	ExerciseName string
	SetOrder     int
	Weight       float64
	Reps         int
	Feeling      string
	IsPR         bool
}

type RecommendationContext struct {
	Set          Set
	ExerciseName string
	MaxWeight    float64
	Recent       []HistorySet
	WorkoutSets  []HistorySet
}

type SummaryExercise struct {
	ExerciseID  string  `json:"exercise_id"`
	Name        string  `json:"name"`
	Sets        int     `json:"sets"`
	TotalReps   int     `json:"total_reps"`
	BestWeight  float64 `json:"best_weight"`
	TotalVolume float64 `json:"total_volume"`
}

type Summary struct {
	TotalSets   int               `json:"total_sets"`
	TotalReps   int               `json:"total_reps"`
	TotalVolume float64           `json:"total_volume"`
	DurationMin int               `json:"duration_min"`
	PRCount     int               `json:"pr_count"`
	AIComment   string            `json:"ai_comment,omitempty"`
	Exercises   []SummaryExercise `json:"exercises"`
}

type Detail struct {
	ID        int     `json:"id"`
	Title     string  `json:"title"`
	StartedAt string  `json:"started_at"`
	EndedAt   string  `json:"ended_at"`
	Status    string  `json:"status"`
	Summary   Summary `json:"summary"`
}

type FinishResult struct {
	WorkoutID int     `json:"workout_id"`
	StartedAt string  `json:"started_at"`
	EndedAt   string  `json:"ended_at"`
	Status    string  `json:"status"`
	Summary   Summary `json:"summary"`
}

type CalendarSet struct {
	ID           int     `json:"id,omitempty"`
	ExerciseID   string  `json:"exercise_id"`
	ExerciseName string  `json:"exercise_name,omitempty"`
	Weight       float64 `json:"weight"`
	Reps         int     `json:"reps"`
	SetOrder     int     `json:"set_order"`
	Feeling      string  `json:"feeling"`
}

type CalendarWorkout struct {
	WorkoutID int           `json:"workout_id"`
	Date      string        `json:"date"`
	Title     string        `json:"title"`
	Sets      []CalendarSet `json:"sets"`
}

type CalendarWorkoutInput struct {
	Title string        `json:"title"`
	Sets  []CalendarSet `json:"sets"`
}

type Repository interface {
	RecordSet(ctx context.Context, userID, workoutID int, key string, input SetInput) (Set, bool, error)
	RecommendationContext(ctx context.Context, userID, workoutID, setID int) (RecommendationContext, error)
	Finish(ctx context.Context, userID, workoutID int) (Detail, error)
	Detail(ctx context.Context, userID, workoutID int) (Detail, error)
	SaveSummaryComment(ctx context.Context, userID, workoutID int, comment string) error
	CalendarWorkout(ctx context.Context, userID int, date time.Time) (CalendarWorkout, error)
	SaveCalendarWorkout(ctx context.Context, userID int, date time.Time, input CalendarWorkoutInput) (CalendarWorkout, error)
}

type PromptRenderer interface {
	Pair(systemFilename, userFilename string, data any) (string, string, error)
}

type Service struct {
	repository Repository
	aiClient   ai.Client
	prompts    PromptRenderer
	clock      clock.Clock
}

func NewService(repository Repository, aiClient ai.Client, prompts PromptRenderer, appClock clock.Clock) *Service {
	return &Service{repository: repository, aiClient: aiClient, prompts: prompts, clock: appClock}
}

func (s *Service) RecordSet(ctx context.Context, userID, workoutID int, key string, input SetInput) (Set, bool, error) {
	key = strings.TrimSpace(key)
	input.ExerciseID = strings.TrimSpace(input.ExerciseID)
	input.Feeling = strings.TrimSpace(input.Feeling)
	fields := map[string]string{}
	if !idempotencyPattern.MatchString(key) {
		fields["Idempotency-Key"] = "must be 8-128 URL-safe characters"
	}
	if workoutID <= 0 {
		fields["workout_id"] = "must be positive"
	}
	if input.ExerciseID == "" {
		fields["exercise_id"] = "required"
	}
	if input.SetOrder <= 0 {
		fields["set_order"] = "must be positive"
	}
	if input.Weight < 0 {
		fields["weight"] = "must be non-negative"
	}
	if input.Reps <= 0 {
		fields["reps"] = "must be positive"
	}
	if len(fields) > 0 {
		return Set{}, false, apperr.Validation("セット記録の入力を確認してください。", fields)
	}
	set, replayed, err := s.repository.RecordSet(ctx, userID, workoutID, key, input)
	if errors.Is(err, ErrNotFound) {
		return Set{}, false, apperr.NotFound("進行中のワークアウトまたは種目が見つかりません。")
	}
	if errors.Is(err, ErrConflict) {
		return Set{}, false, apperr.Conflict("同じIdempotency-Keyが異なる内容で使用されています。")
	}
	if err != nil {
		return Set{}, false, apperr.Internal(err)
	}
	return set, replayed, nil
}

func (s *Service) Recommend(ctx context.Context, userID, workoutID, setID int) (Recommendation, error) {
	data, err := s.repository.RecommendationContext(ctx, userID, workoutID, setID)
	if errors.Is(err, ErrNotFound) {
		return Recommendation{}, apperr.NotFound("対象のセットが見つかりません。")
	}
	if err != nil {
		return Recommendation{}, apperr.Internal(err)
	}
	systemPrompt, userPrompt, err := s.prompts.Pair("recommend_system.txt", "recommend_user.txt", map[string]any{
		"ExerciseName": data.ExerciseName, "SetOrder": data.Set.SetOrder, "Weight": data.Set.Weight,
		"Reps": data.Set.Reps, "Feeling": data.Set.Feeling, "MaxWeight": data.MaxWeight,
		"RecentExerciseHistory": formatRecent(data.Recent), "TodayWorkoutContext": formatWorkout(data.WorkoutSets),
	})
	if err != nil {
		return Recommendation{}, apperr.Internal(err)
	}
	result, err := s.aiClient.Complete(ctx, ai.Request{Task: ai.TaskRecommendation, SystemPrompt: systemPrompt, UserPrompt: userPrompt, JSONMode: true})
	if err != nil {
		return Recommendation{}, ai.ToAppError(err)
	}
	var response Recommendation
	if err := json.Unmarshal([]byte(prompt.JSONText(result)), &response); err != nil {
		return Recommendation{}, apperr.Wrap(err, 502, apperr.CodeAIUnavailable, "AIの提案を解析できません。")
	}
	if err := validateRecommendation(&response); err != nil {
		return Recommendation{}, err
	}
	response.MaxWeight = data.MaxWeight
	return response, nil
}

func validateRecommendation(response *Recommendation) error {
	response.NextAction = strings.ToUpper(strings.TrimSpace(response.NextAction))
	response.Recommendation = strings.TrimSpace(response.Recommendation)
	response.Reason = strings.TrimSpace(response.Reason)
	if response.NextAction != "CONTINUE" && response.NextAction != "STOP" && response.NextAction != "ADJUST" {
		return apperr.New(502, apperr.CodeAIUnavailable, "AIの提案が不正です。")
	}
	if response.Recommendation == "" || response.Reason == "" || response.TargetWeight < 0 || response.TargetWeight > 2000 || response.TargetReps < 0 || response.TargetReps > 1000 {
		return apperr.New(502, apperr.CodeAIUnavailable, "AIの提案が不完全です。")
	}
	if response.NextAction != "STOP" && response.TargetReps == 0 {
		return apperr.New(502, apperr.CodeAIUnavailable, "AIの提案が不完全です。")
	}
	return nil
}

func (s *Service) Finish(ctx context.Context, userID, workoutID int) (FinishResult, error) {
	detail, err := s.repository.Finish(ctx, userID, workoutID)
	if errors.Is(err, ErrNotFound) {
		return FinishResult{}, apperr.NotFound("ワークアウトが見つかりません。")
	}
	if err != nil {
		return FinishResult{}, apperr.Internal(err)
	}
	detail.Summary = s.ensureSummaryComment(ctx, userID, workoutID, detail.Summary)
	return FinishResult{WorkoutID: detail.ID, StartedAt: detail.StartedAt, EndedAt: detail.EndedAt, Status: "completed", Summary: detail.Summary}, nil
}

func (s *Service) Detail(ctx context.Context, userID, workoutID int) (Detail, error) {
	detail, err := s.repository.Detail(ctx, userID, workoutID)
	if errors.Is(err, ErrNotFound) {
		return Detail{}, apperr.NotFound("ワークアウトが見つかりません。")
	}
	if err != nil {
		return Detail{}, apperr.Internal(err)
	}
	if detail.Status == "completed" {
		detail.Summary = s.ensureSummaryComment(ctx, userID, workoutID, detail.Summary)
	}
	detail.Title = displayTitle(detail.Title)
	return detail, nil
}

func (s *Service) CalendarWorkout(ctx context.Context, userID int, dateText string) (CalendarWorkout, error) {
	date, err := s.parseDate(dateText)
	if err != nil {
		return CalendarWorkout{}, err
	}
	result, err := s.repository.CalendarWorkout(ctx, userID, date)
	if errors.Is(err, ErrNotFound) {
		return CalendarWorkout{}, apperr.NotFound("指定日のワークアウトがありません。")
	}
	if err != nil {
		return CalendarWorkout{}, apperr.Internal(err)
	}
	return result, nil
}

func (s *Service) SaveCalendarWorkout(ctx context.Context, userID int, dateText string, input CalendarWorkoutInput) (CalendarWorkout, error) {
	date, err := s.parseDate(dateText)
	if err != nil {
		return CalendarWorkout{}, err
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		input.Title = "筋トレ"
	}
	if len(input.Sets) == 0 {
		return CalendarWorkout{}, apperr.Validation("少なくとも1セットは必要です。", map[string]string{"sets": "required"})
	}
	for index := range input.Sets {
		set := &input.Sets[index]
		set.ExerciseID = strings.TrimSpace(set.ExerciseID)
		set.Feeling = strings.TrimSpace(set.Feeling)
		if set.ExerciseID == "" || set.Reps <= 0 || set.Weight < 0 {
			return CalendarWorkout{}, apperr.Validation("セットの種目・重量・回数を確認してください。", map[string]string{"sets": "invalid set"})
		}
		if set.SetOrder <= 0 {
			set.SetOrder = index + 1
		}
	}
	result, err := s.repository.SaveCalendarWorkout(ctx, userID, date, input)
	if errors.Is(err, ErrNotFound) {
		return CalendarWorkout{}, apperr.NotFound("セット内の種目が見つかりません。")
	}
	if err != nil {
		return CalendarWorkout{}, apperr.Internal(err)
	}
	return result, nil
}

func (s *Service) ensureSummaryComment(ctx context.Context, userID, workoutID int, summary Summary) Summary {
	if strings.TrimSpace(summary.AIComment) != "" {
		return summary
	}
	encoded, _ := json.Marshal(summary)
	data, err := s.repository.RecommendationContext(ctx, userID, workoutID, 0)
	if err != nil {
		summary.AIComment = "AIコメントを生成できませんでした。"
		return summary
	}
	systemPrompt, userPrompt, err := s.prompts.Pair("workout_summary_system.txt", "workout_summary_user.txt", map[string]any{"SummaryJSON": string(encoded), "WorkoutSetContext": formatWorkout(data.WorkoutSets)})
	if err != nil {
		summary.AIComment = "AIコメントを生成できませんでした。"
		return summary
	}
	result, err := s.aiClient.Complete(ctx, ai.Request{Task: ai.TaskWorkoutSummary, SystemPrompt: systemPrompt, UserPrompt: userPrompt, JSONMode: true})
	if err != nil {
		summary.AIComment = "AIコメントを生成できませんでした。"
		return summary
	}
	var response struct {
		Comment string `json:"comment"`
	}
	if json.Unmarshal([]byte(prompt.JSONText(result)), &response) != nil || strings.TrimSpace(response.Comment) == "" {
		summary.AIComment = "AIコメントを生成できませんでした。"
		return summary
	}
	summary.AIComment = strings.TrimSpace(response.Comment)
	_ = s.repository.SaveSummaryComment(ctx, userID, workoutID, summary.AIComment)
	return summary
}

func (s *Service) parseDate(value string) (time.Time, error) {
	date, err := time.ParseInLocation("2006-01-02", value, s.clock.Now().Location())
	if err != nil {
		return time.Time{}, apperr.Validation("日付はYYYY-MM-DD形式で指定してください。", nil)
	}
	return date, nil
}

func formatRecent(rows []HistorySet) string {
	if len(rows) == 0 {
		return "記録なし"
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		feeling := row.Feeling
		if feeling == "" {
			feeling = "感想なし"
		}
		pr := ""
		if row.IsPR {
			pr = " 自己ベスト"
		}
		lines = append(lines, fmt.Sprintf("- %s: %.1fkg x %d回%s / 感想: %s", row.Date, row.Weight, row.Reps, pr, feeling))
	}
	return strings.Join(lines, "\n")
}

func formatWorkout(rows []HistorySet) string {
	if len(rows) == 0 {
		return "記録なし"
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		feeling := row.Feeling
		if feeling == "" {
			feeling = "感想なし"
		}
		lines = append(lines, fmt.Sprintf("- %s %dセット目: %.1fkg x %d回 / 感想: %s", row.ExerciseName, row.SetOrder, row.Weight, row.Reps, feeling))
	}
	return strings.Join(lines, "\n")
}

func displayTitle(value string) string {
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
