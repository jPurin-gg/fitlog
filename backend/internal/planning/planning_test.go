package planning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/ai"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
	"github.com/jPurin-gg/myfitlog-backend/internal/profile"
	"github.com/jPurin-gg/myfitlog-backend/internal/requestctx"
)

func TestGenerateMonthlyLogsFinalFeatureOutcome(t *testing.T) {
	validPlan := MonthlyPlan{
		PlanName:        "週1回プラン",
		Frequency:       "週1回",
		Description:     "無理なく継続するプランです。",
		Rationale:       "初心者向けに負荷を調整します。",
		RecommendedDays: []int{1},
		WeeklyRoutine: []DayRoutine{{
			DayName: "月曜日", Target: "胸", ExampleExercises: []string{"AI name"}, ExerciseIDs: []string{"bench"},
		}},
	}
	validJSON, err := json.Marshal(validPlan)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name           string
		completion     string
		completionErr  error
		saveMonthlyErr error
		wantOutcome    string
		wantErr        bool
	}{
		{name: "applied", completion: string(validJSON), wantOutcome: "applied"},
		{name: "provider error", completionErr: errors.New("provider unavailable"), wantOutcome: "provider_error", wantErr: true},
		{name: "invalid output", completion: "not-json", wantOutcome: "invalid_output", wantErr: true},
		{name: "storage error", completion: string(validJSON), saveMonthlyErr: errors.New("database unavailable"), wantOutcome: "storage_error", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
			repository := newStartDailyRepository(now)
			repository.candidates = []Candidate{{ID: "bench", Name: "ベンチプレス", Equipment: "バーベル", PrimaryMuscles: []string{"胸"}}}
			repository.saveMonthlyErr = test.saveMonthlyErr
			aiClient := &stubPlanningAI{complete: func(context.Context, ai.Request) (string, error) {
				return test.completion, test.completionErr
			}}
			service := NewService(repository, stubPlanningPreferences{}, aiClient, stubPlanningPrompts{}, clock.Fixed{Time: now}, time.Second).WithLogger(logger)
			ctx := requestctx.WithRequestID(context.Background(), "planning-request")

			_, err := service.GenerateMonthly(ctx, 1, "2026-08", GenerateMonthlyInput{Frequency: "週1回"})
			if (err != nil) != test.wantErr {
				t.Fatalf("GenerateMonthly() error = %v, wantErr %v", err, test.wantErr)
			}

			record := decodePlanningFeatureLog(t, output.Bytes())
			if record["stage"] != "feature" || record["request_id"] != "planning-request" || record["task"] != string(ai.TaskMonthlyPlan) || record["outcome"] != test.wantOutcome {
				t.Fatalf("feature log = %#v", record)
			}
			if _, ok := record["total_duration_ms"]; !ok {
				t.Fatal("total_duration_ms is missing")
			}
		})
	}
}

func decodePlanningFeatureLog(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &record); err != nil {
		t.Fatalf("decode feature log: %v; output = %q", err, string(data))
	}
	return record
}

func TestStartDailyAppliesAIRefinement(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repository := newStartDailyRepository(now)
	refined := repository.basePlan()
	refined.WorkoutTitle = "AI調整済み"
	refined.CoachNote = "体調に合わせたメニューです。"
	encoded, err := json.Marshal(refined)
	if err != nil {
		t.Fatal(err)
	}
	aiClient := &stubPlanningAI{complete: func(_ context.Context, request ai.Request) (string, error) {
		if request.Task != ai.TaskWorkoutPlan {
			t.Fatalf("AI task = %q", request.Task)
		}
		return string(encoded), nil
	}}
	service := NewService(repository, nil, aiClient, stubPlanningPrompts{}, clock.Fixed{Time: now}, time.Second)

	result, err := service.StartDaily(context.Background(), 1, now.Format("2006-01-02"))
	if err != nil || result.AIStatus != AIStatusApplied {
		t.Fatalf("StartDaily() = %#v, %v", result, err)
	}
	if repository.savedPlan.WorkoutTitle != "AI調整済み" || repository.saveCalls != 1 {
		t.Fatalf("saved plan = %#v; calls = %d", repository.savedPlan, repository.saveCalls)
	}
}

