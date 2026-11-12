package postgres

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/lib/pq"

	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) RecordSet(ctx context.Context, userID, workoutID int, key string, input workout.SetInput) (workout.Set, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return workout.Set{}, false, err
	}
	defer tx.Rollback()

	var endedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT ended_at FROM workouts
		WHERE id=$1 AND user_id=$2
		FOR UPDATE
	`, workoutID, userID).Scan(&endedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return workout.Set{}, false, workout.ErrNotFound
	}
	if err != nil {
		return workout.Set{}, false, err
	}

	if existing, found, err := findSetByKey(ctx, tx, userID, workoutID, key); err != nil {
		return workout.Set{}, false, err
	} else if found {
		if !sameSet(existing, input) {
			return workout.Set{}, false, workout.ErrConflict
		}
		return existing, true, nil
	}
	if endedAt.Valid {
		return workout.Set{}, false, workout.ErrNotFound
	}

	var exerciseExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM exercises WHERE id=$1)`, input.ExerciseID).Scan(&exerciseExists); err != nil {
		return workout.Set{}, false, err
	}
	if !exerciseExists {
		return workout.Set{}, false, workout.ErrNotFound
	}
	var previousMax float64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(max_weight,0) FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2`, userID, input.ExerciseID).Scan(&previousMax); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return workout.Set{}, false, err
	}

	var result workout.Set
	var createdAt time.Time
	err = tx.QueryRowContext(ctx, `
		INSERT INTO workout_sets (workout_id,exercise_id,weight,reps,set_order,feeling,is_pr,idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id,workout_id,exercise_id,set_order,weight,reps,COALESCE(feeling,''),is_pr,created_at
	`, workoutID, input.ExerciseID, input.Weight, input.Reps, input.SetOrder, input.Feeling, input.Weight > previousMax, key).Scan(
		&result.ID, &result.WorkoutID, &result.ExerciseID, &result.SetOrder, &result.Weight, &result.Reps, &result.Feeling, &result.IsPR, &createdAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			tx.Rollback()
			existing, found, findErr := findSetByKey(ctx, r.db, userID, workoutID, key)
			if findErr != nil {
				return workout.Set{}, false, findErr
			}
			if found && sameSet(existing, input) {
				return existing, true, nil
			}
			return workout.Set{}, false, workout.ErrConflict
		}
		return workout.Set{}, false, err
	}
	result.CreatedAt = createdAt.Format(time.RFC3339)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_exercise_stats (user_id,exercise_id,weight,max_weight)
		VALUES ($1,$2,$3,$3)
		ON CONFLICT (user_id,exercise_id) DO UPDATE SET
			weight=EXCLUDED.weight,
			max_weight=GREATEST(COALESCE(user_exercise_stats.max_weight,0),EXCLUDED.max_weight),
			updated_at=CURRENT_TIMESTAMP
	`, userID, input.ExerciseID, input.Weight)
	if err != nil {
		return workout.Set{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return workout.Set{}, false, err
	}
	return result, false, nil
}

