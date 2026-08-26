package exercise

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/ai"
	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/prompt"
)

var ErrNotFound = errors.New("exercise not found")

type Exercise struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Force            *string  `json:"force"`
	Level            *string  `json:"level"`
	Mechanic         *string  `json:"mechanic"`
	Equipment        *string  `json:"equipment"`
	Category         *string  `json:"category"`
	Instructions     []string `json:"instructions"`
	PrimaryMuscles   []string `json:"primaryMuscles"`
	SecondaryMuscles []string `json:"secondaryMuscles"`
	Images           []string `json:"images"`
}

type Filters struct {
	Name      string
	Muscle    string
	Equipment []string
	Level     string
}

type CreateInput struct {
	Name             string   `json:"name"`
	Category         string   `json:"category"`
	Equipment        string   `json:"equipment"`
	Level            string   `json:"level"`
	Primary          string   `json:"primary_muscle"`
	PrimaryMuscles   []string `json:"primary_muscles"`
	SecondaryMuscles []string `json:"secondary_muscles"`
	Instructions     []string `json:"instructions"`
}

type Settings struct {
	TargetSets int `json:"target_sets"`
}

type Alternative struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AlternativeResponse struct {
	Alternatives []Alternative `json:"alternatives"`
	Message      string        `json:"message"`
}

type AlternativeContext struct {
	ExerciseName string
	Muscles      []string
	Candidates   []AlternativeCandidate
}

type AlternativeCandidate struct {
	ID        string
	Name      string
	Equipment string
}

type Repository interface {
	Search(ctx context.Context, filters Filters) ([]Exercise, error)
	Create(ctx context.Context, exercise Exercise) error
	Favorites(ctx context.Context, userID int) ([]Exercise, error)
	SetFavorite(ctx context.Context, userID int, exerciseID string, favorite bool) error
	Recent(ctx context.Context, userID int) ([]Exercise, error)
	Settings(ctx context.Context, userID int, exerciseID string) (Settings, error)
	SaveSettings(ctx context.Context, userID int, exerciseID string, settings Settings) error
	AlternativeContext(ctx context.Context, exerciseID string) (AlternativeContext, error)
}

type PromptRenderer interface {
	Pair(systemFilename, userFilename string, data any) (string, string, error)
}

type Service struct {
	repository Repository
	aiClient   ai.Client
	prompts    PromptRenderer
	logger     *slog.Logger
}

func NewService(repository Repository, aiClient ai.Client, prompts PromptRenderer) *Service {
	return &Service{repository: repository, aiClient: aiClient, prompts: prompts}
}

func (s *Service) WithLogger(logger *slog.Logger) *Service {
	s.logger = logger
	return s
}

func (s *Service) Search(ctx context.Context, filters Filters) ([]Exercise, error) {
	exercises, err := s.repository.Search(ctx, filters)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if exercises == nil {
		exercises = []Exercise{}
	}
	return exercises, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Exercise, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return Exercise{}, apperr.Validation("種目名は必須です。", map[string]string{"name": "required"})
	}
	primary := cleanStrings(input.PrimaryMuscles)
	if len(primary) == 0 && strings.TrimSpace(input.Primary) != "" {
		primary = []string{strings.TrimSpace(input.Primary)}
	}
	exercise := Exercise{
		ID:               "custom_" + randomID(),
		Name:             input.Name,
		Category:         stringPointer(input.Category),
		Equipment:        stringPointer(input.Equipment),
		Level:            stringPointer(input.Level),
		Instructions:     cleanStrings(input.Instructions),
		PrimaryMuscles:   primary,
		SecondaryMuscles: cleanStrings(input.SecondaryMuscles),
		Images:           []string{},
	}
	if err := s.repository.Create(ctx, exercise); err != nil {
		return Exercise{}, apperr.Internal(err)
	}
	return exercise, nil
}

