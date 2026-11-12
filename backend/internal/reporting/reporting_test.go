package reporting

import (
	"context"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
)

func TestDashboardReturnsRecentWorkoutDateTimes(t *testing.T) {
	tokyo := time.FixedZone("JST", 9*60*60)
	// UTCでは前日でも、アプリのタイムゾーンでは8月25日の朝の記録。
	started := time.Date(2026, 8, 24, 22, 30, 0, 0, time.UTC)
	repository := &stubRepository{recent: []WorkoutRow{{ID: 7, Started: started, Ended: started.Add(45 * time.Minute), Notes: "Full Body"}}}
	service := NewService(repository, clock.Fixed{Time: time.Date(2026, 8, 26, 9, 0, 0, 0, tokyo)})

	dashboard, err := service.Dashboard(context.Background(), 1)
	if err != nil || len(dashboard.RecentWorkouts) != 1 {
		t.Fatalf("Dashboard() = %#v, %v", dashboard, err)
	}
	recent := dashboard.RecentWorkouts[0]
	if recent.ID != 7 || recent.Title != "全身の記録" || recent.Duration != "45分" {
		t.Fatalf("recent workout = %#v", recent)
	}
	// 時刻だけでは何日の記録か分からないため、日付を含む日時をアプリのタイムゾーンで返す。
	if recent.StartedAt != "2026-08-25T07:30:00+09:00" || recent.EndedAt != "2026-08-25T08:15:00+09:00" {
		t.Fatalf("recent workout times = %q - %q", recent.StartedAt, recent.EndedAt)
	}
	if repository.recentLimit != 3 {
		t.Fatalf("RecentWorkouts() limit = %d, want 3", repository.recentLimit)
	}
}

type stubRepository struct {
	recent      []WorkoutRow
	recentLimit int
}

func (*stubRepository) CloseStaleWorkouts(context.Context, int, time.Time) error { return nil }

func (*stubRepository) SetCounts(context.Context, int, time.Time, time.Time) (map[string]int, error) {
	return map[string]int{}, nil
}

func (r *stubRepository) RecentWorkouts(_ context.Context, _ int, limit int) ([]WorkoutRow, error) {
	r.recentLimit = limit
	return r.recent, nil
}

func (*stubRepository) Calendar(context.Context, int, int, int, string) (Calendar, error) {
	return Calendar{}, nil
}
