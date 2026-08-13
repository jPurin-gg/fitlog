package reporting

import (
	"context"
	"fmt"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
)

type Stat struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Unit  string `json:"unit"`
	Trend string `json:"trend"`
}

type RecentWorkout struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	Duration string `json:"duration"`
	Calories string `json:"calories"`
	Time     string `json:"time"`
}

type Dashboard struct {
	Stats          []Stat          `json:"stats"`
	ChartData      []int           `json:"chart_data"`
	RecentWorkouts []RecentWorkout `json:"recent_workouts"`
}

type WorkoutRow struct {
	ID      int
	Started time.Time
	Ended   time.Time
	Notes   string
}

type WorkedOutDay struct {
	Date      int    `json:"date"`
	WorkoutID int    `json:"workout_id"`
	Type      string `json:"type"`
}

type PlannedDay struct {
	Date   int    `json:"date"`
	PlanID int    `json:"plan_id"`
	Target string `json:"target"`
}

type Calendar struct {
	WorkedOutDates []int          `json:"worked_out_dates"`
	WorkedOutDays  []WorkedOutDay `json:"worked_out_days"`
	PlannedDays    []PlannedDay   `json:"planned_days"`
}

type Repository interface {
	CloseStaleWorkouts(ctx context.Context, userID int, today time.Time) error
	SetCounts(ctx context.Context, userID int, start, end time.Time) (map[string]int, error)
	RecentWorkouts(ctx context.Context, userID int, limit int) ([]WorkoutRow, error)
	Calendar(ctx context.Context, userID, year, month int, timezone string) (Calendar, error)
}

type Service struct {
	repository Repository
	clock      clock.Clock
}

func NewService(repository Repository, appClock clock.Clock) *Service {
	return &Service{repository: repository, clock: appClock}
}

func (s *Service) Dashboard(ctx context.Context, userID int) (Dashboard, error) {
	now := s.clock.Now()
	if err := s.repository.CloseStaleWorkouts(ctx, userID, now); err != nil {
		return Dashboard{}, apperr.Internal(err)
	}
	start := beginningOfDay(now).AddDate(0, 0, -6)
	end := beginningOfDay(now).AddDate(0, 0, 1)
	counts, err := s.repository.SetCounts(ctx, userID, start, end)
	if err != nil {
		return Dashboard{}, apperr.Internal(err)
	}
	chart := make([]int, 7)
	for index := range chart {
		key := start.AddDate(0, 0, index).Format("2006-01-02")
		chart[index] = counts[key]
	}
	rows, err := s.repository.RecentWorkouts(ctx, userID, 3)
	if err != nil {
		return Dashboard{}, apperr.Internal(err)
	}
	recent := make([]RecentWorkout, 0, len(rows))
	for _, row := range rows {
		duration := row.Ended.Sub(row.Started).Minutes()
		if duration < 1 {
			duration = 1
		}
		workoutType := DisplayWorkoutType(row.Notes)
		recent = append(recent, RecentWorkout{ID: row.ID, Title: workoutType + "の記録", Type: workoutType, Duration: fmt.Sprintf("%.0f分", duration), Calories: fmt.Sprintf("%.0f kcal", duration*5), Time: row.Started.In(now.Location()).Format("15:04")})
	}
	return Dashboard{
		Stats: []Stat{
			{Label: "消費カロリー", Value: "2,450", Unit: "kcal", Trend: "+12%"},
			{Label: "心拍数", Value: "72", Unit: "bpm", Trend: "-2%"},
			{Label: "活動時間", Value: "45", Unit: "分", Trend: "+5分"},
			{Label: "体重", Value: "68.5", Unit: "kg", Trend: "-0.5kg"},
		},
		ChartData: chart, RecentWorkouts: recent,
	}, nil
}

func (s *Service) Calendar(ctx context.Context, userID, year, month int) (Calendar, error) {
	now := s.clock.Now()
	if year == 0 {
		year = now.Year()
	}
	if month == 0 {
		month = int(now.Month())
	}
	if month < 1 || month > 12 || year < 2000 || year > 2200 {
		return Calendar{}, apperr.Validation("年月の指定が不正です。", nil)
	}
	if err := s.repository.CloseStaleWorkouts(ctx, userID, now); err != nil {
		return Calendar{}, apperr.Internal(err)
	}
	calendar, err := s.repository.Calendar(ctx, userID, year, month, now.Location().String())
	if err != nil {
		return Calendar{}, apperr.Internal(err)
	}
	return calendar, nil
}

func beginningOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func DisplayWorkoutType(value string) string {
	switch value {
	case "PPL法 (Push/Pull/Legs)":
		return "PPL法（押す・引く・脚）"
	case "Full Body":
		return "全身"
	case "Push (胸・肩・三頭)":
		return "押す日（胸・肩・三頭）"
	case "Pull (背中・二頭)":
		return "引く日（背中・二頭）"
	case "Legs (脚・腹)":
		return "脚の日（脚・腹）"
	default:
		if value == "" {
			return "ワークアウト"
		}
		return value
	}
}