func TestStartDailyFallsBackToBasePlanWhenAIFails(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repository := newStartDailyRepository(now)
	aiClient := &stubPlanningAI{complete: func(context.Context, ai.Request) (string, error) {
		return "", errors.New("AI unavailable")
	}}
	service := NewService(repository, nil, aiClient, stubPlanningPrompts{}, clock.Fixed{Time: now}, time.Second)

	result, err := service.StartDaily(context.Background(), 1, now.Format("2006-01-02"))
	if err != nil || result.AIStatus != AIStatusFallback {
		t.Fatalf("StartDaily() = %#v, %v", result, err)
	}
	if repository.savedPlan.WorkoutTitle != repository.basePlan().WorkoutTitle || repository.saveCalls != 1 {
		t.Fatalf("saved plan = %#v; calls = %d", repository.savedPlan, repository.saveCalls)
	}
	if repository.saveContextErr != nil {
		t.Fatalf("SaveDaily() context error = %v; parent context must remain usable", repository.saveContextErr)
	}
}

func TestStartDailyFallsBackAfterOptionalAITimeout(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repository := newStartDailyRepository(now)
	aiClient := &stubPlanningAI{complete: func(ctx context.Context, _ ai.Request) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}}
	service := NewService(repository, nil, aiClient, stubPlanningPrompts{}, clock.Fixed{Time: now}, time.Millisecond)

	result, err := service.StartDaily(context.Background(), 1, now.Format("2006-01-02"))
	if err != nil || result.AIStatus != AIStatusFallback {
		t.Fatalf("StartDaily() = %#v, %v", result, err)
	}
	if repository.saveCalls != 1 || repository.saveContextErr != nil {
		t.Fatalf("SaveDaily() calls = %d; context error = %v", repository.saveCalls, repository.saveContextErr)
	}
}

func TestStartDailyFallsBackWhenAIResponseIsInvalid(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repository := newStartDailyRepository(now)
	aiClient := &stubPlanningAI{complete: func(context.Context, ai.Request) (string, error) {
		return "not-json", nil
	}}
	service := NewService(repository, nil, aiClient, stubPlanningPrompts{}, clock.Fixed{Time: now}, time.Second)

	result, err := service.StartDaily(context.Background(), 1, now.Format("2006-01-02"))
	if err != nil || result.AIStatus != AIStatusFallback {
		t.Fatalf("StartDaily() = %#v, %v", result, err)
	}
	if repository.saveCalls != 1 || repository.savedPlan.WorkoutTitle != repository.basePlan().WorkoutTitle {
		t.Fatalf("saved plan = %#v; calls = %d", repository.savedPlan, repository.saveCalls)
	}
}

func TestStartDailyDoesNotRequestAIForStoredPlan(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repository := newStartDailyRepository(now)
	repository.dailyErr = nil
	repository.dailyResult = PlanSession{ID: 7, PlanDate: now.Format("2006-01-02"), Status: "active", Plan: repository.basePlan()}
	repository.attachResult = repository.dailyResult
	aiClient := &stubPlanningAI{complete: func(context.Context, ai.Request) (string, error) {
		t.Fatal("stored plans must not be refined again")
		return "", nil
	}}
	service := NewService(repository, nil, aiClient, stubPlanningPrompts{}, clock.Fixed{Time: now}, time.Second)

	result, err := service.StartDaily(context.Background(), 1, now.Format("2006-01-02"))
	if err != nil || result.AIStatus != AIStatusNotRequested {
		t.Fatalf("StartDaily() = %#v, %v", result, err)
	}
	if aiClient.calls != 0 || repository.saveCalls != 0 {
		t.Fatalf("AI calls = %d; SaveDaily() calls = %d", aiClient.calls, repository.saveCalls)
	}
}

