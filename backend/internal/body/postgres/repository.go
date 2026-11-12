package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"github.com/jPurin-gg/myfitlog-backend/internal/body"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

const columns = `id, measured_on::text, weight_kg, body_fat_percent, resting_heart_rate`

func scan(row interface{ Scan(...any) error }) (body.Measurement, error) {
	var measurement body.Measurement
	err := row.Scan(&measurement.ID, &measurement.MeasuredOn, &measurement.WeightKg, &measurement.BodyFatPercent, &measurement.RestingHeartRate)
	return measurement, err
}

func (r *Repository) List(ctx context.Context, userID int, limit int) ([]body.Measurement, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+columns+`
		FROM body_measurements WHERE user_id = $1
		ORDER BY measured_on DESC, id DESC LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []body.Measurement{}
	for rows.Next() {
		measurement, err := scan(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, measurement)
	}
	return result, rows.Err()
}

func (r *Repository) Create(ctx context.Context, userID int, input body.MeasurementInput) (body.Measurement, error) {
	measurement, err := scan(r.db.QueryRowContext(ctx, `
		INSERT INTO body_measurements (user_id, measured_on, weight_kg, body_fat_percent, resting_heart_rate)
		VALUES ($1, $2::date, $3, $4, $5)
		RETURNING `+columns+`
	`, userID, input.MeasuredOn, input.WeightKg, input.BodyFatPercent, input.RestingHeartRate))
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return body.Measurement{}, body.ErrDuplicateDate
		}
		return body.Measurement{}, err
	}
	return measurement, nil
}

func (r *Repository) Update(ctx context.Context, userID, id int, input body.MeasurementInput) (body.Measurement, error) {
	measurement, err := scan(r.db.QueryRowContext(ctx, `
		UPDATE body_measurements
		SET measured_on = $3::date, weight_kg = $4, body_fat_percent = $5, resting_heart_rate = $6,
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND id = $2
		RETURNING `+columns+`
	`, userID, id, input.MeasuredOn, input.WeightKg, input.BodyFatPercent, input.RestingHeartRate))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return body.Measurement{}, body.ErrNotFound
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return body.Measurement{}, body.ErrDuplicateDate
		}
		return body.Measurement{}, err
	}
	return measurement, nil
}

func (r *Repository) Delete(ctx context.Context, userID, id int) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM body_measurements WHERE user_id = $1 AND id = $2
	`, userID, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return body.ErrNotFound
	}
	return nil
}
