package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/lib/pq"

	"github.com/jPurin-gg/myfitlog-backend/internal/planning"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Monthly(ctx context.Context, userID int, month string) (planning.MonthlyPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, plan_month, plan_name, frequency, COALESCE(description, ''), COALESCE(rationale, ''), rest_days, recommended_days, weekly_routine
		FROM monthly_plans WHERE user_id = $1 AND plan_month = $2
	`, userID, month)
	plan, err := scanMonthly(row)
	if errors.Is(err, sql.ErrNoRows) {
		return planning.MonthlyPlan{}, planning.ErrNotFound
	}
	return plan, err
}

func (r *Repository) MonthlyList(ctx context.Context, userID int) ([]planning.MonthlyPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, plan_month, plan_name, frequency, COALESCE(description, ''), COALESCE(rationale, ''), rest_days, recommended_days, weekly_routine
		FROM monthly_plans WHERE user_id = $1 ORDER BY plan_month DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []planning.MonthlyPlan{}
	for rows.Next() {
		plan, err := scanMonthly(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, plan)
	}
	return result, rows.Err()
}

func (r *Repository) SaveMonthly(ctx context.Context, userID int, month string, plan planning.MonthlyPlan) (planning.MonthlyPlan, error) {
	restDays, _ := json.Marshal(planning.NormalizeWeekdays(plan.RestDays))
	recommendedDays, _ := json.Marshal(planning.NormalizeWeekdays(plan.RecommendedDays))
	routine, err := json.Marshal(plan.WeeklyRoutine)
	if err != nil {
		return planning.MonthlyPlan{}, err
	}
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO monthly_plans (user_id, plan_month, plan_name, frequency, description, rationale, rest_days, recommended_days, weekly_routine)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9::jsonb)
		ON CONFLICT (user_id, plan_month) DO UPDATE SET
			plan_name=EXCLUDED.plan_name, frequency=EXCLUDED.frequency, description=EXCLUDED.description,
			rationale=EXCLUDED.rationale, rest_days=EXCLUDED.rest_days, recommended_days=EXCLUDED.recommended_days,
			weekly_routine=EXCLUDED.weekly_routine, updated_at=CURRENT_TIMESTAMP
		RETURNING id
	`, userID, month, plan.PlanName, plan.Frequency, plan.Description, plan.Rationale, string(restDays), string(recommendedDays), string(routine)).Scan(&plan.ID)
	if err != nil {
		return planning.MonthlyPlan{}, err
	}
	plan.PlanMonth = month
	plan.RestDays = planning.NormalizeWeekdays(plan.RestDays)
	plan.RecommendedDays = planning.NormalizeWeekdays(plan.RecommendedDays)
	return plan, nil
}

func (r *Repository) Candidates(ctx context.Context, userID int) ([]planning.Candidate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.id, e.name, COALESCE(e.equipment, ''), COALESCE(e.level, ''), COALESCE(e.category, ''), e.primary_muscles, (f.exercise_id IS NOT NULL)
		FROM exercises e
		LEFT JOIN user_favorite_exercises f ON f.exercise_id=e.id AND f.user_id=$1
		WHERE COALESCE(e.category, '') IN ('筋力トレーニング','strength')
		  AND COALESCE(e.level, '') IN ('初級','中級','beginner','intermediate','')
		  AND jsonb_array_length(COALESCE(e.primary_muscles, '[]'::jsonb)) > 0
		ORDER BY CASE WHEN f.exercise_id IS NULL THEN 1 ELSE 0 END,
		         CASE COALESCE(e.level,'') WHEN '初級' THEN 1 WHEN 'beginner' THEN 1 WHEN '中級' THEN 2 WHEN 'intermediate' THEN 2 ELSE 3 END,
		         e.name LIMIT 1000
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []planning.Candidate{}
	for rows.Next() {
		var candidate planning.Candidate
		var muscles []byte
		if err := rows.Scan(&candidate.ID, &candidate.Name, &candidate.Equipment, &candidate.Level, &candidate.Category, &muscles, &candidate.IsFavorite); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(muscles, &candidate.PrimaryMuscles); err != nil {
			return nil, err
		}
		result = append(result, candidate)
	}
	return result, rows.Err()
}