func TestDailyAndSaveDailyMarkAINotRequested(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repository := newStartDailyRepository(now)
	repository.dailyErr = nil
	repository.dailyResult = PlanSession{ID: 7, PlanDate: now.Format("2006-01-02"), Status: "active", Plan: repository.basePlan()}
	service := NewService(repository, nil, nil, nil, clock.Fixed{Time: now}, time.Second)

	daily, err := service.Daily(context.Background(), 1, now.Format("2006-01-02"))
	if err != nil || daily.AIStatus != AIStatusNotRequested {
		t.Fatalf("Daily() = %#v, %v", daily, err)
	}
	saved, err := service.SaveDaily(context.Background(), 1, now.Format("2006-01-02"), repository.basePlan())
	if err != nil || saved.AIStatus != AIStatusNotRequested {
		t.Fatalf("SaveDaily() = %#v, %v", saved, err)
	}
}

type startDailyRepository struct {
	now            time.Time
	dailyResult    PlanSession
	dailyErr       error
	candidates     []Candidate
	saveMonthlyErr error
	savedPlan      WorkoutPlan
	saveCalls      int
	saveContextErr error
	attachResult   PlanSession
}

func newStartDailyRepository(now time.Time) *startDailyRepository {
	return &startDailyRepository{now: now, dailyErr: ErrNotFound}
}

func (r *startDailyRepository) basePlan() WorkoutPlan {
	return WorkoutPlan{
		WorkoutTitle:         "胸",
		Target:               "胸",
		EstimatedDurationMin: 30,
		CoachNote:            "月間プランを反映します。",
		Exercises: []PlanExercise{{
			ExerciseID: "bench", Name: "ベンチプレス", PlannedSets: 3, TargetWeight: 40, TargetReps: 10, LastMaxWeight: 50,
		}},
	}
}

func (r *startDailyRepository) Monthly(context.Context, int, string) (MonthlyPlan, error) {
	return MonthlyPlan{
		RecommendedDays: []int{int(r.now.Weekday())},
		WeeklyRoutine: []DayRoutine{{
			DayName: "1日目", Target: "胸", ExampleExercises: []string{"ベンチプレス"}, ExerciseIDs: []string{"bench"},
		}},
	}, nil
}

func (*startDailyRepository) MonthlyList(context.Context, int) ([]MonthlyPlan, error) {
	return nil, nil
}

func (r *startDailyRepository) SaveMonthly(_ context.Context, _ int, _ string, plan MonthlyPlan) (MonthlyPlan, error) {
	return plan, r.saveMonthlyErr
}

func (r *startDailyRepository) Candidates(context.Context, int) ([]Candidate, error) {
	return r.candidates, nil
}

func (r *startDailyRepository) Daily(context.Context, int, time.Time) (PlanSession, error) {
	return r.dailyResult, r.dailyErr
}

func (r *startDailyRepository) SaveDaily(ctx context.Context, _ int, date time.Time, plan WorkoutPlan) (PlanSession, error) {
	r.saveCalls++
	r.saveContextErr = ctx.Err()
	r.savedPlan = plan
	return PlanSession{ID: 7, PlanDate: date.Format("2006-01-02"), Status: "active", Plan: plan}, nil
}

func (r *startDailyRepository) AttachWorkout(context.Context, int, time.Time) (PlanSession, error) {
	if r.attachResult.ID != 0 {
		return r.attachResult, nil
	}
	return PlanSession{ID: 7, WorkoutID: 9, PlanDate: r.now.Format("2006-01-02"), Status: "active", Plan: r.savedPlan}, nil
}

func (*startDailyRepository) ResolveExercise(context.Context, int, string, string) (ExerciseStats, string, error) {
	return ExerciseStats{Name: "ベンチプレス", TargetSets: 3, TargetWeight: 40, TargetReps: 10, MaxWeight: 50}, "bench", nil
}

type stubPlanningAI struct {
	complete func(context.Context, ai.Request) (string, error)
	calls    int
}

func (c *stubPlanningAI) Complete(ctx context.Context, request ai.Request) (string, error) {
	c.calls++
	return c.complete(ctx, request)
}

type stubPlanningPrompts struct{}

func (stubPlanningPrompts) Pair(systemFilename, userFilename string, _ any) (string, string, error) {
	return systemFilename, userFilename, nil
}

type stubPlanningPreferences struct{}

func (stubPlanningPreferences) Get(context.Context, int) (profile.Preferences, error) {
	return profile.Preferences{}, nil
}
