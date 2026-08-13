package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/reporting"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CloseStaleWorkouts(ctx context.Context, userID int, today time.Time) error {
	dayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	_, err := r.db.ExecContext(ctx, `
		UPDATE workouts SET ended_at = COALESCE(
			(SELECT MAX(created_at) FROM workout_sets WHERE workout_id = workouts.id), started_at
		)
		WHERE user_id = $1 AND ended_at IS NULL AND started_at < $2
	`, userID, dayStart)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE workout_plans SET status = 'completed', updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND status = 'active' AND plan_date < $2::date
	`, userID, dayStart)
	return err
}

func (r *Repository) SetCounts(ctx context.Context, userID int, start, end time.Time) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT (ws.created_at AT TIME ZONE $4)::date::text, COUNT(*)::int
		FROM workout_sets ws JOIN workouts w ON w.id = ws.workout_id
		WHERE w.user_id = $1 AND ws.created_at >= $2 AND ws.created_at < $3
		GROUP BY 1
	`, userID, start, end, start.Location().String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]int{}
	for rows.Next() {
		var day string
		var count int
		if err := rows.Scan(&day, &count); err != nil {
			return nil, err
		}
		result[day] = count
	}
	return result, rows.Err()
}

func (r *Repository) RecentWorkouts(ctx context.Context, userID int, limit int) ([]reporting.WorkoutRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, started_at, ended_at, COALESCE(notes, '筋トレ')
		FROM workouts WHERE user_id = $1 AND ended_at IS NOT NULL
		ORDER BY ended_at DESC, started_at DESC LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []reporting.WorkoutRow{}
	for rows.Next() {
		var row reporting.WorkoutRow
		if err := rows.Scan(&row.ID, &row.Started, &row.Ended, &row.Notes); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Repository) Calendar(ctx context.Context, userID, year, month int, timezone string) (reporting.Calendar, error) {
	result := reporting.Calendar{WorkedOutDates: []int{}, WorkedOutDays: []reporting.WorkedOutDay{}, PlannedDays: []reporting.PlannedDay{}}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (EXTRACT(DAY FROM (started_at AT TIME ZONE $4)))
		       EXTRACT(DAY FROM (started_at AT TIME ZONE $4))::int, id, COALESCE(notes, 'ワークアウト')
		FROM workouts
		WHERE user_id = $1 AND ended_at IS NOT NULL
		  AND EXTRACT(YEAR FROM (started_at AT TIME ZONE $4)) = $2
		  AND EXTRACT(MONTH FROM (started_at AT TIME ZONE $4)) = $3
		ORDER BY EXTRACT(DAY FROM (started_at AT TIME ZONE $4)), started_at DESC, id DESC
	`, userID, year, month, timezone)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var day reporting.WorkedOutDay
		var workoutType string
		if err := rows.Scan(&day.Date, &day.WorkoutID, &workoutType); err != nil {
			rows.Close()
			return result, err
		}
		day.Type = reporting.DisplayWorkoutType(workoutType)
		result.WorkedOutDates = append(result.WorkedOutDates, day.Date)
		result.WorkedOutDays = append(result.WorkedOutDays, day)
	}
	if err := rows.Close(); err != nil {
		return result, err
	}
	planRows, err := r.db.QueryContext(ctx, `
		SELECT EXTRACT(DAY FROM plan_date)::int, id, COALESCE(NULLIF(plan->>'target', ''), title, '予定')
		FROM workout_plans
		WHERE user_id = $1 AND status = 'active' AND EXTRACT(YEAR FROM plan_date) = $2 AND EXTRACT(MONTH FROM plan_date) = $3
		ORDER BY plan_date, updated_at DESC, id DESC
	`, userID, year, month)
	if err != nil {
		return result, err
	}
	defer planRows.Close()
	for planRows.Next() {
		var day reporting.PlannedDay
		if err := planRows.Scan(&day.Date, &day.PlanID, &day.Target); err != nil {
			return result, err
		}
		day.Target = reporting.DisplayWorkoutType(day.Target)
		result.PlannedDays = append(result.PlannedDays, day)
	}
	return result, planRows.Err()
}