func (r *Repository) Daily(ctx context.Context, userID int, date time.Time) (planning.PlanSession, error) {
	var session planning.PlanSession
	var planJSON []byte
	var workoutID sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT id, workout_id, plan_date::text, status, plan
		FROM workout_plans
		WHERE user_id=$1 AND plan_date=$2::date AND status='active'
		ORDER BY updated_at DESC LIMIT 1
	`, userID, date).Scan(&session.ID, &workoutID, &session.PlanDate, &session.Status, &planJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return planning.PlanSession{}, planning.ErrNotFound
	}
	if err != nil {
		return planning.PlanSession{}, err
	}
	if workoutID.Valid {
		session.WorkoutID = int(workoutID.Int64)
	}
	if err := json.Unmarshal(planJSON, &session.Plan); err != nil {
		return planning.PlanSession{}, err
	}
	return session, nil
}

func (r *Repository) SaveDaily(ctx context.Context, userID int, date time.Time, plan planning.WorkoutPlan) (planning.PlanSession, error) {
	encoded, err := json.Marshal(plan)
	if err != nil {
		return planning.PlanSession{}, err
	}
	var session planning.PlanSession
	var workoutID sql.NullInt64
	var savedJSON []byte
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO workout_plans (workout_id,user_id,plan_date,title,estimated_duration_min,status,plan)
		VALUES (NULL,$1,$2::date,$3,$4,'active',$5::jsonb)
		ON CONFLICT (user_id,plan_date) WHERE status='active' DO UPDATE SET
			title=EXCLUDED.title, estimated_duration_min=EXCLUDED.estimated_duration_min,
			plan=EXCLUDED.plan, updated_at=CURRENT_TIMESTAMP
		RETURNING id,workout_id,plan_date::text,status,plan
	`, userID, date, plan.WorkoutTitle, plan.EstimatedDurationMin, string(encoded)).Scan(&session.ID, &workoutID, &session.PlanDate, &session.Status, &savedJSON)
	if err != nil {
		return planning.PlanSession{}, err
	}
	if workoutID.Valid {
		session.WorkoutID = int(workoutID.Int64)
	}
	if err := json.Unmarshal(savedJSON, &session.Plan); err != nil {
		return planning.PlanSession{}, err
	}
	return session, nil
}