func (r *Repository) RecommendationContext(ctx context.Context, userID, workoutID, setID int) (workout.RecommendationContext, error) {
	var result workout.RecommendationContext
	var exists bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workouts WHERE id=$1 AND user_id=$2)`, workoutID, userID).Scan(&exists); err != nil {
		return result, err
	}
	if !exists {
		return result, workout.ErrNotFound
	}
	if setID > 0 {
		var createdAt time.Time
		err := r.db.QueryRowContext(ctx, `
			SELECT ws.id,ws.workout_id,ws.exercise_id,ws.set_order,ws.weight,ws.reps,COALESCE(ws.feeling,''),ws.is_pr,ws.created_at,COALESCE(e.name,ws.exercise_id)
			FROM workout_sets ws JOIN workouts w ON w.id=ws.workout_id LEFT JOIN exercises e ON e.id=ws.exercise_id
			WHERE ws.id=$1 AND ws.workout_id=$2 AND w.user_id=$3
		`, setID, workoutID, userID).Scan(&result.Set.ID, &result.Set.WorkoutID, &result.Set.ExerciseID, &result.Set.SetOrder, &result.Set.Weight, &result.Set.Reps, &result.Set.Feeling, &result.Set.IsPR, &createdAt, &result.ExerciseName)
		if errors.Is(err, sql.ErrNoRows) {
			return result, workout.ErrNotFound
		}
		if err != nil {
			return result, err
		}
		result.Set.CreatedAt = createdAt.Format(time.RFC3339)
		if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(max_weight,0) FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2`, userID, result.Set.ExerciseID).Scan(&result.MaxWeight); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return result, err
		}
		if result.MaxWeight == 0 {
			result.MaxWeight = 10
		}
		recentRows, err := r.db.QueryContext(ctx, `
			SELECT ws.created_at::date::text,COALESCE(e.name,ws.exercise_id),ws.set_order,ws.weight,ws.reps,COALESCE(ws.feeling,''),ws.is_pr
			FROM workout_sets ws JOIN workouts w ON w.id=ws.workout_id LEFT JOIN exercises e ON e.id=ws.exercise_id
			WHERE w.user_id=$1 AND ws.exercise_id=$2
			ORDER BY ws.created_at DESC,ws.id DESC LIMIT 8
		`, userID, result.Set.ExerciseID)
		if err != nil {
			return result, err
		}
		result.Recent, err = scanHistory(recentRows)
		recentRows.Close()
		if err != nil {
			return result, err
		}
	}
	workoutRows, err := r.db.QueryContext(ctx, `
		SELECT ws.created_at::date::text,COALESCE(e.name,ws.exercise_id),ws.set_order,ws.weight,ws.reps,COALESCE(ws.feeling,''),ws.is_pr
		FROM workout_sets ws JOIN workouts w ON w.id=ws.workout_id LEFT JOIN exercises e ON e.id=ws.exercise_id
		WHERE w.user_id=$1 AND ws.workout_id=$2 ORDER BY ws.created_at,ws.id
	`, userID, workoutID)
	if err != nil {
		return result, err
	}
	result.WorkoutSets, err = scanHistory(workoutRows)
	workoutRows.Close()
	return result, err
}

func (r *Repository) Finish(ctx context.Context, userID, workoutID int) (workout.Detail, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return workout.Detail{}, err
	}
	defer tx.Rollback()
	var endedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT ended_at FROM workouts
		WHERE id=$1 AND user_id=$2
		FOR UPDATE
	`, workoutID, userID).Scan(&endedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return workout.Detail{}, workout.ErrNotFound
	}
	if err != nil {
		return workout.Detail{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE workouts SET ended_at=COALESCE(ended_at,(SELECT MAX(created_at) FROM workout_sets WHERE workout_id=workouts.id),CURRENT_TIMESTAMP)
		WHERE id=$1 AND user_id=$2
	`, workoutID, userID)
	if err != nil {
		return workout.Detail{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return workout.Detail{}, err
	}
	if affected == 0 {
		return workout.Detail{}, workout.ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workout_plans SET status='completed',updated_at=CURRENT_TIMESTAMP WHERE workout_id=$1 AND user_id=$2 AND status='active'`, workoutID, userID); err != nil {
		return workout.Detail{}, err
	}
	if err := tx.Commit(); err != nil {
		return workout.Detail{}, err
	}
	return r.Detail(ctx, userID, workoutID)
}

func (r *Repository) Detail(ctx context.Context, userID, workoutID int) (workout.Detail, error) {
	var detail workout.Detail
	var startedAt time.Time
	var endedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id,COALESCE(notes,'ワークアウト'),started_at,ended_at,CASE WHEN ended_at IS NULL THEN 'active' ELSE 'completed' END
		FROM workouts WHERE id=$1 AND user_id=$2
	`, workoutID, userID).Scan(&detail.ID, &detail.Title, &startedAt, &endedAt, &detail.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return workout.Detail{}, workout.ErrNotFound
	}
	if err != nil {
		return workout.Detail{}, err
	}
	detail.StartedAt = startedAt.Format(time.RFC3339)
	if endedAt.Valid {
		detail.EndedAt = endedAt.Time.Format(time.RFC3339)
	}
	detail.Summary, err = r.summary(ctx, userID, workoutID)
	if err != nil {
		return detail, err
	}
	detail.Sets, err = r.detailSets(ctx, userID, workoutID)
	return detail, err
}

