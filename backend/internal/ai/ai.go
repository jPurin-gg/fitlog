package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

type Task string

const (
	TaskRecommendation Task = "recommendation"
	TaskAlternative    Task = "alternative"
	TaskWorkoutPlan    Task = "workout_plan"
	TaskWorkoutSummary Task = "workout_summary"
	TaskMonthlyPlan    Task = "monthly_plan"
)

type Request struct {
	Task         Task
	SystemPrompt string
	UserPrompt   string
	JSONMode     bool
}

type Client interface {
	Complete(ctx context.Context, request Request) (string, error)
}

type Error struct {
	RequestID string
	Status    int
	Code      string
	Model     string
	Attempt   int
	Err       error
}

func (e *Error) Error() string {
	if e.Status > 0 {
		return fmt.Sprintf("AI request %s failed with status %d (%s)", e.RequestID, e.Status, e.Code)
	}
	return fmt.Sprintf("AI request %s failed (%s)", e.RequestID, e.Code)
}

func (e *Error) Unwrap() error { return e.Err }

func ToAppError(err error) error {
	var aiErr *Error
	if !errors.As(err, &aiErr) {
		return apperr.Wrap(err, http.StatusBadGateway, apperr.CodeAIUnavailable, "AIサービスに接続できません。")
	}
	switch aiErr.Status {
	case http.StatusTooManyRequests:
		return apperr.Wrap(err, http.StatusTooManyRequests, apperr.CodeRateLimited, "AIの利用上限に達しました。少し待ってからお試しください。")
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return apperr.Wrap(err, http.StatusBadGateway, apperr.CodeAIUnavailable, "AIの設定または権限に問題があります。")
	default:
		return apperr.Wrap(err, http.StatusBadGateway, apperr.CodeAIUnavailable, "AIサービスが一時的に利用できません。")
	}
}