// CreateDailyIfMissing never replaces a plan committed by another request.
// AttachWorkout subsequently locks that plan and reuses its workout_id.
func (r *Repository) CreateDailyIfMissing(ctx context.Context, userID int, date time.Time, plan planning.WorkoutPlan) (bool, error) {
	encoded, err := json.Marshal(plan)
	if err != nil {
		return false, err
	}
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO workout_plans (workout_id,user_id,plan_date,title,estimated_duration_min,status,plan)
		VALUES (NULL,$1,$2::date,$3,$4,'active',$5::jsonb)
		ON CONFLICT (user_id,plan_date) WHERE status='active' DO NOTHING
	`, userID, date, plan.WorkoutTitle, plan.EstimatedDurationMin, string(encoded))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// ResumeDaily reopens the plan the user already finished on this date together
// with its workout, so a same-day restart appends to that record instead of
// creating a second plan and workout. It reports false when nothing was resumed.
func (r *Repository) ResumeDaily(ctx context.Context, userID int, date time.Time) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var planID int
	var workoutID sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT id, workout_id
		FROM workout_plans
		WHERE user_id=$1 AND plan_date=$2::date AND status='completed'
		ORDER BY updated_at DESC, id DESC LIMIT 1
	`, userID, date).Scan(&planID, &workoutID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if workoutID.Valid {
		// Finish locks the workout before its plan; use the same order here.
		var lockedID int
		if err := tx.QueryRowContext(ctx, `SELECT id FROM workouts WHERE id=$1 AND user_id=$2 FOR UPDATE`, workoutID.Int64, userID).Scan(&lockedID); err != nil {
			return false, err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE workout_plans SET status='active',updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND status='completed'`, planID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "workout_plans_user_date_active_unique" {
			// A concurrent edit made another plan active for this date; the start reuses that one.
			return false, nil
		}
		return false, err
	}
	if count, err := result.RowsAffected(); err != nil || count == 0 {
		return false, err
	}
	if workoutID.Valid {
		// The summary comment described the workout as it was finished, so it is cleared with the end time.
		if _, err := tx.ExecContext(ctx, `UPDATE workouts SET ended_at=NULL,summary_comment=NULL WHERE id=$1 AND user_id=$2`, workoutID.Int64, userID); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

var errWorkoutBindingChanged = errors.New("workout binding changed")

func (r *Repository) AttachWorkout(ctx context.Context, userID int, date time.Time) (planning.PlanSession, error) {
	for {
		session, err := r.attachWorkout(ctx, userID, date)
		if !errors.Is(err, errWorkoutBindingChanged) {
			return session, err
		}
	}
}

func (r *Repository) attachWorkout(ctx context.Context, userID int, date time.Time) (planning.PlanSession, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return planning.PlanSession{}, err
	}
	defer tx.Rollback()

	var observedPlanID int
	var observedWorkoutID sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT id, workout_id FROM workout_plans
		WHERE user_id=$1 AND plan_date=$2::date AND status='active'
	`, userID, date).Scan(&observedPlanID, &observedWorkoutID)
	if errors.Is(err, sql.ErrNoRows) {
		return planning.PlanSession{}, planning.ErrNotFound
	}
	if err != nil {
		return planning.PlanSession{}, err
	}
	if observedWorkoutID.Valid {
		// Finish and ResumeDaily also lock the workout before its plan.
		var lockedID int
		if err := tx.QueryRowContext(ctx, `SELECT id FROM workouts WHERE id=$1 AND user_id=$2 FOR UPDATE`, observedWorkoutID.Int64, userID).Scan(&lockedID); err != nil {
			return planning.PlanSession{}, err
		}
	}

	var session planning.PlanSession
	var planJSON []byte
	var workoutID sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT id, workout_id, plan_date::text, status, plan
		FROM workout_plans
		WHERE user_id=$1 AND plan_date=$2::date AND status='active'
		FOR UPDATE
	`, userID, date).Scan(&session.ID, &workoutID, &session.PlanDate, &session.Status, &planJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return planning.PlanSession{}, planning.ErrNotFound
	}
	if err != nil {
		return planning.PlanSession{}, err
	}
	if session.ID != observedPlanID || workoutID != observedWorkoutID {
		// Another start attached a workout while this transaction waited for the
		// plan. Release the plan and acquire the new workout first on the retry.
		return planning.PlanSession{}, errWorkoutBindingChanged
	}
	if err := json.Unmarshal(planJSON, &session.Plan); err != nil {
		return planning.PlanSession{}, err
	}
	if workoutID.Valid {
		session.WorkoutID = int(workoutID.Int64)
	} else {
		start := date
		end := date.AddDate(0, 0, 1)
		err = tx.QueryRowContext(ctx, `
			SELECT id FROM workouts WHERE user_id=$1 AND ended_at IS NULL AND started_at >= $2 AND started_at < $3
			ORDER BY started_at DESC LIMIT 1
		`, userID, start, end).Scan(&session.WorkoutID)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `INSERT INTO workouts (user_id, notes) VALUES ($1,$2) RETURNING id`, userID, session.Plan.WorkoutTitle).Scan(&session.WorkoutID)
		}
		if err != nil {
			return planning.PlanSession{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE workout_plans SET workout_id=$1,updated_at=CURRENT_TIMESTAMP WHERE id=$2`, session.WorkoutID, session.ID); err != nil {
			return planning.PlanSession{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workouts SET notes=$1 WHERE id=$2 AND user_id=$3`, session.Plan.WorkoutTitle, session.WorkoutID, userID); err != nil {
		return planning.PlanSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return planning.PlanSession{}, err
	}
	return session, nil
}

func (r *Repository) ResolveExercise(ctx context.Context, userID int, exerciseID, name string) (planning.ExerciseStats, string, error) {
	var id, displayName string
	if exerciseID != "" {
		err := r.db.QueryRowContext(ctx, `SELECT id,name FROM exercises WHERE id=$1`, exerciseID).Scan(&id, &displayName)
		if errors.Is(err, sql.ErrNoRows) {
			return planning.ExerciseStats{}, "", planning.ErrExerciseNotFound
		}
		if err != nil {
			return planning.ExerciseStats{}, "", err
		}
	} else {
		err := r.db.QueryRowContext(ctx, `SELECT id,name FROM exercises WHERE name=$1 OR id=$1 LIMIT 1`, name).Scan(&id, &displayName)
		if errors.Is(err, sql.ErrNoRows) {
			err = r.db.QueryRowContext(ctx, `SELECT id,name FROM exercises WHERE name ILIKE $1 OR id ILIKE $1 ORDER BY name LIMIT 1`, "%"+name+"%").Scan(&id, &displayName)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return planning.ExerciseStats{}, "", planning.ErrExerciseNotFound
		}
		if err != nil {
			return planning.ExerciseStats{}, "", err
		}
	}
	stats := planning.ExerciseStats{Name: displayName, TargetSets: 3, TargetWeight: 20, TargetReps: 10}
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(weight,0),COALESCE(max_weight,0),COALESCE(target_sets,3)
		FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2
	`, userID, id).Scan(&stats.TargetWeight, &stats.MaxWeight, &stats.TargetSets)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return planning.ExerciseStats{}, "", err
	}
	if stats.TargetWeight == 0 && stats.MaxWeight > 0 {
		stats.TargetWeight = stats.MaxWeight * 0.8
	}
	if stats.TargetWeight == 0 {
		stats.TargetWeight = 20
	}
	return stats, id, nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanMonthly(row rowScanner) (planning.MonthlyPlan, error) {
	var plan planning.MonthlyPlan
	var restDays, recommendedDays, routines []byte
	err := row.Scan(&plan.ID, &plan.PlanMonth, &plan.PlanName, &plan.Frequency, &plan.Description, &plan.Rationale, &restDays, &recommendedDays, &routines)
	if err != nil {
		return planning.MonthlyPlan{}, err
	}
	if err := json.Unmarshal(restDays, &plan.RestDays); err != nil {
		return planning.MonthlyPlan{}, err
	}
	if err := json.Unmarshal(recommendedDays, &plan.RecommendedDays); err != nil {
		return planning.MonthlyPlan{}, err
	}
	if err := json.Unmarshal(routines, &plan.WeeklyRoutine); err != nil {
		return planning.MonthlyPlan{}, err
	}
	return plan, nil
}
