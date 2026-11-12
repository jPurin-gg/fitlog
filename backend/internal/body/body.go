package body

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

var (
	ErrNotFound      = errors.New("body measurement not found")
	ErrDuplicateDate = errors.New("body measurement already exists for the date")
)

// Measurement is one day's body data recorded by the user.
// Optional fields stay nil when the user did not record them.
type Measurement struct {
	ID               int      `json:"id"`
	MeasuredOn       string   `json:"measured_on"`
	WeightKg         float64  `json:"weight_kg"`
	BodyFatPercent   *float64 `json:"body_fat_percent,omitempty"`
	RestingHeartRate *int     `json:"resting_heart_rate,omitempty"`
}

type MeasurementInput struct {
	MeasuredOn       string   `json:"measured_on"`
	WeightKg         float64  `json:"weight_kg"`
	BodyFatPercent   *float64 `json:"body_fat_percent,omitempty"`
	RestingHeartRate *int     `json:"resting_heart_rate,omitempty"`
}

type Repository interface {
	List(ctx context.Context, userID int, limit int) ([]Measurement, error)
	Create(ctx context.Context, userID int, input MeasurementInput) (Measurement, error)
	Update(ctx context.Context, userID, id int, input MeasurementInput) (Measurement, error)
	Delete(ctx context.Context, userID, id int) error
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context, userID int) ([]Measurement, error) {
	measurements, err := s.repository.List(ctx, userID, 90)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if measurements == nil {
		measurements = []Measurement{}
	}
	return measurements, nil
}

func (s *Service) Create(ctx context.Context, userID int, input MeasurementInput) (Measurement, error) {
	if err := Validate(input); err != nil {
		return Measurement{}, err
	}
	measurement, err := s.repository.Create(ctx, userID, input)
	if err != nil {
		if errors.Is(err, ErrDuplicateDate) {
			return Measurement{}, apperr.Conflict("その日付の記録はすでにあります。編集してください。")
		}
		return Measurement{}, apperr.Internal(err)
	}
	return measurement, nil
}

func (s *Service) Update(ctx context.Context, userID, id int, input MeasurementInput) (Measurement, error) {
	if id <= 0 {
		return Measurement{}, apperr.Validation("記録の指定が不正です。", nil)
	}
	if err := Validate(input); err != nil {
		return Measurement{}, err
	}
	measurement, err := s.repository.Update(ctx, userID, id, input)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Measurement{}, apperr.NotFound("記録が見つかりません。")
		}
		if errors.Is(err, ErrDuplicateDate) {
			return Measurement{}, apperr.Conflict("その日付の記録はすでにあります。")
		}
		return Measurement{}, apperr.Internal(err)
	}
	return measurement, nil
}

func (s *Service) Delete(ctx context.Context, userID, id int) error {
	if id <= 0 {
		return apperr.Validation("記録の指定が不正です。", nil)
	}
	if err := s.repository.Delete(ctx, userID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return apperr.NotFound("記録が見つかりません。")
		}
		return apperr.Internal(err)
	}
	return nil
}

func Validate(input MeasurementInput) error {
	if _, err := time.Parse("2006-01-02", input.MeasuredOn); err != nil {
		return apperr.Validation("日付はYYYY-MM-DD形式で指定してください。", nil)
	}
	if math.IsNaN(input.WeightKg) || math.IsInf(input.WeightKg, 0) || input.WeightKg <= 0 || input.WeightKg > 1000 {
		return apperr.Validation("体重は0より大きく1000kg以下で指定してください。", nil)
	}
	if input.BodyFatPercent != nil {
		value := *input.BodyFatPercent
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
			return apperr.Validation("体脂肪率は0から100の間で指定してください。", nil)
		}
	}
	if input.RestingHeartRate != nil {
		value := *input.RestingHeartRate
		if value < 20 || value > 250 {
			return apperr.Validation("安静時心拍数は20から250の間で指定してください。", nil)
		}
	}
	return nil
}