func (r *Repository) History(ctx context.Context, userID, limit, offset int) ([]workout.HistoryEntry, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)::int FROM workouts WHERE user_id=$1 AND ended_at IS NOT NULL
	`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT w.id,COALESCE(w.notes,'ワークアウト'),w.started_at,w.ended_at,
		       GREATEST(1,CEIL(EXTRACT(EPOCH FROM (w.ended_at-w.started_at))/60))::int,
		       COALESCE(s.total_sets,0)::int,COALESCE(s.total_volume,0)
		FROM workouts w
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS total_sets,SUM(ws.weight*ws.reps) AS total_volume
			FROM workout_sets ws WHERE ws.workout_id=w.id
		) s ON TRUE
		WHERE w.user_id=$1 AND w.ended_at IS NOT NULL
		ORDER BY w.started_at DESC,w.id DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []workout.HistoryEntry{}
	for rows.Next() {
		var entry workout.HistoryEntry
		var startedAt, endedAt time.Time
		if err := rows.Scan(&entry.ID, &entry.Title, &startedAt, &endedAt, &entry.DurationMin, &entry.TotalSets, &entry.TotalVolume); err != nil {
			return nil, 0, err
		}
		entry.StartedAt = startedAt.Format(time.RFC3339)
		entry.EndedAt = endedAt.Format(time.RFC3339)
		result = append(result, entry)
	}
	return result, total, rows.Err()
}

// Delete はワークアウトとセットを削除し、関係する種目の最新重量・最大重量を残りの記録から再計算する。
func (r *Repository) Delete(ctx context.Context, userID, workoutID int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// セット保存・終了・カレンダー編集と同じworkout行のロックで直列化する。
	var lockedID int
	err = tx.QueryRowContext(ctx, `SELECT id FROM workouts WHERE id=$1 AND user_id=$2 FOR UPDATE`, workoutID, userID).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return workout.ErrNotFound
	}
	if err != nil {
		return err
	}
	impacted := map[string]bool{}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT exercise_id FROM workout_sets WHERE workout_id=$1 AND exercise_id IS NOT NULL`, workoutID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		impacted[id] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	// 日次予定はworkoutsを参照している。実施済みの予定は記録と一緒に消し、
	// 進行中だった予定は紐付けだけ外して、その日をやり直せるようにする。
	if _, err := tx.ExecContext(ctx, `DELETE FROM workout_plans WHERE workout_id=$1 AND status<>'active'`, workoutID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workout_plans SET workout_id=NULL,updated_at=CURRENT_TIMESTAMP WHERE workout_id=$1`, workoutID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workout_sets WHERE workout_id=$1`, workoutID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workouts WHERE id=$1 AND user_id=$2`, workoutID, userID); err != nil {
		return err
	}
	if err := recalculateImpactedStats(ctx, tx, userID, impacted); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) SaveSummaryComment(ctx context.Context, userID, workoutID int, comment string) (string, bool, error) {
	var saved string
	err := r.db.QueryRowContext(ctx, `
		UPDATE workouts SET summary_comment=$1
		WHERE id=$2 AND user_id=$3 AND ended_at IS NOT NULL
		  AND COALESCE(BTRIM(summary_comment),'')=''
		RETURNING summary_comment
	`, comment, workoutID, userID).Scan(&saved)
	if err == nil {
		return saved, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}

	var existing string
	var completed bool
	err = r.db.QueryRowContext(ctx, `
		SELECT COALESCE(summary_comment,''), ended_at IS NOT NULL
		FROM workouts WHERE id=$1 AND user_id=$2
	`, workoutID, userID).Scan(&existing, &completed)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, workout.ErrNotFound
	}
	if err != nil {
		return "", false, err
	}
	if !completed {
		return "", false, workout.ErrConflict
	}
	if existing != "" {
		return existing, true, nil
	}
	return "", false, errors.New("summary comment was not saved")
}

func (r *Repository) CalendarWorkout(ctx context.Context, userID int, date time.Time) (workout.CalendarWorkout, error) {
	start, end := date, date.AddDate(0, 0, 1)
	var result workout.CalendarWorkout
	err := r.db.QueryRowContext(ctx, `
		SELECT id,COALESCE(notes,'筋トレ') FROM workouts
		WHERE user_id=$1 AND started_at >= $2 AND started_at < $3
		ORDER BY started_at DESC,id DESC LIMIT 1
	`, userID, start, end).Scan(&result.WorkoutID, &result.Title)
	if errors.Is(err, sql.ErrNoRows) {
		return workout.CalendarWorkout{}, workout.ErrNotFound
	}
	if err != nil {
		return workout.CalendarWorkout{}, err
	}
	result.Date = date.Format("2006-01-02")
	result.Sets, err = r.calendarSets(ctx, userID, result.WorkoutID)
	return result, err
}

func (r *Repository) SaveCalendarWorkout(ctx context.Context, userID int, date time.Time, input workout.CalendarWorkoutInput) (workout.CalendarWorkout, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return workout.CalendarWorkout{}, err
	}
	defer tx.Rollback()
	for _, set := range input.Sets {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM exercises WHERE id=$1)`, set.ExerciseID).Scan(&exists); err != nil {
			return workout.CalendarWorkout{}, err
		}
		if !exists {
			return workout.CalendarWorkout{}, workout.ErrNotFound
		}
	}
	start, end := date, date.AddDate(0, 0, 1)
	var workoutID int
	var startedAt time.Time
	// 行ロックで、同じワークアウトへのセット保存・終了・削除と直列化する。
	err = tx.QueryRowContext(ctx, `
		SELECT id,started_at FROM workouts
		WHERE user_id=$1 AND started_at >= $2 AND started_at < $3
		ORDER BY started_at DESC,id DESC LIMIT 1
		FOR UPDATE
	`, userID, start, end).Scan(&workoutID, &startedAt)
	impacted := map[string]bool{}
	if errors.Is(err, sql.ErrNoRows) {
		workoutID, err = createCalendarWorkout(ctx, tx, userID, date, input, impacted)
	} else if err == nil {
		err = updateCalendarWorkout(ctx, tx, userID, workoutID, startedAt, input, impacted)
	}
	if err != nil {
		return workout.CalendarWorkout{}, err
	}
	if err := recalculateImpactedStats(ctx, tx, userID, impacted); err != nil {
		return workout.CalendarWorkout{}, err
	}
	if err := tx.Commit(); err != nil {
		return workout.CalendarWorkout{}, err
	}
	return r.CalendarWorkout(ctx, userID, date)
}

