package workout

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

func (*stubRepository) UpdateSet(context.Context, int, int, int, SetPatch) (Set, error) {
	return Set{}, nil
}

func (*stubRepository) DeleteSet(context.Context, int, int, int) error {
	return nil
}

type setEditRepository struct {
	stubRepository
	updateCalls int
	deleteCalls int
	patch       SetPatch
	updated     Set
	err         error
}

func (r *setEditRepository) UpdateSet(_ context.Context, _, _, _ int, patch SetPatch) (Set, error) {
	r.updateCalls++
	r.patch = patch
	return r.updated, r.err
}

func (r *setEditRepository) DeleteSet(context.Context, int, int, int) error {
	r.deleteCalls++
	return r.err
}

func ptr[T any](value T) *T { return &value }

func TestUpdateSetValidatesCorrectionBeforePersistence(t *testing.T) {
	tests := []struct {
		name      string
		patch     SetPatch
		wantField string
	}{
		{name: "empty_patch_rejected", patch: SetPatch{}, wantField: "body"},
		{name: "reps_zero_rejected", patch: SetPatch{Reps: ptr(0)}, wantField: "reps"},
		{name: "reps_over_max_rejected", patch: SetPatch{Reps: ptr(MaxReps + 1)}, wantField: "reps"},
		{name: "weight_negative_rejected", patch: SetPatch{Weight: ptr(-1.0)}, wantField: "weight"},
		{name: "weight_over_max_rejected", patch: SetPatch{Weight: ptr(MaxWeight + 0.5)}, wantField: "weight"},
		{name: "weight_only_accepted", patch: SetPatch{Weight: ptr(80.0)}},
		{name: "reps_only_accepted", patch: SetPatch{Reps: ptr(MaxReps)}},
		{name: "feeling_only_accepted", patch: SetPatch{Feeling: ptr("  きつい  ")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &setEditRepository{updated: Set{ID: 7}}
			service := NewService(repository, nil, nil, nil, 0)
			set, err := service.UpdateSet(context.Background(), 1, 2, 7, tt.patch)
			if tt.wantField != "" {
				appErr := apperr.As(err)
				if appErr.Status != http.StatusBadRequest || appErr.Fields[tt.wantField] == "" || repository.updateCalls != 0 {
					t.Fatalf("UpdateSet() error = %#v; repository calls = %d", appErr, repository.updateCalls)
				}
				return
			}
			if err != nil || set.ID != 7 || repository.updateCalls != 1 {
				t.Fatalf("UpdateSet() = %#v, %v; repository calls = %d", set, err, repository.updateCalls)
			}
		})
	}
}

func TestUpdateSetTrimsFeeling(t *testing.T) {
	repository := &setEditRepository{}
	service := NewService(repository, nil, nil, nil, 0)
	if _, err := service.UpdateSet(context.Background(), 1, 2, 7, SetPatch{Feeling: ptr("  余裕  ")}); err != nil {
		t.Fatal(err)
	}
	if repository.patch.Feeling == nil || *repository.patch.Feeling != "余裕" || repository.patch.Weight != nil || repository.patch.Reps != nil {
		t.Fatalf("UpdateSet() patch = %#v", repository.patch)
	}
}

func TestSetCorrectionMapsRepositoryErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "not_found", err: ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "completed_workout", err: ErrConflict, wantStatus: http.StatusConflict},
		{name: "internal", err: errors.New("database down"), wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &setEditRepository{err: tt.err}
			service := NewService(repository, nil, nil, nil, 0)
			_, updateErr := service.UpdateSet(context.Background(), 1, 2, 7, SetPatch{Reps: ptr(8)})
			if appErr := apperr.As(updateErr); appErr.Status != tt.wantStatus {
				t.Fatalf("UpdateSet() error = %#v", appErr)
			}
			if appErr := apperr.As(service.DeleteSet(context.Background(), 1, 2, 7)); appErr.Status != tt.wantStatus {
				t.Fatalf("DeleteSet() error = %#v", appErr)
			}
		})
	}
}

func TestDeleteSetDelegatesToRepository(t *testing.T) {
	repository := &setEditRepository{}
	service := NewService(repository, nil, nil, nil, 0)
	if err := service.DeleteSet(context.Background(), 1, 2, 7); err != nil || repository.deleteCalls != 1 {
		t.Fatalf("DeleteSet() = %v; repository calls = %d", err, repository.deleteCalls)
	}
}
