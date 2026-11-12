package body

import (
	"context"
	"net/http"
	"testing"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

type fakeRepository struct {
	createErr error
	updateErr error
	deleteErr error
}

func (f *fakeRepository) List(context.Context, int, int) ([]Measurement, error) {
	return []Measurement{}, nil
}

func (f *fakeRepository) Create(_ context.Context, _ int, input MeasurementInput) (Measurement, error) {
	if f.createErr != nil {
		return Measurement{}, f.createErr
	}
	return Measurement{ID: 1, MeasuredOn: input.MeasuredOn, WeightKg: input.WeightKg, BodyFatPercent: input.BodyFatPercent, RestingHeartRate: input.RestingHeartRate}, nil
}

func (f *fakeRepository) Update(_ context.Context, _ int, _ int, input MeasurementInput) (Measurement, error) {
	if f.updateErr != nil {
		return Measurement{}, f.updateErr
	}
	return Measurement{ID: 1, MeasuredOn: input.MeasuredOn, WeightKg: input.WeightKg}, nil
}

func (f *fakeRepository) Delete(context.Context, int, int) error { return f.deleteErr }

func validInput() MeasurementInput {
	return MeasurementInput{MeasuredOn: "2026-11-11", WeightKg: 68.5}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	service := NewService(&fakeRepository{})
	cases := []MeasurementInput{
		{MeasuredOn: "11/11/2026", WeightKg: 68.5},
		{MeasuredOn: "2026-11-11", WeightKg: 0},
		{MeasuredOn: "2026-11-11", WeightKg: -3},
		{MeasuredOn: "2026-11-11", WeightKg: 1001},
	}
	for _, input := range cases {
		_, err := service.Create(context.Background(), 1, input)
		appErr := apperr.As(err)
		if appErr == nil || appErr.Status != http.StatusBadRequest {
			t.Fatalf("input %+v: err = %v, want validation error", input, err)
		}
	}
}

func TestCreateRejectsOutOfRangeOptionals(t *testing.T) {
	service := NewService(&fakeRepository{})
	fat := 120.0
	input := validInput()
	input.BodyFatPercent = &fat
	if _, err := service.Create(context.Background(), 1, input); apperr.As(err) == nil {
		t.Fatal("body fat 120%: expected validation error")
	}
	rate := 10
	input = validInput()
	input.RestingHeartRate = &rate
	if _, err := service.Create(context.Background(), 1, input); apperr.As(err) == nil {
		t.Fatal("heart rate 10: expected validation error")
	}
}

func TestCreateMapsDuplicateDateToConflict(t *testing.T) {
	service := NewService(&fakeRepository{createErr: ErrDuplicateDate})
	_, err := service.Create(context.Background(), 1, validInput())
	appErr := apperr.As(err)
	if appErr == nil || appErr.Status != http.StatusConflict {
		t.Fatalf("err = %v, want conflict", err)
	}
}

func TestUpdateAndDeleteMapNotFound(t *testing.T) {
	service := NewService(&fakeRepository{updateErr: ErrNotFound, deleteErr: ErrNotFound})
	if _, err := service.Update(context.Background(), 1, 42, validInput()); apperr.As(err) == nil || apperr.As(err).Status != http.StatusNotFound {
		t.Fatalf("update err = %v, want not found", err)
	}
	if err := service.Delete(context.Background(), 1, 42); apperr.As(err) == nil || apperr.As(err).Status != http.StatusNotFound {
		t.Fatalf("delete err = %v, want not found", err)
	}
}
