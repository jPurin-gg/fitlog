package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type UserPreferences struct {
	UserID              int      `json:"user_id"`
	PreferredEquipment  []string `json:"preferred_equipment"`
	AvoidedEquipment    []string `json:"avoided_equipment"`
	TrainingEnvironment string   `json:"training_environment"`
	Notes               string   `json:"notes"`
}

func (app *App) handleUserPreferences(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		app.getUserPreferences(w, r)
	case http.MethodPut:
		app.saveUserPreferences(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (app *App) getUserPreferences(w http.ResponseWriter, r *http.Request) {
	userID := parseUserID(r.URL.Query().Get("user_id"))
	preferences, err := app.loadUserPreferences(userID)
	if err != nil {
		log.Printf("Failed to load user preferences: %v", err)
		http.Error(w, "Failed to load user preferences", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(preferences)
}

func (app *App) saveUserPreferences(w http.ResponseWriter, r *http.Request) {
	var req UserPreferences
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	if req.UserID == 0 {
		req.UserID = 1
	}
	req = normalizeUserPreferences(req)

	preferredJSON, err := json.Marshal(req.PreferredEquipment)
	if err != nil {
		http.Error(w, "Invalid preferred equipment", http.StatusBadRequest)
		return
	}
	avoidedJSON, err := json.Marshal(req.AvoidedEquipment)
	if err != nil {
		http.Error(w, "Invalid avoided equipment", http.StatusBadRequest)
		return
	}

	err = app.db.QueryRow(`
		INSERT INTO user_preferences (
			user_id, preferred_equipment, avoided_equipment, training_environment, notes
		)
		VALUES ($1, $2::jsonb, $3::jsonb, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			preferred_equipment = EXCLUDED.preferred_equipment,
			avoided_equipment = EXCLUDED.avoided_equipment,
			training_environment = EXCLUDED.training_environment,
			notes = EXCLUDED.notes,
			updated_at = CURRENT_TIMESTAMP
		RETURNING user_id
	`, req.UserID, string(preferredJSON), string(avoidedJSON), req.TrainingEnvironment, req.Notes).Scan(&req.UserID)
	if err != nil {
		log.Printf("Failed to save user preferences: %v", err)
		http.Error(w, "Failed to save user preferences", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

func (app *App) loadUserPreferences(userID int) (UserPreferences, error) {
	preferences := UserPreferences{
		UserID:             userID,
		PreferredEquipment: []string{},
		AvoidedEquipment:   []string{},
	}
	var preferredJSON []byte
	var avoidedJSON []byte
	err := app.db.QueryRow(`
		SELECT preferred_equipment, avoided_equipment, training_environment, notes
		FROM user_preferences
		WHERE user_id = $1
	`, userID).Scan(&preferredJSON, &avoidedJSON, &preferences.TrainingEnvironment, &preferences.Notes)
	if err != nil {
		if err == sql.ErrNoRows {
			return preferences, nil
		}
		return preferences, err
	}
	if err := json.Unmarshal(preferredJSON, &preferences.PreferredEquipment); err != nil {
		return preferences, err
	}
	if err := json.Unmarshal(avoidedJSON, &preferences.AvoidedEquipment); err != nil {
		return preferences, err
	}
	return normalizeUserPreferences(preferences), nil
}

func normalizeUserPreferences(preferences UserPreferences) UserPreferences {
	preferences.PreferredEquipment = cleanStringList(preferences.PreferredEquipment)
	preferences.AvoidedEquipment = cleanStringList(preferences.AvoidedEquipment)
	preferences.TrainingEnvironment = strings.TrimSpace(preferences.TrainingEnvironment)
	preferences.Notes = strings.TrimSpace(preferences.Notes)

	avoided := map[string]bool{}
	for _, equipment := range preferences.AvoidedEquipment {
		avoided[equipment] = true
	}
	preferred := []string{}
	for _, equipment := range preferences.PreferredEquipment {
		if !avoided[equipment] {
			preferred = append(preferred, equipment)
		}
	}
	preferences.PreferredEquipment = preferred
	return preferences
}