// createCalendarWorkout は、記録のない日にカレンダーから新しい記録を作る。
// 実際の時刻が分からないため、正午開始・「15分＋3分×セット数」の所要時間を仮置きする。
func createCalendarWorkout(ctx context.Context, tx *sql.Tx, userID int, date time.Time, input workout.CalendarWorkoutInput, impacted map[string]bool) (int, error) {
	startedAt := date.Add(12 * time.Hour)
	endedAt := startedAt.Add(time.Duration(15+len(input.Sets)*3) * time.Minute)
	var workoutID int
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO workouts (user_id,started_at,ended_at,notes,summary_comment) VALUES ($1,$2,$3,$4,NULL) RETURNING id
	`, userID, startedAt, endedAt, input.Title).Scan(&workoutID); err != nil {
		return 0, err
	}
	prs := newCalendarPRChecker(tx, userID, workoutID)
	for index, set := range input.Sets {
		impacted[set.ExerciseID] = true
		isPR, err := prs.observe(ctx, set.ExerciseID, set.Weight)
		if err != nil {
			return 0, err
		}
		if err := insertCalendarSet(ctx, tx, workoutID, set, isPR, startedAt.Add(time.Duration(index)*time.Minute)); err != nil {
			return 0, err
		}
	}
	return workoutID, nil
}

// updateCalendarWorkout は、既存の記録をカレンダーの編集内容で置き換える。
// 開始・終了時刻とAI総評には触れず、残るセットは記録時刻・冪等キーごとその場で更新する。
func updateCalendarWorkout(ctx context.Context, tx *sql.Tx, userID, workoutID int, startedAt time.Time, input workout.CalendarWorkoutInput, impacted map[string]bool) error {
	if _, err := tx.ExecContext(ctx, `UPDATE workouts SET notes=$1 WHERE id=$2 AND user_id=$3`, input.Title, workoutID, userID); err != nil {
		return err
	}
	type storedSet struct {
		exerciseID string
		weight     float64
		isPR       bool
		createdAt  time.Time
	}
	stored := map[int]storedSet{}
	rows, err := tx.QueryContext(ctx, `
		SELECT id,COALESCE(exercise_id,''),weight,COALESCE(is_pr,FALSE),created_at
		FROM workout_sets WHERE workout_id=$1
	`, workoutID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int
		var set storedSet
		if err := rows.Scan(&id, &set.exerciseID, &set.weight, &set.isPR, &set.createdAt); err != nil {
			rows.Close()
			return err
		}
		stored[id] = set
		if set.exerciseID != "" {
			impacted[set.exerciseID] = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	// 入力のIDがこのワークアウトの保存済みセットを指すときだけ「同じセットの修正」として扱う。
	// IDなし・未知のID・重複したIDは新しいセットとして追加する。
	kept := map[int]bool{}
	keptIDs := []int64{}
	for _, set := range input.Sets {
		if _, found := stored[set.ID]; found && !kept[set.ID] {
			kept[set.ID] = true
			keptIDs = append(keptIDs, int64(set.ID))
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workout_sets WHERE workout_id=$1 AND NOT (id = ANY($2))`, workoutID, pq.Array(keptIDs)); err != nil {
		return err
	}

	prs := newCalendarPRChecker(tx, userID, workoutID)
	updated := map[int]bool{}
	// 追加したセットは直前のセットと同じ時刻にして、ワークアウトの時間内・表示順（記録時刻, ID）を保つ。
	lastCreatedAt := startedAt
	for _, set := range input.Sets {
		impacted[set.ExerciseID] = true
		isPR, err := prs.observe(ctx, set.ExerciseID, set.Weight)
		if err != nil {
			return err
		}
		previous, found := stored[set.ID]
		if !found || updated[set.ID] {
			if err := insertCalendarSet(ctx, tx, workoutID, set, isPR, lastCreatedAt); err != nil {
				return err
			}
			continue
		}
		updated[set.ID] = true
		if previous.exerciseID == set.ExerciseID && previous.weight == set.Weight {
			// 種目と重量が同じなら、記録時点の自己ベスト判定を残す。
			isPR = previous.isPR
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE workout_sets SET exercise_id=$1,weight=$2,reps=$3,set_order=$4,feeling=$5,is_pr=$6
			WHERE id=$7 AND workout_id=$8
		`, set.ExerciseID, set.Weight, set.Reps, set.SetOrder, set.Feeling, isPR, set.ID, workoutID); err != nil {
			return err
		}
		if previous.createdAt.After(lastCreatedAt) {
			lastCreatedAt = previous.createdAt
		}
	}
	return nil
}

func insertCalendarSet(ctx context.Context, tx *sql.Tx, workoutID int, set workout.CalendarSet, isPR bool, createdAt time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO workout_sets (workout_id,exercise_id,weight,reps,set_order,feeling,is_pr,created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, workoutID, set.ExerciseID, set.Weight, set.Reps, set.SetOrder, set.Feeling, isPR, createdAt)
	return err
}

// calendarPRChecker は、カレンダーから保存するセットが自己ベストかを判定する。
// 基準は、他のワークアウトでの最大重量と、同じ保存内で先行するセットの重量。
type calendarPRChecker struct {
	tx        *sql.Tx
	userID    int
	workoutID int
	best      map[string]float64
}

func newCalendarPRChecker(tx *sql.Tx, userID, workoutID int) *calendarPRChecker {
	return &calendarPRChecker{tx: tx, userID: userID, workoutID: workoutID, best: map[string]float64{}}
}

func (c *calendarPRChecker) observe(ctx context.Context, exerciseID string, weight float64) (bool, error) {
	current, known := c.best[exerciseID]
	if !known {
		if err := c.tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(ws.weight),0) FROM workout_sets ws JOIN workouts w ON w.id=ws.workout_id
			WHERE w.user_id=$1 AND ws.exercise_id=$2 AND ws.workout_id<>$3
		`, c.userID, exerciseID, c.workoutID).Scan(&current); err != nil {
			return false, err
		}
	}
	isPR := weight > current
	if isPR {
		current = weight
	}
	c.best[exerciseID] = current
	return isPR, nil
}

