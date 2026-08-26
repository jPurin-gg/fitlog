package workout

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/ai"
	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

func TestRecordSetRequiresIdempotencyKey(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository, nil, nil, nil, 0)
	_, _, err := service.RecordSet(context.Background(), 1, 2, "short", SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 80, Reps: 10})
	appErr := apperr.As(err)
	if appErr.Status != 400 || repository.recordCalls != 0 {
		t.Fatalf("RecordSet() error = %#v; repository calls = %d", appErr, repository.recordCalls)
	}
}

func TestRecordSetPassesStableKeyToRepository(t *testing.T) {
	repository := &stubRepository{recorded: Set{ID: 10, WorkoutID: 2, ExerciseID: "bench", SetOrder: 1, Weight: 80, Reps: 10}}
	service := NewService(repository, nil, nil, nil, 0)
	result, replayed, err := service.RecordSet(context.Background(), 1, 2, "request_123", SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 80, Reps: 10})
	if err != nil || replayed || result.ID != 10 || repository.key != "request_123" {
		t.Fatalf("RecordSet() = %#v, %v, %v; key = %q", result, replayed, err, repository.key)
	}
}

func TestValidateRecommendationNormalizesActionAndRejectsInvalidTargets(t *testing.T) {
	valid := Recommendation{NextAction: " adjust ", Recommendation: " 次は軽く ", Reason: " 疲労 ", TargetWeight: 20, TargetReps: 8}
	if err := validateRecommendation(&valid); err != nil || valid.NextAction != "ADJUST" {
		t.Fatalf("validateRecommendation(valid) = %#v, %v", valid, err)
	}
	invalid := Recommendation{NextAction: "CONTINUE", Recommendation: "続行", Reason: "問題なし", TargetWeight: -1, TargetReps: 8}
	if err := validateRecommendation(&invalid); err == nil {
		t.Fatal("validateRecommendation(negative weight) error = nil")
	}
}

func TestFinishDoesNotRequestAI(t *testing.T) {
	repository := &stubRepository{finished: Detail{ID: 2, Status: "completed", Summary: Summary{TotalSets: 3}}}
	aiClient := &stubAIClient{complete: func(context.Context, ai.Request) (string, error) {
		t.Fatal("Finish() must not request an AI summary")
		return "", nil
	}}
	service := NewService(repository, aiClient, nil, nil, time.Second)

	result, err := service.Finish(context.Background(), 1, 2)
	if err != nil || result.Status != "completed" || result.Summary.TotalSets != 3 {
		t.Fatalf("Finish() = %#v, %v", result, err)
	}
	if aiClient.calls != 0 || repository.recommendationCalls != 0 {
		t.Fatalf("Finish() AI calls = %d; context calls = %d", aiClient.calls, repository.recommendationCalls)
	}
}

func TestDetailDoesNotRequestAI(t *testing.T) {
	repository := &stubRepository{detail: Detail{ID: 2, Title: "Workout", Status: "completed"}}
	aiClient := &stubAIClient{complete: func(context.Context, ai.Request) (string, error) {
		t.Fatal("Detail() must not request an AI summary")
		return "", nil
	}}
	service := NewService(repository, aiClient, nil, nil, time.Second)

	result, err := service.Detail(context.Background(), 1, 2)
	if err != nil || result.Title != "ワークアウト" {
		t.Fatalf("Detail() = %#v, %v", result, err)
	}
	if aiClient.calls != 0 || repository.recommendationCalls != 0 {
		t.Fatalf("Detail() AI calls = %d; context calls = %d", aiClient.calls, repository.recommendationCalls)
	}
}

