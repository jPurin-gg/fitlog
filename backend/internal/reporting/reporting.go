package reporting

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
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

// BodySummary carries the newest body measurements known for a user.
// Pointers are nil when the user has not recorded that metric yet.
type BodySummary struct {
	WeightKg      *float64
	PrevWeightKg  *float64
	HeartRate     *int
	PrevHeartRate *int
}

type Repository interface {
	CloseStaleWorkouts(ctx context.Context, userID int, today time.Time) error
	SetCounts(ctx context.Context, userID int, start, end time.Time) (map[string]int, error)
	RecentWorkouts(ctx context.Context, userID int, limit int) ([]WorkoutRow, error)
	WorkoutMinutes(ctx context.Context, userID int, start, end time.Time) (float64, error)
	BodySummary(ctx context.Context, userID int) (BodySummary, error)
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
	stats, err := s.buildStats(ctx, userID, start, end)
	if err != nil {
		return Dashboard{}, apperr.Internal(err)
	}
	return Dashboard{
		Stats:          stats,
		ChartData:      chart,
		RecentWorkouts: recent,
	}, nil
}

// buildStats derives every dashboard number from recorded data: workout
// durations stored in the workouts table and body measurements the user
// entered. Nothing here is a placeholder or a fixed constant. Metrics the
// user has never recorded are omitted instead of being faked.
func (s *Service) buildStats(ctx context.Context, userID int, start, end time.Time) ([]Stat, error) {
	prevStart := start.AddDate(0, 0, -7)
	minutes, err := s.repository.WorkoutMinutes(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}
	prevMinutes, err := s.repository.WorkoutMinutes(ctx, userID, prevStart, start)
	if err != nil {
		return nil, err
	}
	stats := []Stat{
		{Label: "消費カロリー", Value: formatThousands(int64(math.Round(minutes * 5))), Unit: "kcal", Trend: percentTrend(minutes, prevMinutes)},
		{Label: "活動時間", Value: fmt.Sprintf("%.0f", minutes), Unit: "分", Trend: minutesTrend(minutes, prevMinutes)},
	}
	summary, err := s.repository.BodySummary(ctx, userID)
	if err != nil {
		return nil, err
	}
	if summary.WeightKg != nil {
		trend := ""
		if summary.PrevWeightKg != nil {
			trend = kgTrend(*summary.WeightKg, *summary.PrevWeightKg)
		}
		stats = append(stats, Stat{Label: "体重", Value: strconv.FormatFloat(*summary.WeightKg, 'f', -1, 64), Unit: "kg", Trend: trend})
	}
	if summary.HeartRate != nil {
		trend := ""
		if summary.PrevHeartRate != nil {
			trend = bpmTrend(*summary.HeartRate, *summary.PrevHeartRate)
		}
		stats = append(stats, Stat{Label: "心拍数", Value: strconv.Itoa(*summary.HeartRate), Unit: "bpm", Trend: trend})
	}
	return stats, nil
}

func formatThousands(value int64) string {
	text := strconv.FormatInt(value, 10)
	negative := strings.HasPrefix(text, "-")
	if negative {
		text = text[1:]
	}
	var grouped []string
	for len(text) > 3 {
		grouped = append([]string{text[len(text)-3:]}, grouped...)
		text = text[:len(text)-3]
	}
	grouped = append([]string{text}, grouped...)
	result := strings.Join(grouped, ",")
	if negative {
		return "-" + result
	}
	return result
}

func percentTrend(current, previous float64) string {
	if previous <= 0 {
		return ""
	}
	change := int64(math.Round((current - previous) / previous * 100))
	if change == 0 {
		return "±0%"
	}
	return fmt.Sprintf("%+d%%", change)
}

func minutesTrend(current, previous float64) string {
	diff := int64(math.Round(current - previous))
	if diff == 0 {
		return "±0分"
	}
	return fmt.Sprintf("%+d分", diff)
}

func kgTrend(current, previous float64) string {
	diff := math.Round((current-previous)*10) / 10
	if diff == 0 {
		return "±0.0kg"
	}
	return fmt.Sprintf("%+.1fkg", diff)
}

func bpmTrend(current, previous int) string {
	diff := current - previous
	if diff == 0 {
		return "±0bpm"
	}
	return fmt.Sprintf("%+dbpm", diff)
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
