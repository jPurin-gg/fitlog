package reporting

import (
	"context"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
)

type fakeRepository struct {
	minutesByStart map[string]float64
	summary        BodySummary
}

func (f *fakeRepository) CloseStaleWorkouts(context.Context, int, time.Time) error { return nil }

func (f *fakeRepository) SetCounts(context.Context, int, time.Time, time.Time) (map[string]int, error) {
	return map[string]int{}, nil
}

func (f *fakeRepository) RecentWorkouts(context.Context, int, int) ([]WorkoutRow, error) {
	return []WorkoutRow{}, nil
}

func (f *fakeRepository) WorkoutMinutes(_ context.Context, _ int, start, _ time.Time) (float64, error) {
	return f.minutesByStart[start.Format("2006-01-02")], nil
}

func (f *fakeRepository) BodySummary(context.Context, int) (BodySummary, error) {
	return f.summary, nil
}

func (f *fakeRepository) Calendar(context.Context, int, int, int, string) (Calendar, error) {
	return Calendar{}, nil
}

func floatPtr(value float64) *float64 { return &value }
func intPtr(value int) *int           { return &value }

func statByLabel(stats []Stat, label string) *Stat {
	for index := range stats {
		if stats[index].Label == label {
			return &stats[index]
		}
	}
	return nil
}

func TestDashboardStatsComeFromRecordedData(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		// Current 7-day window starts 2026-11-05, previous starts 2026-10-29.
		minutesByStart: map[string]float64{"2026-11-05": 200, "2026-10-29": 100},
		summary: BodySummary{
			WeightKg: floatPtr(68.5), PrevWeightKg: floatPtr(69.0),
			HeartRate: intPtr(72), PrevHeartRate: intPtr(75),
		},
	}
	service := NewService(repository, clock.Fixed{Time: now})
	dashboard, err := service.Dashboard(context.Background(), 1)
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if stat := statByLabel(dashboard.Stats, "消費カロリー"); stat == nil || stat.Value != "1,000" || stat.Trend != "+100%" {
		t.Fatalf("calories stat = %+v", stat)
	}
	if stat := statByLabel(dashboard.Stats, "活動時間"); stat == nil || stat.Value != "200" || stat.Trend != "+100分" {
		t.Fatalf("activity stat = %+v", stat)
	}
	if stat := statByLabel(dashboard.Stats, "体重"); stat == nil || stat.Value != "68.5" || stat.Trend != "-0.5kg" {
		t.Fatalf("weight stat = %+v", stat)
	}
	if stat := statByLabel(dashboard.Stats, "心拍数"); stat == nil || stat.Value != "72" || stat.Trend != "-3bpm" {
		t.Fatalf("heart rate stat = %+v", stat)
	}
}

func TestDashboardOmitsUnrecordedBodyStats(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	repository := &fakeRepository{minutesByStart: map[string]float64{}}
	service := NewService(repository, clock.Fixed{Time: now})
	dashboard, err := service.Dashboard(context.Background(), 1)
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if len(dashboard.Stats) != 2 {
		t.Fatalf("stats = %+v, want only workout-derived stats", dashboard.Stats)
	}
	if stat := statByLabel(dashboard.Stats, "体重"); stat != nil {
		t.Fatalf("weight stat must be omitted without measurements: %+v", stat)
	}
	if stat := statByLabel(dashboard.Stats, "心拍数"); stat != nil {
		t.Fatalf("heart rate stat must be omitted without measurements: %+v", stat)
	}
}