func recalculateImpactedStats(ctx context.Context, tx *sql.Tx, userID int, impacted map[string]bool) error {
	ids := make([]string, 0, len(impacted))
	for id := range impacted {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := recalculateStats(ctx, tx, userID, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) summary(ctx context.Context, userID, workoutID int) (workout.Summary, error) {
	summary := workout.Summary{Exercises: []workout.SummaryExercise{}}
	err := r.db.QueryRowContext(ctx, `
		SELECT GREATEST(1,CEIL(EXTRACT(EPOCH FROM (COALESCE(ended_at,CURRENT_TIMESTAMP)-started_at))/60))::int,COALESCE(summary_comment,'')
		FROM workouts WHERE id=$1 AND user_id=$2
	`, workoutID, userID).Scan(&summary.DurationMin, &summary.AIComment)
	if err != nil {
		return summary, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT ws.exercise_id,COALESCE(e.name,ws.exercise_id),COUNT(*)::int,COALESCE(SUM(ws.reps),0)::int,
		       COALESCE(MAX(ws.weight),0),COALESCE(SUM(ws.weight*ws.reps),0),COALESCE(SUM(CASE WHEN ws.is_pr THEN 1 ELSE 0 END),0)::int
		FROM workout_sets ws LEFT JOIN exercises e ON e.id=ws.exercise_id
		WHERE ws.workout_id=$1 GROUP BY ws.exercise_id,e.name ORDER BY MIN(ws.created_at),MIN(ws.id)
	`, workoutID)
	if err != nil {
		return summary, err
	}
	defer rows.Close()
	for rows.Next() {
		var item workout.SummaryExercise
		var prCount int
		if err := rows.Scan(&item.ExerciseID, &item.Name, &item.Sets, &item.TotalReps, &item.BestWeight, &item.TotalVolume, &prCount); err != nil {
			return summary, err
		}
		summary.TotalSets += item.Sets
		summary.TotalReps += item.TotalReps
		summary.TotalVolume += item.TotalVolume
		summary.PRCount += prCount
		summary.Exercises = append(summary.Exercises, item)
	}
	return summary, rows.Err()
}

func (r *Repository) detailSets(ctx context.Context, userID, workoutID int) ([]workout.DetailSet, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ws.id,COALESCE(ws.exercise_id,''),COALESCE(e.name,ws.exercise_id,''),ws.set_order,ws.weight,ws.reps,
		       COALESCE(ws.feeling,''),COALESCE(ws.is_pr,FALSE),ws.created_at
		FROM workout_sets ws JOIN workouts w ON w.id=ws.workout_id LEFT JOIN exercises e ON e.id=ws.exercise_id
		WHERE w.user_id=$1 AND ws.workout_id=$2 ORDER BY ws.created_at,ws.id
	`, userID, workoutID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []workout.DetailSet{}
	for rows.Next() {
		var set workout.DetailSet
		var createdAt time.Time
		if err := rows.Scan(&set.ID, &set.ExerciseID, &set.ExerciseName, &set.SetOrder, &set.Weight, &set.Reps, &set.Feeling, &set.IsPR, &createdAt); err != nil {
			return nil, err
		}
		set.CreatedAt = createdAt.Format(time.RFC3339)
		result = append(result, set)
	}
	return result, rows.Err()
}

func (r *Repository) calendarSets(ctx context.Context, userID, workoutID int) ([]workout.CalendarSet, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ws.id,ws.exercise_id,COALESCE(e.name,ws.exercise_id),ws.weight,ws.reps,ws.set_order,COALESCE(ws.feeling,'')
		FROM workout_sets ws JOIN workouts w ON w.id=ws.workout_id LEFT JOIN exercises e ON e.id=ws.exercise_id
		WHERE w.user_id=$1 AND ws.workout_id=$2 ORDER BY ws.created_at,ws.id
	`, userID, workoutID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []workout.CalendarSet{}
	for rows.Next() {
		var set workout.CalendarSet
		if err := rows.Scan(&set.ID, &set.ExerciseID, &set.ExerciseName, &set.Weight, &set.Reps, &set.SetOrder, &set.Feeling); err != nil {
			return nil, err
		}
		result = append(result, set)
	}
	return result, rows.Err()
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func findSetByKey(ctx context.Context, query queryer, userID, workoutID int, key string) (workout.Set, bool, error) {
	var set workout.Set
	var createdAt time.Time
	err := query.QueryRowContext(ctx, `
		SELECT ws.id,ws.workout_id,ws.exercise_id,ws.set_order,ws.weight,ws.reps,COALESCE(ws.feeling,''),ws.is_pr,ws.created_at
		FROM workout_sets ws JOIN workouts w ON w.id=ws.workout_id
		WHERE w.user_id=$1 AND ws.workout_id=$2 AND ws.idempotency_key=$3
	`, userID, workoutID, key).Scan(&set.ID, &set.WorkoutID, &set.ExerciseID, &set.SetOrder, &set.Weight, &set.Reps, &set.Feeling, &set.IsPR, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return workout.Set{}, false, nil
	}
	if err != nil {
		return workout.Set{}, false, err
	}
	set.CreatedAt = createdAt.Format(time.RFC3339)
	return set, true, nil
}

func sameSet(existing workout.Set, input workout.SetInput) bool {
	return existing.ExerciseID == input.ExerciseID && existing.SetOrder == input.SetOrder && existing.Weight == input.Weight && existing.Reps == input.Reps && existing.Feeling == input.Feeling
}

func scanHistory(rows *sql.Rows) ([]workout.HistorySet, error) {
	result := []workout.HistorySet{}
	for rows.Next() {
		var item workout.HistorySet
		if err := rows.Scan(&item.Date, &item.ExerciseName, &item.SetOrder, &item.Weight, &item.Reps, &item.Feeling, &item.IsPR); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func recalculateStats(ctx context.Context, tx *sql.Tx, userID int, exerciseID string) error {
	var latestWeight, maxWeight float64
	err := tx.QueryRowContext(ctx, `
		SELECT latest.weight,stats.max_weight FROM (
			SELECT ws.weight FROM workout_sets ws JOIN workouts w ON w.id=ws.workout_id
			WHERE w.user_id=$1 AND ws.exercise_id=$2 ORDER BY w.started_at DESC,ws.created_at DESC,ws.id DESC LIMIT 1
		) latest CROSS JOIN (
			SELECT COALESCE(MAX(ws.weight),0) max_weight FROM workout_sets ws JOIN workouts w ON w.id=ws.workout_id
			WHERE w.user_id=$1 AND ws.exercise_id=$2
		) stats
	`, userID, exerciseID).Scan(&latestWeight, &maxWeight)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `
			UPDATE user_exercise_stats
			SET weight=NULL,max_weight=NULL,updated_at=CURRENT_TIMESTAMP
			WHERE user_id=$1 AND exercise_id=$2
		`, userID, exerciseID)
		return err
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_exercise_stats (user_id,exercise_id,weight,max_weight) VALUES ($1,$2,$3,$4)
		ON CONFLICT (user_id,exercise_id) DO UPDATE SET weight=EXCLUDED.weight,max_weight=EXCLUDED.max_weight,updated_at=CURRENT_TIMESTAMP
	`, userID, exerciseID, latestWeight, maxWeight)
	return err
}
