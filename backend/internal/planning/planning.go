package planning

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/ai"
	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
	"github.com/jPurin-gg/myfitlog-backend/internal/profile"
	"github.com/jPurin-gg/myfitlog-backend/internal/prompt"
)

var (
	ErrNotFound         = errors.New("plan not found")
	ErrExerciseNotFound = errors.New("exercise not found")
)

const (
	AIStatusApplied      = "applied"
	AIStatusFallback     = "fallback"
	AIStatusNotRequested = "not_requested"
)

type DayRoutine struct {
	DayName          string   `json:"day_name"`
	Target           string   `json:"target"`
	ExampleExercises []string `json:"example_exercises"`
	ExerciseIDs      []string `json:"exercise_ids,omitempty"`
}

type MonthlyPlan struct {
	ID              int          `json:"id,omitempty"`
	PlanMonth       string       `json:"plan_month"`
	PlanName        string       `json:"plan_name"`
	Frequency       string       `json:"frequency"`
	Description     string       `json:"description"`
	Rationale       string       `json:"rationale"`
	RestDays        []int        `json:"rest_days"`
	RecommendedDays []int        `json:"recommended_days"`
	WeeklyRoutine   []DayRoutine `json:"weekly_routine"`
}

type MonthlyPlanInput struct {
	PlanName        string       `json:"plan_name"`
	Frequency       string       `json:"frequency"`
	Description     string       `json:"description"`
	Rationale       string       `json:"rationale"`
	RestDays        []int        `json:"rest_days"`
	RecommendedDays []int        `json:"recommended_days"`
	WeeklyRoutine   []DayRoutine `json:"weekly_routine"`
}

type GenerateMonthlyInput struct {
	Motivation string `json:"motivation"`
	Frequency  string `json:"frequency"`
	RestDays   []int  `json:"rest_days"`
}

type StartDailyInput struct {
	UseAI      *bool   `json:"use_ai"`
	ExerciseID *string `json:"exercise_id,omitempty"`
}

type Candidate struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Equipment      string   `json:"equipment"`
	Level          string   `json:"level"`
	Category       string   `json:"category"`
	PrimaryMuscles []string `json:"primary_muscles"`
	IsFavorite     bool     `json:"is_favorite,omitempty"`
}

type PlanExercise struct {
	ExerciseID    string  `json:"exercise_id"`
	Name          string  `json:"name"`
	PlannedSets   int     `json:"planned_sets"`
	TargetWeight  float64 `json:"target_weight"`
	TargetReps    int     `json:"target_reps"`
	LastMaxWeight float64 `json:"last_max_weight"`
}

type WorkoutPlan struct {
	WorkoutTitle         string         `json:"workout_title"`
	Target               string         `json:"target"`
	EstimatedDurationMin int            `json:"estimated_duration_min"`
	CoachNote            string         `json:"coach_note"`
	Exercises            []PlanExercise `json:"exercises"`
}

type PlanSession struct {
	ID        int         `json:"id,omitempty"`
	WorkoutID int         `json:"workout_id,omitempty"`
	PlanDate  string      `json:"plan_date"`
	Status    string      `json:"status"`
	AIStatus  string      `json:"ai_status"`
	Plan      WorkoutPlan `json:"plan"`
}

