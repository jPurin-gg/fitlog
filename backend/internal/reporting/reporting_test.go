package reporting

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
	"github.com/jPurin-gg/myfitlog-backend/internal/requestctx"
)

func TestReadsContinueWhenStaleWorkoutCleanupFails(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	for _, endpoint := range []string{"dashboard", "calendar"} {
		for _, cleanupFails := range []bool{false, true} {
			name := endpoint + "/cleanup succeeds"
			if cleanupFails {
				name = endpoint + "/cleanup fails"
			}
			t.Run(name, func(t *testing.T) {
				repository := &stubReportingRepository{now: now}
				if cleanupFails {
					repository.closeErr = errors.New("constraint violation containing private record values")
				}
				var logs bytes.Buffer
				service := NewService(repository, clock.Fixed{Time: now}).WithLogger(slog.New(slog.NewJSONHandler(&logs, nil)))
				ctx := requestctx.WithRequestID(context.Background(), "cleanup-request")
				if endpoint == "dashboard" {
					result, err := service.Dashboard(ctx, 1)
					if err != nil || !reflect.DeepEqual(result.ChartData, []int{0, 0, 0, 0, 0, 2, 0}) || len(result.RecentWorkouts) != 1 || result.RecentWorkouts[0].ID != 9 {
						t.Fatalf("Dashboard() = %#v, %v", result, err)
					}
				} else {
					result, err := service.Calendar(ctx, 1, 2026, 8)
					if err != nil || len(result.WorkedOutDays) != 1 || result.WorkedOutDays[0].WorkoutID != 9 {
						t.Fatalf("Calendar() = %#v, %v", result, err)
					}
				}
				if repository.closeCalls != 1 {
					t.Fatalf("CloseStaleWorkouts calls = %d", repository.closeCalls)
				}
				if cleanupFails {
					var record map[string]any
					if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
						t.Fatal(err)
					}
					if record["level"] != "ERROR" || record["request_id"] != "cleanup-request" || strings.Contains(logs.String(), "private record values") {
						t.Fatalf("cleanup log = %s", logs.String())
					}
				} else if logs.Len() != 0 {
					t.Fatalf("unexpected log = %s", logs.String())
				}
			})
		}
	}
}

func TestReadFailuresStillReturnInternalErrors(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	for _, operation := range []string{"counts", "history", "calendar"} {
		t.Run(operation, func(t *testing.T) {
			repository := &stubReportingRepository{now: now, closeErr: errors.New("cleanup failed"), failedRead: operation}
			service := NewService(repository, clock.Fixed{Time: now})
			var err error
			if operation == "calendar" {
				_, err = service.Calendar(context.Background(), 1, 2026, 8)
			} else {
				_, err = service.Dashboard(context.Background(), 1)
			}
			if err == nil || apperr.As(err).Status != 500 {
				t.Fatalf("read error = %v, want internal error", err)
			}
		})
	}
}

type stubReportingRepository struct {
	now        time.Time
	closeErr   error
	closeCalls int
	failedRead string
}

func (r *stubReportingRepository) CloseStaleWorkouts(context.Context, int, time.Time) error {
	r.closeCalls++
	return r.closeErr
}

func (r *stubReportingRepository) SetCounts(context.Context, int, time.Time, time.Time) (map[string]int, error) {
	if r.failedRead == "counts" {
		return nil, errors.New("count query failed")
	}
	return map[string]int{"2026-08-24": 2}, nil
}

func (r *stubReportingRepository) RecentWorkouts(context.Context, int, int) ([]WorkoutRow, error) {
	if r.failedRead == "history" {
		return nil, errors.New("history query failed")
	}
	return []WorkoutRow{{ID: 9, Started: r.now.Add(-24 * time.Hour), Ended: r.now.Add(-23 * time.Hour), Notes: "胸"}}, nil
}

func (r *stubReportingRepository) Calendar(context.Context, int, int, int, string) (Calendar, error) {
	if r.failedRead == "calendar" {
		return Calendar{}, errors.New("calendar query failed")
	}
	return Calendar{WorkedOutDates: []int{24}, WorkedOutDays: []WorkedOutDay{{Date: 24, WorkoutID: 9, Type: "胸"}}}, nil
}
