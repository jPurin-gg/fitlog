package planning

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
)

func TestTodayUsesAppTimezoneDateForPlanAndStart(t *testing.T) {
	tokyo := time.FixedZone("Asia/Tokyo", 9*60*60)
	// 01:30 in Tokyo is still the previous day in UTC and in America/Los_Angeles.
	now := time.Date(2026, 8, 24, 16, 30, 0, 0, time.UTC).In(tokyo)
	repository := newStartDailyRepository(now)
	service := NewService(repository, nil, nil, nil, clock.Fixed{Time: now}, time.Second)

	today, err := service.Today(context.Background(), 1)
	if err != nil {
		t.Fatalf("Today() error = %v", err)
	}
	if today.Date != "2026-08-25" || today.Timezone != "Asia/Tokyo" {
		t.Fatalf("Today() date = %q, timezone = %q", today.Date, today.Timezone)
	}
	wantDate := time.Date(2026, 8, 25, 0, 0, 0, 0, tokyo)
	if len(repository.dailyDates) != 1 || !repository.dailyDates[0].Equal(wantDate) {
		t.Fatalf("Daily() dates = %v, want %v", repository.dailyDates, wantDate)
	}
	if len(repository.workoutDates) != 1 || !repository.workoutDates[0].Equal(wantDate) {
		t.Fatalf("WorkoutOn() dates = %v, want %v", repository.workoutDates, wantDate)
	}

	enabled := false
	if _, err := service.StartDaily(context.Background(), 1, "2026-08-24", StartDailyInput{UseAI: &enabled}); apperr.As(err).Status != 400 {
		t.Fatalf("StartDaily(browser date) error = %v, want 400", err)
	}
	if _, err := service.StartDaily(context.Background(), 1, today.Date, StartDailyInput{UseAI: &enabled}); err != nil {
		t.Fatalf("StartDaily(Today().Date) error = %v", err)
	}
}

func TestTodayPrefersSavedDailyPlanOverMonthlyRestDay(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repository := newStartDailyRepository(now)
	repository.monthlyResult = restDayMonthly(now)
	saved := WorkoutPlan{
		WorkoutTitle: "カレンダーで編集", Target: "背中", EstimatedDurationMin: 40,
		Exercises: []PlanExercise{{ExerciseID: "row", Name: "ベントオーバーロウ", PlannedSets: 4, TargetWeight: 55, TargetReps: 8}},
	}
	repository.dailyErr = nil
	repository.dailyResult = PlanSession{ID: 7, PlanDate: "2026-08-24", Status: "active", Plan: saved}
	service := NewService(repository, nil, nil, nil, clock.Fixed{Time: now}, time.Second)

	today, err := service.Today(context.Background(), 1)
	if err != nil {
		t.Fatalf("Today() error = %v", err)
	}
	if today.RestDay || today.Plan == nil || today.Plan.Status != "active" || today.Plan.AIStatus != AIStatusNotRequested {
		t.Fatalf("Today() = %#v", today)
	}
	got := today.Plan.Plan.Exercises
	if len(got) != 1 || got[0].ExerciseID != "row" || got[0].PlannedSets != 4 || got[0].TargetWeight != 55 || got[0].TargetReps != 8 {
		t.Fatalf("Today() exercises = %#v", got)
	}
	if repository.monthlyCalls != 0 {
		t.Fatalf("monthly plan was consulted %d times despite a saved daily plan", repository.monthlyCalls)
	}
}

func TestTodayReturnsSavedDailyPlanWithoutMonthlyPlan(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repository := newStartDailyRepository(now)
	repository.monthlyErr = ErrNotFound
	repository.dailyErr = nil
	repository.dailyResult = PlanSession{ID: 7, PlanDate: "2026-08-24", Status: "active", Plan: repository.basePlan()}
	service := NewService(repository, nil, nil, nil, clock.Fixed{Time: now}, time.Second)

	today, err := service.Today(context.Background(), 1)
	if err != nil || today.Plan == nil || today.Plan.Status != "active" || today.RestDay {
		t.Fatalf("Today() = %#v, %v", today, err)
	}
}