// TodayWorkout is the workout recorded on the app-timezone date returned by Today.
type TodayWorkout struct {
	ID        int        `json:"id"`
	Status    string     `json:"status"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// Today is the single source of "today" for clients. Date and Timezone come
// from the app clock; Plan is the saved daily plan (status "active"), a draft
// built from the monthly plan (status "draft"), or nil when neither exists.
// RestDay is true only when no daily plan is saved and the monthly plan marks
// the weekday as a rest day.
type Today struct {
	Date     string        `json:"date"`
	Timezone string        `json:"timezone"`
	RestDay  bool          `json:"rest_day"`
	Plan     *PlanSession  `json:"plan"`
	Workout  *TodayWorkout `json:"workout"`
}

type ExerciseStats struct {
	Name         string
	TargetSets   int
	TargetWeight float64
	TargetReps   int
	MaxWeight    float64
}

type Repository interface {
	Monthly(ctx context.Context, userID int, month string) (MonthlyPlan, error)
	MonthlyList(ctx context.Context, userID int) ([]MonthlyPlan, error)
	SaveMonthly(ctx context.Context, userID int, month string, plan MonthlyPlan) (MonthlyPlan, error)
	Candidates(ctx context.Context, userID int) ([]Candidate, error)
	Daily(ctx context.Context, userID int, date time.Time) (PlanSession, error)
	SaveDaily(ctx context.Context, userID int, date time.Time, plan WorkoutPlan) (PlanSession, error)
	CreateDailyIfMissing(ctx context.Context, userID int, date time.Time, plan WorkoutPlan) (bool, error)
	AttachWorkout(ctx context.Context, userID int, date time.Time) (PlanSession, error)
	ResolveExercise(ctx context.Context, userID int, exerciseID, name string) (ExerciseStats, string, error)
	// WorkoutOn returns the workout started in [date, date+1 day), preferring an
	// unfinished one. It returns ErrNotFound when there is none.
	WorkoutOn(ctx context.Context, userID int, date time.Time) (TodayWorkout, error)
}

type PreferencesReader interface {
	Get(ctx context.Context, userID int) (profile.Preferences, error)
}

type PromptRenderer interface {
	Pair(systemFilename, userFilename string, data any) (string, string, error)
}

type Service struct {
	repository  Repository
	preferences PreferencesReader
	aiClient    ai.Client
	prompts     PromptRenderer
	clock       clock.Clock
	aiTimeout   time.Duration
	logger      *slog.Logger
}

func NewService(repository Repository, preferences PreferencesReader, aiClient ai.Client, prompts PromptRenderer, appClock clock.Clock, aiTimeout time.Duration) *Service {
	return &Service{repository: repository, preferences: preferences, aiClient: aiClient, prompts: prompts, clock: appClock, aiTimeout: aiTimeout}
}

func (s *Service) WithLogger(logger *slog.Logger) *Service {
	s.logger = logger
	return s
}

func (s *Service) Monthly(ctx context.Context, userID int, month string) (MonthlyPlan, error) {
	if err := ValidateMonth(month); err != nil {
		return MonthlyPlan{}, err
	}
	plan, err := s.repository.Monthly(ctx, userID, month)
	if errors.Is(err, ErrNotFound) {
		return MonthlyPlan{}, apperr.NotFound("月間プランがまだありません。")
	}
	if err != nil {
		return MonthlyPlan{}, apperr.Internal(err)
	}
	return plan, nil
}

func (s *Service) MonthlyList(ctx context.Context, userID int) ([]MonthlyPlan, error) {
	plans, err := s.repository.MonthlyList(ctx, userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if plans == nil {
		plans = []MonthlyPlan{}
	}
	return plans, nil
}

func (s *Service) SaveMonthly(ctx context.Context, userID int, month string, input MonthlyPlanInput) (MonthlyPlan, error) {
	if err := ValidateMonth(month); err != nil {
		return MonthlyPlan{}, err
	}
	plan := MonthlyPlan{PlanMonth: month, PlanName: strings.TrimSpace(input.PlanName), Frequency: strings.TrimSpace(input.Frequency), Description: strings.TrimSpace(input.Description), Rationale: strings.TrimSpace(input.Rationale), RestDays: NormalizeWeekdays(input.RestDays), RecommendedDays: NormalizeWeekdays(input.RecommendedDays), WeeklyRoutine: input.WeeklyRoutine}
	if plan.PlanName == "" || len(plan.WeeklyRoutine) == 0 {
		return MonthlyPlan{}, apperr.Validation("月間プラン名とルーティンが必要です。", nil)
	}
	if err := validateSavedPlan(plan); err != nil {
		return MonthlyPlan{}, err
	}
	saved, err := s.repository.SaveMonthly(ctx, userID, month, plan)
	if err != nil {
		return MonthlyPlan{}, apperr.Internal(err)
	}
	return saved, nil
}

func (s *Service) GenerateMonthly(ctx context.Context, userID int, month string, input GenerateMonthlyInput) (MonthlyPlan, error) {
	if err := ValidateMonth(month); err != nil {
		return MonthlyPlan{}, err
	}
	restDays := NormalizeWeekdays(input.RestDays)
	if len(restDays) >= 7 {
		return MonthlyPlan{}, apperr.Validation("休息日は6日までにしてください。", map[string]string{"rest_days": "at least one training day is required"})
	}
	preferences, err := s.preferences.Get(ctx, userID)
	if err != nil {
		return MonthlyPlan{}, err
	}
	rawCandidates, err := s.repository.Candidates(ctx, userID)
	if err != nil {
		return MonthlyPlan{}, apperr.Internal(err)
	}
	candidates := filterCandidates(rawCandidates, preferences)
	if len(candidates) == 0 {
		return MonthlyPlan{}, apperr.NotFound("月間プランに使える種目がありません。")
	}
	candidateJSON, _ := json.Marshal(candidates)
	systemPrompt, userPrompt, err := s.prompts.Pair("monthly_plan_system.txt", "monthly_plan_user.txt", map[string]any{
		"Motivation": input.Motivation, "Frequency": input.Frequency,
		"RestDays": WeekdayListText(restDays), "RestDaysJSON": marshalString(restDays),
		"PreferencesJSON": marshalString(preferences), "PreferencesText": preferencesText(preferences),
		"CandidatesJSON": string(candidateJSON),
	})
	if err != nil {
		return MonthlyPlan{}, apperr.Internal(err)
	}
	aiStarted := time.Now()
	result, err := s.aiClient.Complete(ctx, ai.Request{Task: ai.TaskMonthlyPlan, SystemPrompt: systemPrompt, UserPrompt: userPrompt, JSONMode: true})
	if err != nil {
		ai.LogFeatureOutcome(ctx, s.logger, ai.TaskMonthlyPlan, "provider_error", aiStarted)
		return MonthlyPlan{}, ai.ToAppError(err)
	}
	var plan MonthlyPlan
	if err := json.Unmarshal([]byte(prompt.JSONText(result)), &plan); err != nil {
		ai.LogFeatureOutcome(ctx, s.logger, ai.TaskMonthlyPlan, "invalid_output", aiStarted)
		return MonthlyPlan{}, apperr.Wrap(err, 502, apperr.CodeAIUnavailable, "AIの月間プランを解析できません。")
	}
	if err := validateAIPlan(&plan, candidates); err != nil {
		ai.LogFeatureOutcome(ctx, s.logger, ai.TaskMonthlyPlan, "invalid_output", aiStarted)
		return MonthlyPlan{}, err
	}
	if plan.Frequency == "" {
		plan.Frequency = input.Frequency
	}
	plan.PlanMonth = month
	applyRestDays(&plan, restDays)
	if err := validateSavedPlan(plan); err != nil {
		ai.LogFeatureOutcome(ctx, s.logger, ai.TaskMonthlyPlan, "invalid_output", aiStarted)
		return MonthlyPlan{}, apperr.New(502, apperr.CodeAIUnavailable, "AIの月間プランが不完全です。")
	}
	saved, err := s.repository.SaveMonthly(ctx, userID, month, plan)
	if err != nil {
		ai.LogFeatureOutcome(ctx, s.logger, ai.TaskMonthlyPlan, "storage_error", aiStarted)
		return MonthlyPlan{}, apperr.Internal(err)
	}
	ai.LogFeatureOutcome(ctx, s.logger, ai.TaskMonthlyPlan, "applied", aiStarted)
	return saved, nil
}

func (s *Service) Daily(ctx context.Context, userID int, dateText string) (PlanSession, error) {
	date, err := s.parseDate(dateText)
	if err != nil {
		return PlanSession{}, err
	}
	session, err := s.repository.Daily(ctx, userID, date)
	if err == nil {
		session.AIStatus = AIStatusNotRequested
		return session, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return PlanSession{}, apperr.Internal(err)
	}
	base, err := s.basePlan(ctx, userID, date)
	if err != nil {
		return PlanSession{}, err
	}
	return PlanSession{PlanDate: dateText, Status: "draft", AIStatus: AIStatusNotRequested, Plan: base}, nil
}

func (s *Service) SaveDaily(ctx context.Context, userID int, dateText string, plan WorkoutPlan) (PlanSession, error) {
	date, err := s.parseDate(dateText)
	if err != nil {
		return PlanSession{}, err
	}
	if err := s.normalizeDaily(ctx, userID, &plan); err != nil {
		return PlanSession{}, err
	}
	saved, err := s.repository.SaveDaily(ctx, userID, date, plan)
	if err != nil {
		return PlanSession{}, apperr.Internal(err)
	}
	saved.AIStatus = AIStatusNotRequested
	return saved, nil
}

func (s *Service) StartDaily(ctx context.Context, userID int, dateText string, input StartDailyInput) (PlanSession, error) {
	date, err := s.parseDate(dateText)
	if err != nil {
		return PlanSession{}, err
	}
	if !sameDate(date, s.clock.Now()) {
		return PlanSession{}, apperr.Validation("ワークアウトを開始できるのは今日のプランだけです。", map[string]string{"date": "must be today"})
	}
	aiStatus := AIStatusNotRequested
	var manual *WorkoutPlan
	if input.ExerciseID != nil {
		plan := WorkoutPlan{
			WorkoutTitle: "フリーワークアウト", Target: "自分で選んだ種目", EstimatedDurationMin: 15,
			CoachNote: "体調に合わせて無理のない重量で進めましょう。",
			Exercises: []PlanExercise{{ExerciseID: strings.TrimSpace(*input.ExerciseID), PlannedSets: 3, TargetReps: 10}},
		}
		if err := s.normalizeDaily(ctx, userID, &plan); err != nil {
			return PlanSession{}, err
		}
		manual = &plan
	}
	if _, err := s.repository.Daily(ctx, userID, date); errors.Is(err, ErrNotFound) {
		var base WorkoutPlan
		if manual != nil {
			base = *manual
		} else {
			var buildErr error
			base, buildErr = s.basePlan(ctx, userID, date)
			if buildErr != nil {
				return PlanSession{}, buildErr
			}
		}
		aiStarted := time.Now()
		refined := base
		if manual == nil && (input.UseAI == nil || *input.UseAI) {
			aiContext, cancel := s.optionalAIContext(ctx)
			adjusted, refineErr := s.refineDaily(aiContext, base)
			cancel()
			if refineErr != nil {
				aiStatus = AIStatusFallback
			} else {
				refined = adjusted
				aiStatus = AIStatusApplied
			}
		}
		created, saveErr := s.repository.CreateDailyIfMissing(ctx, userID, date, refined)
		if saveErr != nil {
			ai.LogFeatureOutcome(ctx, s.logger, ai.TaskWorkoutPlan, "storage_error", aiStarted)
			return PlanSession{}, apperr.Internal(saveErr)
		}
		if !created {
			// Another start or manual edit won while optional AI was running.
			aiStatus = AIStatusNotRequested
		}
		ai.LogFeatureOutcome(ctx, s.logger, ai.TaskWorkoutPlan, aiStatus, aiStarted)
	} else if err != nil {
		return PlanSession{}, apperr.Internal(err)
	}
	session, err := s.repository.AttachWorkout(ctx, userID, date)
	if err != nil {
		return PlanSession{}, apperr.Internal(err)
	}
	session.AIStatus = aiStatus
	return session, nil
}

// Today resolves the app-timezone date on the server and returns what a start
// request for that date would use: the saved daily plan first, otherwise the
// monthly-plan draft. Clients must not derive "today" from the browser clock.
func (s *Service) Today(ctx context.Context, userID int) (Today, error) {
	now := s.clock.Now()
	year, month, day := now.Date()
	date := time.Date(year, month, day, 0, 0, 0, 0, now.Location())
	result := Today{Date: date.Format("2006-01-02"), Timezone: now.Location().String()}

	session, err := s.repository.Daily(ctx, userID, date)
	switch {
	case err == nil:
		session.AIStatus = AIStatusNotRequested
		result.Plan = &session
	case errors.Is(err, ErrNotFound):
		monthly, monthlyErr := s.monthlyFor(ctx, userID, date)
		if isNotFound(monthlyErr) {
			break
		}
		if monthlyErr != nil {
			return Today{}, monthlyErr
		}
		result.RestDay = routineIndex(monthly, date) < 0
		base, buildErr := s.planFromMonthly(ctx, userID, date, monthly)
		if isNotFound(buildErr) {
			break
		}
		if buildErr != nil {
			return Today{}, buildErr
		}
		result.Plan = &PlanSession{PlanDate: result.Date, Status: "draft", AIStatus: AIStatusNotRequested, Plan: base}
	default:
		return Today{}, apperr.Internal(err)
	}

	workout, err := s.repository.WorkoutOn(ctx, userID, date)
	if err == nil {
		result.Workout = &workout
	} else if !errors.Is(err, ErrNotFound) {
		return Today{}, apperr.Internal(err)
	}
	return result, nil
}

func (s *Service) basePlan(ctx context.Context, userID int, date time.Time) (WorkoutPlan, error) {
	monthly, err := s.monthlyFor(ctx, userID, date)
	if err != nil {
		return WorkoutPlan{}, err
	}
	return s.planFromMonthly(ctx, userID, date, monthly)
}

func (s *Service) monthlyFor(ctx context.Context, userID int, date time.Time) (MonthlyPlan, error) {
	monthly, err := s.repository.Monthly(ctx, userID, date.Format("2006-01"))
	if errors.Is(err, ErrNotFound) {
		return MonthlyPlan{}, apperr.NotFound("今月の月間プランがまだありません。")
	}
	if err != nil {
		return MonthlyPlan{}, apperr.Internal(err)
	}
	if len(monthly.WeeklyRoutine) == 0 {
		return MonthlyPlan{}, apperr.NotFound("今月の月間プランがまだありません。")
	}
	return monthly, nil
}

// routineIndex returns the weekly routine assigned to date's weekday, or -1
// when the monthly plan treats that weekday as a rest day.
func routineIndex(monthly MonthlyPlan, date time.Time) int {
	for i, day := range monthly.RecommendedDays {
		if day == int(date.Weekday()) {
			if i < len(monthly.WeeklyRoutine) {
				return i
			}
			return -1
		}
	}
	return -1
}

func (s *Service) planFromMonthly(ctx context.Context, userID int, date time.Time, monthly MonthlyPlan) (WorkoutPlan, error) {
	index := routineIndex(monthly, date)
	routine := monthly.WeeklyRoutine[0]
	coachNote := "今日は月間プラン上は休息日ですが、実施する場合に備えて軽めに調整します。"
	if index >= 0 {
		routine = monthly.WeeklyRoutine[index]
		coachNote = "月間プランの今日のメニューに、直近の重量と目標セット数を反映します。"
	}
	exercises := []PlanExercise{}
	for i, name := range routine.ExampleExercises {
		id := ""
		if i < len(routine.ExerciseIDs) {
			id = routine.ExerciseIDs[i]
		}
		stats, resolvedID, err := s.repository.ResolveExercise(ctx, userID, id, name)
		if errors.Is(err, ErrExerciseNotFound) {
			continue
		}
		if err != nil {
			return WorkoutPlan{}, apperr.Internal(err)
		}
		exercises = append(exercises, PlanExercise{ExerciseID: resolvedID, Name: stats.Name, PlannedSets: stats.TargetSets, TargetWeight: stats.TargetWeight, TargetReps: stats.TargetReps, LastMaxWeight: stats.MaxWeight})
	}
	if len(exercises) == 0 {
		return WorkoutPlan{}, apperr.NotFound("月間プランの種目を辞書から見つけられません。")
	}
	estimated := len(exercises) * 12
	if estimated < 30 {
		estimated = 30
	}
	return WorkoutPlan{WorkoutTitle: routine.Target, Target: routine.Target, EstimatedDurationMin: estimated, CoachNote: coachNote, Exercises: exercises}, nil
}

func (s *Service) refineDaily(ctx context.Context, base WorkoutPlan) (WorkoutPlan, error) {
	encoded, _ := json.Marshal(base)
	systemPrompt, userPrompt, err := s.prompts.Pair("workout_plan_system.txt", "workout_plan_user.txt", map[string]any{"BasePlanJSON": string(encoded)})
	if err != nil {
		return WorkoutPlan{}, apperr.Internal(err)
	}
	result, err := s.aiClient.Complete(ctx, ai.Request{Task: ai.TaskWorkoutPlan, SystemPrompt: systemPrompt, UserPrompt: userPrompt, JSONMode: true})
	if err != nil {
		return WorkoutPlan{}, ai.ToAppError(err)
	}
	var refined WorkoutPlan
	if err := json.Unmarshal([]byte(prompt.JSONText(result)), &refined); err != nil || len(refined.Exercises) != len(base.Exercises) || refined.WorkoutTitle == "" {
		return WorkoutPlan{}, apperr.Wrap(err, 502, apperr.CodeAIUnavailable, "AIのワークアウト計画を解析できません。")
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

func (s *Service) normalizeDaily(ctx context.Context, userID int, plan *WorkoutPlan) error {
	plan.WorkoutTitle = strings.TrimSpace(plan.WorkoutTitle)
	if plan.WorkoutTitle == "" {
		plan.WorkoutTitle = "筋トレ"
	}
	plan.Target = strings.TrimSpace(plan.Target)
	if plan.Target == "" {
		plan.Target = plan.WorkoutTitle
	}
	if len(plan.Exercises) == 0 {
		return apperr.Validation("少なくとも1種目は選択してください。", map[string]string{"exercises": "required"})
	}
	for index := range plan.Exercises {
		exercise := &plan.Exercises[index]
		if strings.TrimSpace(exercise.ExerciseID) == "" {
			return apperr.Validation("種目IDが必要です。", map[string]string{"exercises": "exercise_id is required"})
		}
		stats, resolvedID, err := s.repository.ResolveExercise(ctx, userID, exercise.ExerciseID, exercise.Name)
		if errors.Is(err, ErrExerciseNotFound) {
			return apperr.NotFound("計画内の種目が辞書にありません。")
		}
		if err != nil {
			return apperr.Internal(err)
		}
		exercise.ExerciseID, exercise.Name, exercise.LastMaxWeight = resolvedID, stats.Name, stats.MaxWeight
		if exercise.PlannedSets <= 0 {
			exercise.PlannedSets = 3
		}
		if exercise.TargetReps <= 0 {
			exercise.TargetReps = 10
		}
		if exercise.TargetWeight < 0 {
			exercise.TargetWeight = 0
		}
	}
	if plan.EstimatedDurationMin <= 0 {
		plan.EstimatedDurationMin = len(plan.Exercises) * 12
	}
	if plan.EstimatedDurationMin < 15 {
		plan.EstimatedDurationMin = 15
	}
	if strings.TrimSpace(plan.CoachNote) == "" {
		plan.CoachNote = "カレンダーで調整した予定です。当日の体調に合わせて無理なく進めます。"
	}
	return nil
}

func (s *Service) parseDate(value string) (time.Time, error) {
	date, err := time.ParseInLocation("2006-01-02", value, s.clock.Now().Location())
	if err != nil {
		return time.Time{}, apperr.Validation("日付はYYYY-MM-DD形式で指定してください。", nil)
	}
	return date, nil
}

func (s *Service) optionalAIContext(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := s.aiTimeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func ValidateMonth(value string) error {
	if _, err := time.Parse("2006-01", value); err != nil {
		return apperr.Validation("月はYYYY-MM形式で指定してください。", nil)
	}
	return nil
}

func isNotFound(err error) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Status == http.StatusNotFound
}

func sameDate(left, right time.Time) bool {
	leftYear, leftMonth, leftDay := left.Date()
	rightYear, rightMonth, rightDay := right.In(left.Location()).Date()
	return leftYear == rightYear && leftMonth == rightMonth && leftDay == rightDay
}
