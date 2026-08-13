package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/jPurin-gg/myfitlog-backend/internal/profile"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Get(ctx context.Context, userID int) (profile.Preferences, error) {
	preferences := profile.Preferences{PreferredEquipment: []string{}, AvoidedEquipment: []string{}}
	var preferredJSON, avoidedJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT preferred_equipment, avoided_equipment, training_environment, notes
		FROM user_preferences WHERE user_id = $1
	`, userID).Scan(&preferredJSON, &avoidedJSON, &preferences.TrainingEnvironment, &preferences.Notes)
	if errors.Is(err, sql.ErrNoRows) {
		return preferences, nil
	}
	if err != nil {
		return profile.Preferences{}, err
	}
	if err := json.Unmarshal(preferredJSON, &preferences.PreferredEquipment); err != nil {
		return profile.Preferences{}, err
	}
	if err := json.Unmarshal(avoidedJSON, &preferences.AvoidedEquipment); err != nil {
		return profile.Preferences{}, err
	}
	return preferences, nil
}

func (r *Repository) Save(ctx context.Context, userID int, preferences profile.Preferences) (profile.Preferences, error) {
	preferredJSON, err := json.Marshal(preferences.PreferredEquipment)
	if err != nil {
		return profile.Preferences{}, err
	}
	avoidedJSON, err := json.Marshal(preferences.AvoidedEquipment)
	if err != nil {
		return profile.Preferences{}, err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, preferred_equipment, avoided_equipment, training_environment, notes)
		VALUES ($1, $2::jsonb, $3::jsonb, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			preferred_equipment = EXCLUDED.preferred_equipment,
			avoided_equipment = EXCLUDED.avoided_equipment,
			training_environment = EXCLUDED.training_environment,
			notes = EXCLUDED.notes,
			updated_at = CURRENT_TIMESTAMP
	`, userID, string(preferredJSON), string(avoidedJSON), preferences.TrainingEnvironment, preferences.Notes)
	return preferences, err
}