func TestSummaryCommentReusesStoredComment(t *testing.T) {
	repository := &stubRepository{detail: Detail{ID: 2, Status: "completed", Summary: Summary{AIComment: "  よいトレーニングでした。  "}}}
	aiClient := &stubAIClient{}
	service := NewService(repository, aiClient, nil, nil, time.Second)

	result, err := service.SummaryComment(context.Background(), 1, 2)
	if err != nil || result.Comment != "よいトレーニングでした。" || !result.Replayed {
		t.Fatalf("SummaryComment() = %#v, %v", result, err)
	}
	if aiClient.calls != 0 || repository.saveSummaryCalls != 0 {
		t.Fatalf("SummaryComment() AI calls = %d; save calls = %d", aiClient.calls, repository.saveSummaryCalls)
	}
}

func TestSummaryCommentRequiresCompletedWorkout(t *testing.T) {
	repository := &stubRepository{detail: Detail{ID: 2, Status: "active"}}
	service := NewService(repository, &stubAIClient{}, nil, nil, time.Second)

	_, err := service.SummaryComment(context.Background(), 1, 2)
	if appErr := apperr.As(err); appErr.Status != http.StatusConflict {
		t.Fatalf("SummaryComment() error = %#v", appErr)
	}
}

func TestSummaryCommentGeneratesAndSavesComment(t *testing.T) {
	repository := &stubRepository{
		detail:              Detail{ID: 2, Status: "completed", Summary: Summary{TotalSets: 3}},
		recommendation:      RecommendationContext{WorkoutSets: []HistorySet{{ExerciseName: "ベンチプレス", SetOrder: 1, Weight: 40, Reps: 10}}},
		savedSummaryComment: "その調子で続けましょう。",
	}
	aiClient := &stubAIClient{complete: func(_ context.Context, request ai.Request) (string, error) {
		if request.Task != ai.TaskWorkoutSummary {
			t.Fatalf("AI task = %q", request.Task)
		}
		return `{"comment":" その調子で続けましょう。 "}`, nil
	}}
	service := NewService(repository, aiClient, stubPrompts{}, nil, time.Second)

	result, err := service.SummaryComment(context.Background(), 1, 2)
	if err != nil || result.Comment != "その調子で続けましょう。" || result.Replayed {
		t.Fatalf("SummaryComment() = %#v, %v", result, err)
	}
	if repository.summaryCommentInput != "その調子で続けましょう。" || repository.saveSummaryCalls != 1 {
		t.Fatalf("saved comment = %q; calls = %d", repository.summaryCommentInput, repository.saveSummaryCalls)
	}
}

func TestSummaryCommentAIFailureDoesNotSave(t *testing.T) {
	repository := &stubRepository{detail: Detail{ID: 2, Status: "completed"}}
	aiClient := &stubAIClient{complete: func(context.Context, ai.Request) (string, error) {
		return "", &ai.Error{Status: http.StatusTooManyRequests, Code: "rate_limit"}
	}}
	service := NewService(repository, aiClient, stubPrompts{}, nil, time.Second)

	_, err := service.SummaryComment(context.Background(), 1, 2)
	if appErr := apperr.As(err); appErr.Status != http.StatusTooManyRequests || appErr.Code != apperr.CodeRateLimited {
		t.Fatalf("SummaryComment() error = %#v", appErr)
	}
	if repository.saveSummaryCalls != 0 {
		t.Fatalf("SaveSummaryComment() calls = %d", repository.saveSummaryCalls)
	}
}

func TestSummaryCommentReturnsCommentSavedByConcurrentRequestWhenAIFails(t *testing.T) {
	repository := &stubRepository{detail: Detail{ID: 2, Status: "completed"}}
	aiClient := &stubAIClient{complete: func(context.Context, ai.Request) (string, error) {
		repository.detail.Summary.AIComment = "先に完了した総評です。"
		return "", &ai.Error{Status: http.StatusTooManyRequests, Code: "rate_limit"}
	}}
	service := NewService(repository, aiClient, stubPrompts{}, nil, time.Second)

	result, err := service.SummaryComment(context.Background(), 1, 2)
	if err != nil || result.Comment != "先に完了した総評です。" || !result.Replayed {
		t.Fatalf("SummaryComment() = %#v, %v", result, err)
	}
	if repository.saveSummaryCalls != 0 {
		t.Fatalf("SaveSummaryComment() calls = %d", repository.saveSummaryCalls)
	}
}

