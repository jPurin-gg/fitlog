//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

func TestDetailListsSetsInRecordedOrder(t *testing.T) {
	fixture := newIntegrationFixture(t)
	recorded := fixture.createRecordedWorkout(t, time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC))

	detail, err := fixture.repository.Detail(context.Background(), fixture.userID, recorded.id)
	if err != nil || len(detail.Sets) != 2 {
		t.Fatalf("Detail() = (%+v, %v), want two sets", detail, err)
	}
	if set := detail.Sets[0]; set.ID != recorded.setIDs[0] || set.ExerciseID != fixture.exerciseID || set.ExerciseName != "Integration Exercise" || set.SetOrder != 1 || set.Weight != 60 || set.Reps != 10 || set.Feeling != "普通" || !set.IsPR || set.CreatedAt == "" {
		t.Fatalf("Detail() first set = %+v", set)
	}
	if set := detail.Sets[1]; set.ID != recorded.setIDs[1] || set.SetOrder != 2 || set.Weight != 50 || set.Reps != 8 || set.Feeling != "重い" || set.IsPR {
		t.Fatalf("Detail() second set = %+v", set)
	}
}

func TestDeleteWorkoutRecalculatesStatsAndReleasesPlans(t *testing.T) {
	fixture := newIntegrationFixture(t)
	ctx := context.Background()
	older := fixture.saveCalendarWorkout(t, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), 60)
	newer := fixture.saveCalendarWorkout(t, time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC), 80)
	fixture.assertStats(t, fixture.exerciseID, 80, 80)
	completedPlanID := fixture.createPlan(t, newer, "2026-08-22", "completed")

	var otherUserID int
	if err := fixture.db.QueryRow(`INSERT INTO users (username,password_hash) VALUES ('delete_other_user','test') RETURNING id`).Scan(&otherUserID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Delete(ctx, otherUserID, newer); !errors.Is(err, workout.ErrNotFound) {
		t.Fatalf("other user Delete() error = %v, want ErrNotFound", err)
	}

	if err := fixture.repository.Delete(ctx, fixture.userID, newer); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := fixture.repository.Detail(ctx, fixture.userID, newer); !errors.Is(err, workout.ErrNotFound) {
		t.Fatalf("Detail() after delete error = %v, want ErrNotFound", err)
	}
	if err := fixture.repository.Delete(ctx, fixture.userID, newer); !errors.Is(err, workout.ErrNotFound) {
		t.Fatalf("second Delete() error = %v, want ErrNotFound", err)
	}
	if count := fixture.count(t, `SELECT COUNT(*) FROM workout_sets WHERE workout_id=$1`, newer); count != 0 {
		t.Fatalf("sets left after delete = %d", count)
	}
	// 実施済みの予定は記録と一緒に消え、同じ日を記録し直しても完了済み予定と衝突しない。
	if count := fixture.count(t, `SELECT COUNT(*) FROM workout_plans WHERE id=$1`, completedPlanID); count != 0 {
		t.Fatalf("completed plans left after delete = %d", count)
	}
	// 最新重量・最大重量は、残っている古い記録から再計算する。
	fixture.assertStats(t, fixture.exerciseID, 60, 60)

	// 進行中のワークアウトを消したときは、予定を残して紐付けだけ外す。
	active := fixture.createWorkout(t)
	activePlanID := fixture.createPlan(t, active, "2026-08-23", "active")
	if err := fixture.repository.Delete(ctx, fixture.userID, active); err != nil {
		t.Fatalf("Delete() of active workout error = %v", err)
	}
	var linked sql.NullInt64
	var status string
	if err := fixture.db.QueryRow(`SELECT workout_id,status FROM workout_plans WHERE id=$1`, activePlanID).Scan(&linked, &status); err != nil {
		t.Fatal(err)
	}
	if linked.Valid || status != "active" {
		t.Fatalf("active plan after delete: workout_id=%v status=%q, want unlinked active plan", linked, status)
	}

	if err := fixture.repository.Delete(ctx, fixture.userID, older); err != nil {
		t.Fatalf("Delete() of last workout error = %v", err)
	}
	var latestWeight, maxWeight sql.NullFloat64
	if err := fixture.db.QueryRow(`SELECT weight,max_weight FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2`, fixture.userID, fixture.exerciseID).Scan(&latestWeight, &maxWeight); err != nil {
		t.Fatal(err)
	}
	if latestWeight.Valid || maxWeight.Valid {
		t.Fatalf("stats without records: weight=%v max_weight=%v, want both cleared", latestWeight, maxWeight)
	}
}

func TestHistoryPagesCompletedWorkoutsNewestFirst(t *testing.T) {
	fixture := newIntegrationFixture(t)
	ctx := context.Background()
	first := fixture.saveCalendarWorkout(t, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), 40)
	second := fixture.saveCalendarWorkout(t, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC), 50)
	third := fixture.saveCalendarWorkout(t, time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC), 60)
	fixture.createWorkout(t) // 進行中の記録は履歴に含めない。

	page, total, err := fixture.repository.History(ctx, fixture.userID, 2, 0)
	if err != nil || total != 3 || len(page) != 2 || page[0].ID != third || page[1].ID != second {
		t.Fatalf("History(2, 0) = (%+v, %d, %v), want the two newest of three", page, total, err)
	}
	if entry := page[0]; entry.Title != "あとから記録" || entry.TotalSets != 1 || entry.TotalVolume != 600 || entry.DurationMin != 18 || entry.StartedAt == "" || entry.EndedAt == "" {
		t.Fatalf("History() entry = %+v", entry)
	}
	page, total, err = fixture.repository.History(ctx, fixture.userID, 2, 2)
	if err != nil || total != 3 || len(page) != 1 || page[0].ID != first {
		t.Fatalf("History(2, 2) = (%+v, %d, %v), want the oldest workout", page, total, err)
	}

	var otherUserID int
	if err := fixture.db.QueryRow(`INSERT INTO users (username,password_hash) VALUES ('history_other_user','test') RETURNING id`).Scan(&otherUserID); err != nil {
		t.Fatal(err)
	}
	page, total, err = fixture.repository.History(ctx, otherUserID, 20, 0)
	if err != nil || total != 0 || len(page) != 0 {
		t.Fatalf("other user History() = (%+v, %d, %v), want nothing", page, total, err)
	}
}

// saveCalendarWorkout は、記録のない日にカレンダーから1セット（10回）の記録を作る。
func (f integrationFixture) saveCalendarWorkout(t *testing.T, date time.Time, weight float64) int {
	t.Helper()
	saved, err := f.repository.SaveCalendarWorkout(context.Background(), f.userID, date, workout.CalendarWorkoutInput{
		Title: "あとから記録",
		Sets:  []workout.CalendarSet{{ExerciseID: f.exerciseID, Weight: weight, Reps: 10, SetOrder: 1}},
	})
	if err != nil {
		t.Fatalf("SaveCalendarWorkout(%s) error = %v", date.Format("2006-01-02"), err)
	}
	return saved.WorkoutID
}

func (f integrationFixture) createPlan(t *testing.T, workoutID int, planDate, status string) int {
	t.Helper()
	var id int
	if err := f.db.QueryRow(`
		INSERT INTO workout_plans (workout_id,user_id,plan_date,title,status,plan)
		VALUES ($1,$2,$3::date,'Integration Plan',$4,'{}'::jsonb) RETURNING id
	`, workoutID, f.userID, planDate, status).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f integrationFixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var count int
	if err := f.db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
