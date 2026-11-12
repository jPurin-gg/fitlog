package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/lib/pq"

	"github.com/jPurin-gg/myfitlog-backend/internal/exercise"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

// Search returns one page of lightweight items together with the number of
// exercises matching the filters. The id tiebreaker keeps the order stable
// across pages when several exercises share a name.
func (r *Repository) Search(ctx context.Context, filters exercise.Filters) ([]exercise.Summary, int, error) {
	conditions, args := searchConditions(filters)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM exercises WHERE `+conditions, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT id, name, level, equipment, category, primary_muscles FROM exercises WHERE ` + conditions +
		` ORDER BY CASE level WHEN '初級' THEN 1 WHEN 'beginner' THEN 1 WHEN '中級' THEN 2 WHEN 'intermediate' THEN 2 WHEN '上級' THEN 3 WHEN 'expert' THEN 3 ELSE 4 END, name ASC, id ASC` +
		fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	pageArgs := append(append([]any{}, args...), filters.Limit, filters.Offset)
	rows, err := r.db.QueryContext(ctx, query, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []exercise.Summary{}
	for rows.Next() {
		var item exercise.Summary
		var primary []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Level, &item.Equipment, &item.Category, &primary); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(primary, &item.PrimaryMuscles); err != nil || item.PrimaryMuscles == nil {
			item.PrimaryMuscles = []string{}
		}
		result = append(result, item)
	}
	return result, total, rows.Err()
}

// searchConditions builds the WHERE clause shared by the count and page
// queries. Muscles are compared with the elements of the JSON arrays, so a
// broad category can send all of its muscles at once.
func searchConditions(filters exercise.Filters) (string, []any) {
	conditions := `TRUE`
	args := []any{}
	add := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filters.Name != "" {
		placeholder := add("%" + filters.Name + "%")
		conditions += ` AND (name ILIKE ` + placeholder + ` OR id ILIKE ` + placeholder + `)`
	}
	if len(filters.Muscles) > 0 {
		placeholder := add(pq.Array(filters.Muscles))
		conditions += ` AND (primary_muscles ?| ` + placeholder + `::text[] OR secondary_muscles ?| ` + placeholder + `::text[])`
	}
	if len(filters.Equipment) == 1 {
		conditions += ` AND equipment = ` + add(filters.Equipment[0])
	} else if len(filters.Equipment) > 1 {
		conditions += ` AND equipment = ANY(` + add(pq.Array(filters.Equipment)) + `)`
	}
	if filters.Level != "" {
		conditions += ` AND level = ` + add(filters.Level)
	}
	return conditions, args
}

func (r *Repository) Get(ctx context.Context, exerciseID string) (exercise.Exercise, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, force, level, mechanic, equipment, category, instructions, primary_muscles, secondary_muscles, images
		FROM exercises WHERE id = $1
	`, exerciseID)
	if err != nil {
		return exercise.Exercise{}, err
	}
	defer rows.Close()
	items, err := scanExercises(rows)
	if err != nil {
		return exercise.Exercise{}, err
	}
	if len(items) == 0 {
		return exercise.Exercise{}, exercise.ErrNotFound
	}
	return items[0], nil
}

func (r *Repository) Create(ctx context.Context, item exercise.Exercise) error {
	primary, _ := json.Marshal(item.PrimaryMuscles)
	secondary, _ := json.Marshal(item.SecondaryMuscles)
	instructions, _ := json.Marshal(item.Instructions)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO exercises (id, name, category, equipment, primary_muscles, force, level, mechanic, instructions, secondary_muscles, images)
		VALUES ($1, $2, $3, $4, $5::jsonb, NULL, $6, NULL, $7::jsonb, $8::jsonb, '[]'::jsonb)
	`, item.ID, item.Name, item.Category, item.Equipment, string(primary), item.Level, string(instructions), string(secondary))
	return err
}

func (r *Repository) Favorites(ctx context.Context, userID int) ([]exercise.Exercise, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.id, e.name, e.force, e.level, e.mechanic, e.equipment, e.category,
		       e.instructions, e.primary_muscles, e.secondary_muscles, e.images
		FROM user_favorite_exercises f
		JOIN exercises e ON e.id = f.exercise_id
		WHERE f.user_id = $1
		ORDER BY f.created_at DESC, e.name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExercises(rows)
}