func TestSummaryCommentHonorsOptionalAITimeout(t *testing.T) {
	repository := &stubRepository{detail: Detail{ID: 2, Status: "completed"}}
	aiClient := &stubAIClient{complete: func(ctx context.Context, _ ai.Request) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}}
	service := NewService(repository, aiClient, stubPrompts{}, nil, time.Millisecond)

	_, err := service.SummaryComment(context.Background(), 1, 2)
	if appErr := apperr.As(err); appErr.Status != http.StatusBadGateway || appErr.Code != apperr.CodeAIUnavailable {
		t.Fatalf("SummaryComment() error = %#v", appErr)
	}
	if repository.saveSummaryCalls != 0 {
		t.Fatalf("SaveSummaryComment() calls = %d", repository.saveSummaryCalls)
	}
}

func TestRecommendHonorsOptionalAITimeout(t *testing.T) {
	repository := &stubRepository{recommendation: RecommendationContext{
		Set:          Set{ID: 3, WorkoutID: 2, ExerciseID: "bench", SetOrder: 1, Weight: 40, Reps: 10},
		ExerciseName: "ベンチプレス",
	}}
	aiClient := &stubAIClient{complete: func(ctx context.Context, _ ai.Request) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}}
	service := NewService(repository, aiClient, stubPrompts{}, nil, time.Millisecond)

	_, err := service.Recommend(context.Background(), 1, 2, 3)
	if appErr := apperr.As(err); appErr.Status != http.StatusBadGateway || appErr.Code != apperr.CodeAIUnavailable {
		t.Fatalf("Recommend() error = %#v", appErr)
	}
}

type stubRepository struct {
	recordCalls         int
	key                 string
	recorded            Set
	recommendation      RecommendationContext
	recommendationErr   error
	recommendationCalls int
	finished            Detail
	finishErr           error
	detail              Detail
	detailErr           error
	savedSummaryComment string
	saveSummaryReplayed bool
	saveSummaryErr      error
	saveSummaryCalls    int
	summaryCommentInput string
}

func (r *stubRepository) RecordSet(_ context.Context, _, _ int, key string, _ SetInput) (Set, bool, error) {
	r.recordCalls++
	r.key = key
	return r.recorded, false, nil
}

func (r *stubRepository) RecommendationContext(context.Context, int, int, int) (RecommendationContext, error) {
	r.recommendationCalls++
	return r.recommendation, r.recommendationErr
}

func (r *stubRepository) Finish(context.Context, int, int) (Detail, error) {
	return r.finished, r.finishErr
}

func (r *stubRepository) Detail(context.Context, int, int) (Detail, error) {
	return r.detail, r.detailErr
}

func (r *stubRepository) SaveSummaryComment(_ context.Context, _, _ int, comment string) (string, bool, error) {
	r.saveSummaryCalls++
	r.summaryCommentInput = comment
	if r.savedSummaryComment == "" {
		r.savedSummaryComment = comment
	}
	return r.savedSummaryComment, r.saveSummaryReplayed, r.saveSummaryErr
}

func (*stubRepository) CalendarWorkout(context.Context, int, time.Time) (CalendarWorkout, error) {
	return CalendarWorkout{}, nil
}

func (*stubRepository) SaveCalendarWorkout(context.Context, int, time.Time, CalendarWorkoutInput) (CalendarWorkout, error) {
	return CalendarWorkout{}, nil
}

type stubAIClient struct {
	complete func(context.Context, ai.Request) (string, error)
	calls    int
}

func (c *stubAIClient) Complete(ctx context.Context, request ai.Request) (string, error) {
	c.calls++
	if c.complete == nil {
		return "", errors.New("unexpected AI request")
	}
	return c.complete(ctx, request)
}

type stubPrompts struct{}

func (stubPrompts) Pair(systemFilename, userFilename string, _ any) (string, string, error) {
	return systemFilename, userFilename, nil
}