func (s *Service) Favorites(ctx context.Context, userID int) ([]Exercise, error) {
	exercises, err := s.repository.Favorites(ctx, userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return exercises, nil
}

func (s *Service) SetFavorite(ctx context.Context, userID int, exerciseID string, favorite bool) error {
	if strings.TrimSpace(exerciseID) == "" {
		return apperr.Validation("種目IDが必要です。", map[string]string{"exercise_id": "required"})
	}
	if err := s.repository.SetFavorite(ctx, userID, exerciseID, favorite); err != nil {
		if errors.Is(err, ErrNotFound) {
			return apperr.NotFound("種目が見つかりません。")
		}
		return apperr.Internal(err)
	}
	return nil
}

func (s *Service) Recent(ctx context.Context, userID int) ([]Exercise, error) {
	exercises, err := s.repository.Recent(ctx, userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return exercises, nil
}

func (s *Service) Settings(ctx context.Context, userID int, exerciseID string) (Settings, error) {
	settings, err := s.repository.Settings(ctx, userID, exerciseID)
	if errors.Is(err, ErrNotFound) {
		return Settings{TargetSets: 3}, nil
	}
	if err != nil {
		return Settings{}, apperr.Internal(err)
	}
	return settings, nil
}

func (s *Service) SaveSettings(ctx context.Context, userID int, exerciseID string, settings Settings) (Settings, error) {
	if settings.TargetSets < 1 || settings.TargetSets > 20 {
		return Settings{}, apperr.Validation("目標セット数は1〜20で指定してください。", map[string]string{"target_sets": "must be between 1 and 20"})
	}
	if err := s.repository.SaveSettings(ctx, userID, exerciseID, settings); err != nil {
		if errors.Is(err, ErrNotFound) {
			return Settings{}, apperr.NotFound("種目が見つかりません。")
		}
		return Settings{}, apperr.Internal(err)
	}
	return settings, nil
}

func (s *Service) Alternatives(ctx context.Context, exerciseID, reason string) (AlternativeResponse, error) {
	contextData, err := s.repository.AlternativeContext(ctx, exerciseID)
	if errors.Is(err, ErrNotFound) {
		return AlternativeResponse{}, apperr.NotFound("代替元の種目が見つかりません。")
	}
	if err != nil {
		return AlternativeResponse{}, apperr.Internal(err)
	}
	if len(contextData.Candidates) == 0 {
		return AlternativeResponse{}, apperr.NotFound("代替種目の候補が見つかりません。")
	}
	allowed := make(map[string]AlternativeCandidate, len(contextData.Candidates))
	candidateLines := make([]string, 0, len(contextData.Candidates))
	for _, candidate := range contextData.Candidates {
		allowed[candidate.ID] = candidate
		candidateLines = append(candidateLines, fmt.Sprintf("- ID: %s, Name: %s (器具: %s)", candidate.ID, candidate.Name, candidate.Equipment))
	}
	dbContext := "以下の種目が同じ筋肉( " + strings.Join(contextData.Muscles, ", ") + " )を鍛えられるデータベース内の候補です:\n" + strings.Join(candidateLines, "\n")
	systemPrompt, userPrompt, err := s.prompts.Pair("alternative_system.txt", "alternative_user.txt", map[string]any{
		"Exercise":  contextData.ExerciseName,
		"Reason":    strings.TrimSpace(reason),
		"DBContext": dbContext,
	})
	if err != nil {
		return AlternativeResponse{}, apperr.Internal(err)
	}
	aiStarted := time.Now()
	result, err := s.aiClient.Complete(ctx, ai.Request{Task: ai.TaskAlternative, SystemPrompt: systemPrompt, UserPrompt: userPrompt, JSONMode: true})
	if err != nil {
		ai.LogFeatureOutcome(ctx, s.logger, ai.TaskAlternative, "provider_error", aiStarted)
		return AlternativeResponse{}, ai.ToAppError(err)
	}
	response := AlternativeResponse{Alternatives: []Alternative{}}
	if err := json.Unmarshal([]byte(prompt.JSONText(result)), &response); err != nil || len(response.Alternatives) == 0 {
		ai.LogFeatureOutcome(ctx, s.logger, ai.TaskAlternative, "invalid_output", aiStarted)
		return AlternativeResponse{}, apperr.Wrap(err, 502, apperr.CodeAIUnavailable, "AIの返答を解析できません。")
	}
	if err := validateAlternatives(&response, allowed); err != nil {
		ai.LogFeatureOutcome(ctx, s.logger, ai.TaskAlternative, "invalid_output", aiStarted)
		return AlternativeResponse{}, err
	}
	ai.LogFeatureOutcome(ctx, s.logger, ai.TaskAlternative, "applied", aiStarted)
	return response, nil
}

func validateAlternatives(response *AlternativeResponse, allowed map[string]AlternativeCandidate) error {
	for index := range response.Alternatives {
		candidate, exists := allowed[response.Alternatives[index].ID]
		if !exists {
			return apperr.New(502, apperr.CodeAIUnavailable, "AIが辞書外の代替種目を選択しました。")
		}
		response.Alternatives[index].Name = candidate.Name
	}
	return nil
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

func stringPointer(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func randomID() string {
	value := make([]byte, 8)
	_, _ = rand.Read(value)
	return hex.EncodeToString(value)
}
