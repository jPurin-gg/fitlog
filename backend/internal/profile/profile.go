package profile

import (
	"context"
	"strings"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

type Preferences struct {
	PreferredEquipment  []string `json:"preferred_equipment"`
	AvoidedEquipment    []string `json:"avoided_equipment"`
	TrainingEnvironment string   `json:"training_environment"`
	Notes               string   `json:"notes"`
}

type Repository interface {
	Get(ctx context.Context, userID int) (Preferences, error)
	Save(ctx context.Context, userID int, preferences Preferences) (Preferences, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Get(ctx context.Context, userID int) (Preferences, error) {
	preferences, err := s.repository.Get(ctx, userID)
	if err != nil {
		return Preferences{}, apperr.Internal(err)
	}
	return Normalize(preferences), nil
}

func (s *Service) Save(ctx context.Context, userID int, preferences Preferences) (Preferences, error) {
	preferences = Normalize(preferences)
	saved, err := s.repository.Save(ctx, userID, preferences)
	if err != nil {
		return Preferences{}, apperr.Internal(err)
	}
	return saved, nil
}

func Normalize(preferences Preferences) Preferences {
	preferences.PreferredEquipment = cleanStrings(preferences.PreferredEquipment)
	preferences.AvoidedEquipment = cleanStrings(preferences.AvoidedEquipment)
	preferences.TrainingEnvironment = strings.TrimSpace(preferences.TrainingEnvironment)
	preferences.Notes = strings.TrimSpace(preferences.Notes)
	avoided := make(map[string]bool, len(preferences.AvoidedEquipment))
	for _, equipment := range preferences.AvoidedEquipment {
		avoided[equipment] = true
	}
	preferred := make([]string, 0, len(preferences.PreferredEquipment))
	for _, equipment := range preferences.PreferredEquipment {
		if !avoided[equipment] {
			preferred = append(preferred, equipment)
		}
	}
	preferences.PreferredEquipment = preferred
	return preferences
}

func cleanStrings(values []string) []string {
	result := []string{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}