func TestTodayBuildsDraftFromMonthlyPlan(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repository := newStartDailyRepository(now)
	service := NewService(repository, nil, nil, nil, clock.Fixed{Time: now}, time.Second)

	today, err := service.Today(context.Background(), 1)
	if err != nil {
		t.Fatalf("Today() error = %v", err)
	}
	if today.RestDay || today.Plan == nil || today.Plan.Status != "draft" || today.Plan.PlanDate != "2026-08-24" {
		t.Fatalf("Today() = %#v", today)
	}
	exercise := today.Plan.Plan.Exercises[0]
	if exercise.ExerciseID != "bench" || exercise.PlannedSets != 3 || exercise.TargetWeight != 40 || exercise.TargetReps != 10 {
		t.Fatalf("draft exercise = %#v", exercise)
	}
	if repository.saveCalls != 0 {
		t.Fatalf("Today() must not persist the draft; SaveDaily() calls = %d", repository.saveCalls)
	}
}

func TestTodayMarksMonthlyRestDayWithoutSavedPlan(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repository := newStartDailyRepository(now)
	repository.monthlyResult = restDayMonthly(now)
	service := NewService(repository, nil, nil, nil, clock.Fixed{Time: now}, time.Second)

	today, err := service.Today(context.Background(), 1)
	if err != nil || !today.RestDay {
		t.Fatalf("Today() = %#v, %v", today, err)
	}
}

func TestTodayWithoutAnyPlan(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	for name, mutate := range map[string]func(*startDailyRepository){
		"no monthly plan":           func(r *startDailyRepository) { r.monthlyErr = ErrNotFound },
		"no resolvable exercises":   func(r *startDailyRepository) { r.resolveErr = ErrExerciseNotFound },
		"monthly plan without days": func(r *startDailyRepository) { r.monthlyResult = &MonthlyPlan{} },
	} {
		t.Run(name, func(t *testing.T) {
			repository := newStartDailyRepository(now)
			mutate(repository)
			service := NewService(repository, nil, nil, nil, clock.Fixed{Time: now}, time.Second)

			today, err := service.Today(context.Background(), 1)
			if err != nil || today.Plan != nil || today.RestDay || today.Date != "2026-08-24" {
				t.Fatalf("Today() = %#v, %v", today, err)
			}
		})
	}
}

func TestTodayIncludesTodaysWorkout(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repository := newStartDailyRepository(now)
	repository.workoutErr = nil
	repository.workoutResult = TodayWorkout{ID: 9, Status: "active", StartedAt: now.Add(-time.Hour)}
	service := NewService(repository, nil, nil, nil, clock.Fixed{Time: now}, time.Second)

	today, err := service.Today(context.Background(), 1)
	if err != nil || today.Workout == nil || today.Workout.ID != 9 || today.Workout.Status != "active" {
		t.Fatalf("Today() = %#v, %v", today, err)
	}
}

func TestTodayDoesNotMaskStorageFailures(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	unavailable := errors.New("database unavailable")
	for name, mutate := range map[string]func(*startDailyRepository){
		"daily plan": func(r *startDailyRepository) { r.dailyErr = unavailable },
		"monthly":    func(r *startDailyRepository) { r.monthlyErr = unavailable },
		"exercise":   func(r *startDailyRepository) { r.resolveErr = unavailable },
		"workout":    func(r *startDailyRepository) { r.workoutErr = unavailable },
	} {
		t.Run(name, func(t *testing.T) {
			repository := newStartDailyRepository(now)
			mutate(repository)
			service := NewService(repository, nil, nil, nil, clock.Fixed{Time: now}, time.Second)

			if _, err := service.Today(context.Background(), 1); apperr.As(err).Status != 500 {
				t.Fatalf("Today() error = %v, want 500", err)
			}
		})
	}
}

func restDayMonthly(now time.Time) *MonthlyPlan {
	return &MonthlyPlan{
		RecommendedDays: []int{int((now.Weekday() + 1) % 7)},
		WeeklyRoutine: []DayRoutine{{
			DayName: "1日目", Target: "胸", ExampleExercises: []string{"ベンチプレス"}, ExerciseIDs: []string{"bench"},
		}},
	}
}
