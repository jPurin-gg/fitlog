package reporting

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
)

var tokyo = time.FixedZone("JST", 9*60*60)

type stubRepository struct {
	setCounts            map[string]int
	weekWorkouts         int
	weekPlanned          int
	weekErr              error
	weekStart, weekEnd   time.Time
	chartStart, chartEnd time.Time
	weekProgressCalls    int
	recentWorkouts       []WorkoutRow
}

func (r *stubRepository) CloseStaleWorkouts(context.Context, int, time.Time) error { return nil }

func (r *stubRepository) SetCounts(_ context.Context, _ int, start, end time.Time) (map[string]int, error) {
	r.chartStart, r.chartEnd = start, end
	return r.setCounts, nil
}

func (r *stubRepository) RecentWorkouts(context.Context, int, int) ([]WorkoutRow, error) {
	return r.recentWorkouts, nil
}

func (r *stubRepository) WeekProgress(_ context.Context, _ int, start, end time.Time) (int, int, error) {
	r.weekProgressCalls++
	r.weekStart, r.weekEnd = start, end
	return r.weekWorkouts, r.weekPlanned, r.weekErr
}

func (r *stubRepository) Calendar(context.Context, int, int, int, string) (Calendar, error) {
	return Calendar{}, nil
}

func TestDashboardChartCoversLastSevenDaysWithDates(t *testing.T) {
	repository := &stubRepository{setCounts: map[string]int{"2026-11-07": 4, "2026-11-11": 9, "2026-11-13": 2}}
	service := NewService(repository, clock.Fixed{Time: time.Date(2026, 11, 13, 10, 0, 0, 0, tokyo)})

	dashboard, err := service.Dashboard(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{4, 0, 0, 0, 9, 0, 2}; !reflect.DeepEqual(dashboard.ChartData, want) {
		t.Fatalf("ChartData = %v, want %v", dashboard.ChartData, want)
	}
	wantDates := []string{"2026-11-07", "2026-11-08", "2026-11-09", "2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13"}
	if !reflect.DeepEqual(dashboard.ChartDates, wantDates) {
		t.Fatalf("ChartDates = %v, want %v", dashboard.ChartDates, wantDates)
	}
	if !repository.chartStart.Equal(time.Date(2026, 11, 7, 0, 0, 0, 0, tokyo)) || !repository.chartEnd.Equal(time.Date(2026, 11, 14, 0, 0, 0, 0, tokyo)) {
		t.Fatalf("SetCounts range = [%v, %v)", repository.chartStart, repository.chartEnd)
	}
}

func TestDashboardWithoutRecordsKeepsSevenZeroBars(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository, clock.Fixed{Time: time.Date(2026, 11, 13, 10, 0, 0, 0, tokyo)})

	dashboard, err := service.Dashboard(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(dashboard)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded["chart_data"]; !reflect.DeepEqual(got, []any{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}) {
		t.Fatalf("chart_data = %#v, want seven zeros", got)
	}
	if got, ok := decoded["chart_dates"].([]any); !ok || len(got) != 7 {
		t.Fatalf("chart_dates = %#v, want seven dates", decoded["chart_dates"])
	}
	want := map[string]any{"week_start": "2026-11-09", "workout_count": 0.0, "planned_count": 0.0}
	if got := decoded["this_week"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("this_week = %#v, want %#v", got, want)
	}
}

func TestDashboardWeekStartsOnMondayInAppTimezone(t *testing.T) {
	tests := []struct {
		name      string
		now       time.Time
		weekStart time.Time
	}{
		{name: "friday", now: time.Date(2026, 11, 13, 10, 0, 0, 0, tokyo), weekStart: time.Date(2026, 11, 9, 0, 0, 0, 0, tokyo)},
		{name: "monday just after midnight", now: time.Date(2026, 11, 9, 0, 5, 0, 0, tokyo), weekStart: time.Date(2026, 11, 9, 0, 0, 0, 0, tokyo)},
		{name: "sunday night belongs to the same week", now: time.Date(2026, 11, 15, 23, 59, 0, 0, tokyo), weekStart: time.Date(2026, 11, 9, 0, 0, 0, 0, tokyo)},
		{name: "week across a month and year boundary", now: time.Date(2027, 1, 2, 9, 0, 0, 0, tokyo), weekStart: time.Date(2026, 12, 28, 0, 0, 0, 0, tokyo)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &stubRepository{weekWorkouts: 2, weekPlanned: 3}
			service := NewService(repository, clock.Fixed{Time: test.now})
			dashboard, err := service.Dashboard(context.Background(), 1)
			if err != nil {
				t.Fatal(err)
			}
			if !repository.weekStart.Equal(test.weekStart) || !repository.weekEnd.Equal(test.weekStart.AddDate(0, 0, 7)) {
				t.Fatalf("WeekProgress range = [%v, %v), want week from %v", repository.weekStart, repository.weekEnd, test.weekStart)
			}
			want := WeekProgress{WeekStart: test.weekStart.Format("2006-01-02"), WorkoutCount: 2, PlannedCount: 3}
			if dashboard.ThisWeek != want {
				t.Fatalf("ThisWeek = %+v, want %+v", dashboard.ThisWeek, want)
			}
		})
	}
}

func TestDashboardWeekProgressFailureIsInternalError(t *testing.T) {
	repository := &stubRepository{weekErr: errors.New("db down")}
	service := NewService(repository, clock.Fixed{Time: time.Date(2026, 11, 13, 10, 0, 0, 0, tokyo)})
	_, err := service.Dashboard(context.Background(), 1)
	if appErr := apperr.As(err); appErr == nil || appErr.Status != 500 {
		t.Fatalf("Dashboard() error = %#v, want internal error", err)
	}
}