func (r *Repository) SetFavorite(ctx context.Context, userID int, exerciseID string, favorite bool) error {
	var exists bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM exercises WHERE id = $1)`, exerciseID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return exercise.ErrNotFound
	}
	if favorite {
		_, err := r.db.ExecContext(ctx, `INSERT INTO user_favorite_exercises (user_id, exercise_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, exerciseID)
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_favorite_exercises WHERE user_id = $1 AND exercise_id = $2`, userID, exerciseID)
	return err
}

func (r *Repository) Recent(ctx context.Context, userID int) ([]exercise.Exercise, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.id, e.name, e.force, e.level, e.mechanic, e.equipment, e.category,
		       e.instructions, e.primary_muscles, e.secondary_muscles, e.images
		FROM (
			SELECT ws.exercise_id, MAX(ws.created_at) AS last_used_at
			FROM workout_sets ws JOIN workouts w ON w.id = ws.workout_id
			WHERE w.user_id = $1
			GROUP BY ws.exercise_id ORDER BY last_used_at DESC LIMIT 20
		) recent
		JOIN exercises e ON e.id = recent.exercise_id
		ORDER BY recent.last_used_at DESC, e.name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExercises(rows)
}

func (r *Repository) Settings(ctx context.Context, userID int, exerciseID string) (exercise.Settings, error) {
	var settings exercise.Settings
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(target_sets, 3) FROM user_exercise_stats WHERE user_id = $1 AND exercise_id = $2`, userID, exerciseID).Scan(&settings.TargetSets)
	if errors.Is(err, sql.ErrNoRows) {
		return exercise.Settings{}, exercise.ErrNotFound
	}
	return settings, err
}

func (r *Repository) SaveSettings(ctx context.Context, userID int, exerciseID string, settings exercise.Settings) error {
	var exists bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM exercises WHERE id = $1)`, exerciseID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return exercise.ErrNotFound
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_exercise_stats (user_id, exercise_id, target_sets)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, exercise_id) DO UPDATE SET target_sets = EXCLUDED.target_sets, updated_at = CURRENT_TIMESTAMP
	`, userID, exerciseID, settings.TargetSets)
	return err
}

func (r *Repository) AlternativeContext(ctx context.Context, exerciseID string) (exercise.AlternativeContext, error) {
	var result exercise.AlternativeContext
	var musclesJSON []byte
	err := r.db.QueryRowContext(ctx, `SELECT name, primary_muscles FROM exercises WHERE id = $1`, exerciseID).Scan(&result.ExerciseName, &musclesJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return exercise.AlternativeContext{}, exercise.ErrNotFound
	}
	if err != nil {
		return exercise.AlternativeContext{}, err
	}
	if err := json.Unmarshal(musclesJSON, &result.Muscles); err != nil {
		return exercise.AlternativeContext{}, err
	}
	if len(result.Muscles) == 0 {
		return exercise.AlternativeContext{}, exercise.ErrNotFound
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, COALESCE(equipment, '不明')
		FROM exercises
		WHERE primary_muscles ?| $1::text[] AND id <> $2
		ORDER BY name LIMIT 30
	`, pq.Array(result.Muscles), exerciseID)
	if err != nil {
		return exercise.AlternativeContext{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, equipment string
		if err := rows.Scan(&id, &name, &equipment); err != nil {
			return exercise.AlternativeContext{}, err
		}
		result.Candidates = append(result.Candidates, exercise.AlternativeCandidate{ID: id, Name: name, Equipment: equipment})
	}
	return result, rows.Err()
}

type scanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanExercises(rows scanner) ([]exercise.Exercise, error) {
	result := []exercise.Exercise{}
	for rows.Next() {
		var item exercise.Exercise
		var instructions, primary, secondary, images []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Force, &item.Level, &item.Mechanic, &item.Equipment, &item.Category, &instructions, &primary, &secondary, &images); err != nil {
			return nil, err
		}
		item.Instructions = []string{}
		item.PrimaryMuscles = []string{}
		item.SecondaryMuscles = []string{}
		item.Images = []string{}
		_ = json.Unmarshal(instructions, &item.Instructions)
		_ = json.Unmarshal(primary, &item.PrimaryMuscles)
		_ = json.Unmarshal(secondary, &item.SecondaryMuscles)
		_ = json.Unmarshal(images, &item.Images)
		result = append(result, item)
	}
	return result, rows.Err()
}

func Seed(ctx context.Context, db *sql.DB, fileData []byte, logger *slog.Logger) error {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM exercises`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		logger.Info("exercise seed skipped", "reason", "already populated", "count", count)
		return nil
	}
	var exercises []exercise.Exercise
	if err := json.Unmarshal(fileData, &exercises); err != nil {
		return fmt.Errorf("parse exercise seed: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statement, err := tx.PrepareContext(ctx, `
		INSERT INTO exercises (id, name, force, level, mechanic, equipment, category, instructions, primary_muscles, secondary_muscles, images)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, item := range exercises {
		instructions, _ := json.Marshal(item.Instructions)
		primary, _ := json.Marshal(item.PrimaryMuscles)
		secondary, _ := json.Marshal(item.SecondaryMuscles)
		images, _ := json.Marshal(item.Images)
		if _, err := statement.ExecContext(ctx, item.ID, item.Name, item.Force, item.Level, item.Mechanic, item.Equipment, item.Category, string(instructions), string(primary), string(secondary), string(images)); err != nil {
			return fmt.Errorf("insert exercise %s: %w", item.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	logger.Info("exercise seed completed", "count", len(exercises))
	return nil
}
