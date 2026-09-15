package workout

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
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

func TestRecordSetValidatesInput(t *testing.T) {
	valid := SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 80, Reps: 10}
	tests := []struct {
		name           string
		key            string
		workoutID      int
		input          SetInput
		wantField      string
		wantKey        string
		wantExerciseID string
	}{
		{name: "key_7_chars_rejected", key: "abcdefg", workoutID: 2, input: valid, wantField: "Idempotency-Key"},
		{name: "key_8_chars_accepted", key: "abcdefgh", workoutID: 2, input: valid, wantKey: "abcdefgh", wantExerciseID: "bench"},
		{name: "key_128_chars_accepted", key: strings.Repeat("k", 128), workoutID: 2, input: valid, wantKey: strings.Repeat("k", 128), wantExerciseID: "bench"},
		{name: "key_129_chars_rejected", key: strings.Repeat("k", 129), workoutID: 2, input: valid, wantField: "Idempotency-Key"},
		{name: "key_with_space_rejected", key: "request 123", workoutID: 2, input: valid, wantField: "Idempotency-Key"},
		{name: "key_with_symbol_rejected", key: "request!123", workoutID: 2, input: valid, wantField: "Idempotency-Key"},
		{name: "key_surrounding_whitespace_trimmed", key: "  request_123\n", workoutID: 2, input: valid, wantKey: "request_123", wantExerciseID: "bench"},
		{name: "workout_id_zero_rejected", key: "request_123", workoutID: 0, input: valid, wantField: "workout_id"},
		{name: "workout_id_negative_rejected", key: "request_123", workoutID: -1, input: valid, wantField: "workout_id"},
		{name: "exercise_id_empty_rejected", key: "request_123", workoutID: 2, input: SetInput{ExerciseID: "", SetOrder: 1, Weight: 80, Reps: 10}, wantField: "exercise_id"},
		{name: "exercise_id_blank_rejected", key: "request_123", workoutID: 2, input: SetInput{ExerciseID: "   ", SetOrder: 1, Weight: 80, Reps: 10}, wantField: "exercise_id"},
		{name: "exercise_id_surrounding_whitespace_trimmed", key: "request_123", workoutID: 2, input: SetInput{ExerciseID: " bench ", SetOrder: 1, Weight: 80, Reps: 10}, wantKey: "request_123", wantExerciseID: "bench"},
		{name: "set_order_zero_rejected", key: "request_123", workoutID: 2, input: SetInput{ExerciseID: "bench", SetOrder: 0, Weight: 80, Reps: 10}, wantField: "set_order"},
		{name: "set_order_negative_rejected", key: "request_123", workoutID: 2, input: SetInput{ExerciseID: "bench", SetOrder: -1, Weight: 80, Reps: 10}, wantField: "set_order"},
		{name: "weight_negative_rejected", key: "request_123", workoutID: 2, input: SetInput{ExerciseID: "bench", SetOrder: 1, Weight: -0.5, Reps: 10}, wantField: "weight"},
		{name: "weight_zero_accepted", key: "request_123", workoutID: 2, input: SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 0, Reps: 10}, wantKey: "request_123", wantExerciseID: "bench"},
		{name: "reps_zero_rejected", key: "request_123", workoutID: 2, input: SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 80, Reps: 0}, wantField: "reps"},
		{name: "reps_negative_rejected", key: "request_123", workoutID: 2, input: SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 80, Reps: -1}, wantField: "reps"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &stubRepository{}
			service := NewService(repository, nil, nil, nil, 0)
			_, _, err := service.RecordSet(context.Background(), 1, tt.workoutID, tt.key, tt.input)
			if tt.wantField != "" {
				appErr := apperr.As(err)
				if appErr == nil || appErr.Status != http.StatusBadRequest || appErr.Code != apperr.CodeValidation || appErr.Fields[tt.wantField] == "" || repository.recordCalls != 0 {
					t.Fatalf("RecordSet() error = %#v; repository calls = %d", appErr, repository.recordCalls)
				}
				return
			}
			if err != nil || repository.recordCalls != 1 || repository.key != tt.wantKey || repository.input.ExerciseID != tt.wantExerciseID {
				t.Fatalf("RecordSet() error = %v; repository calls = %d; key = %q; input = %#v", err, repository.recordCalls, repository.key, repository.input)
			}
		})
	}
}

func TestRecordSetMapsRepositoryErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "not_found", err: ErrNotFound, wantStatus: http.StatusNotFound, wantCode: apperr.CodeNotFound},
		{name: "wrapped_not_found", err: fmt.Errorf("query: %w", ErrNotFound), wantStatus: http.StatusNotFound, wantCode: apperr.CodeNotFound},
		{name: "conflict", err: ErrConflict, wantStatus: http.StatusConflict, wantCode: apperr.CodeConflict},
		{name: "wrapped_conflict", err: fmt.Errorf("insert: %w", ErrConflict), wantStatus: http.StatusConflict, wantCode: apperr.CodeConflict},
		{name: "other", err: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantCode: apperr.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &stubRepository{recordErr: tt.err}
			service := NewService(repository, nil, nil, nil, 0)
			_, _, err := service.RecordSet(context.Background(), 1, 2, "request_123", SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 80, Reps: 10})
			appErr := apperr.As(err)
			if appErr == nil || appErr.Status != tt.wantStatus || appErr.Code != tt.wantCode || repository.recordCalls != 1 {
				t.Fatalf("RecordSet() error = %#v; repository calls = %d", appErr, repository.recordCalls)
			}
		})
	}
}

func TestRecordSetPropagatesReplayedFlag(t *testing.T) {
	repository := &stubRepository{recorded: Set{ID: 10, WorkoutID: 2, ExerciseID: "bench", SetOrder: 1, Weight: 80, Reps: 10}, recordReplayed: true}
	service := NewService(repository, nil, nil, nil, 0)
	result, replayed, err := service.RecordSet(context.Background(), 1, 2, "request_123", SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 80, Reps: 10})
	if err != nil || !replayed || result.ID != 10 {
		t.Fatalf("RecordSet() = %#v, %v, %v", result, replayed, err)
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

func TestValidateRecommendationCoversEveryRule(t *testing.T) {
	tests := []struct {
		name       string
		input      Recommendation
		wantErr    bool
		wantAction string
	}{
		{name: "normalizes_adjust", input: Recommendation{NextAction: " adjust ", Recommendation: "次は軽く", Reason: "疲労", TargetWeight: 20, TargetReps: 8}, wantAction: "ADJUST"},
		{name: "normalizes_stop", input: Recommendation{NextAction: "stop", Recommendation: "終了", Reason: "疲労", TargetWeight: 20, TargetReps: 8}, wantAction: "STOP"},
		{name: "normalizes_continue", input: Recommendation{NextAction: "continue", Recommendation: "続行", Reason: "問題なし", TargetWeight: 20, TargetReps: 8}, wantAction: "CONTINUE"},
		{name: "invalid_action_rejected", input: Recommendation{NextAction: "PAUSE", Recommendation: "休憩", Reason: "疲労", TargetWeight: 20, TargetReps: 8}, wantErr: true},
		{name: "empty_action_rejected", input: Recommendation{NextAction: "", Recommendation: "続行", Reason: "問題なし", TargetWeight: 20, TargetReps: 8}, wantErr: true},
		{name: "empty_recommendation_rejected", input: Recommendation{NextAction: "CONTINUE", Recommendation: "", Reason: "問題なし", TargetWeight: 20, TargetReps: 8}, wantErr: true},
		{name: "blank_recommendation_rejected", input: Recommendation{NextAction: "CONTINUE", Recommendation: "   ", Reason: "問題なし", TargetWeight: 20, TargetReps: 8}, wantErr: true},
		{name: "empty_reason_rejected", input: Recommendation{NextAction: "CONTINUE", Recommendation: "続行", Reason: "", TargetWeight: 20, TargetReps: 8}, wantErr: true},
		{name: "blank_reason_rejected", input: Recommendation{NextAction: "CONTINUE", Recommendation: "続行", Reason: "  ", TargetWeight: 20, TargetReps: 8}, wantErr: true},
		{name: "negative_weight_rejected", input: Recommendation{NextAction: "CONTINUE", Recommendation: "続行", Reason: "問題なし", TargetWeight: -1, TargetReps: 8}, wantErr: true},
		{name: "weight_above_2000_rejected", input: Recommendation{NextAction: "CONTINUE", Recommendation: "続行", Reason: "問題なし", TargetWeight: 2001, TargetReps: 8}, wantErr: true},
		{name: "weight_2000_accepted", input: Recommendation{NextAction: "CONTINUE", Recommendation: "続行", Reason: "問題なし", TargetWeight: 2000, TargetReps: 8}, wantAction: "CONTINUE"},
		{name: "negative_reps_rejected", input: Recommendation{NextAction: "CONTINUE", Recommendation: "続行", Reason: "問題なし", TargetWeight: 20, TargetReps: -1}, wantErr: true},
		{name: "reps_above_1000_rejected", input: Recommendation{NextAction: "CONTINUE", Recommendation: "続行", Reason: "問題なし", TargetWeight: 20, TargetReps: 1001}, wantErr: true},
		{name: "reps_1000_accepted", input: Recommendation{NextAction: "CONTINUE", Recommendation: "続行", Reason: "問題なし", TargetWeight: 20, TargetReps: 1000}, wantAction: "CONTINUE"},
		{name: "stop_with_zero_reps_accepted", input: Recommendation{NextAction: "STOP", Recommendation: "終了", Reason: "疲労", TargetWeight: 0, TargetReps: 0}, wantAction: "STOP"},
		{name: "continue_with_zero_reps_rejected", input: Recommendation{NextAction: "CONTINUE", Recommendation: "続行", Reason: "問題なし", TargetWeight: 20, TargetReps: 0}, wantErr: true},
		{name: "adjust_with_zero_reps_rejected", input: Recommendation{NextAction: "ADJUST", Recommendation: "調整", Reason: "疲労", TargetWeight: 20, TargetReps: 0}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := tt.input
			err := validateRecommendation(&response)
			if tt.wantErr {
				appErr := apperr.As(err)
				if appErr == nil || appErr.Status != http.StatusBadGateway || appErr.Code != apperr.CodeAIUnavailable {
					t.Fatalf("validateRecommendation(%#v) error = %#v", tt.input, appErr)
				}
				return
			}
			if err != nil || response.NextAction != tt.wantAction {
				t.Fatalf("validateRecommendation(%#v) = %#v, %v", tt.input, response, err)
			}
		})
	}
}

func TestRecommendMapsRepositoryErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "not_found", err: ErrNotFound, wantStatus: http.StatusNotFound, wantCode: apperr.CodeNotFound},
		{name: "wrapped_not_found", err: fmt.Errorf("query: %w", ErrNotFound), wantStatus: http.StatusNotFound, wantCode: apperr.CodeNotFound},
		{name: "other", err: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantCode: apperr.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &stubRepository{recommendationErr: tt.err}
			aiClient := &stubAIClient{}
			service := NewService(repository, aiClient, stubPrompts{}, nil, time.Second)

			_, err := service.Recommend(context.Background(), 1, 2, 3)
			appErr := apperr.As(err)
			if appErr == nil || appErr.Status != tt.wantStatus || appErr.Code != tt.wantCode {
				t.Fatalf("Recommend() error = %#v", appErr)
			}
			if aiClient.calls != 0 {
				t.Fatalf("Recommend() AI calls = %d", aiClient.calls)
			}
		})
	}
}

func TestRecommendRejectsInvalidAIOutput(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantDetail string
	}{
		{name: "non_json", output: "もう1セットいけます", wantDetail: "AIの提案を解析できません。"},
		{name: "invalid_action", output: `{"next_action":"PAUSE","recommendation":"休憩","target_weight":40,"target_reps":8,"reason":"疲労"}`, wantDetail: "AIの提案が不正です。"},
		{name: "continue_without_reps", output: `{"next_action":"CONTINUE","recommendation":"続行","target_weight":40,"target_reps":0,"reason":"問題なし"}`, wantDetail: "AIの提案が不完全です。"},
		{name: "weight_above_limit", output: `{"next_action":"CONTINUE","recommendation":"続行","target_weight":2001,"target_reps":8,"reason":"問題なし"}`, wantDetail: "AIの提案が不完全です。"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &stubRepository{recommendation: RecommendationContext{
				Set:          Set{ID: 3, WorkoutID: 2, ExerciseID: "bench", SetOrder: 1, Weight: 40, Reps: 10},
				ExerciseName: "ベンチプレス",
				MaxWeight:    77.5,
			}}
			aiClient := &stubAIClient{complete: func(context.Context, ai.Request) (string, error) {
				return tt.output, nil
			}}
			service := NewService(repository, aiClient, stubPrompts{}, nil, time.Second)

			_, err := service.Recommend(context.Background(), 1, 2, 3)
			appErr := apperr.As(err)
			if appErr == nil || appErr.Status != http.StatusBadGateway || appErr.Code != apperr.CodeAIUnavailable || appErr.Detail != tt.wantDetail {
				t.Fatalf("Recommend() error = %#v", appErr)
			}
			if aiClient.calls != 1 {
				t.Fatalf("Recommend() AI calls = %d", aiClient.calls)
			}
		})
	}
}

func TestRecommendReturnsParsedRecommendationWithContextMaxWeight(t *testing.T) {
	repository := &stubRepository{recommendation: RecommendationContext{
		Set:          Set{ID: 3, WorkoutID: 2, ExerciseID: "bench", SetOrder: 1, Weight: 40, Reps: 10},
		ExerciseName: "ベンチプレス",
		MaxWeight:    77.5,
	}}
	var request ai.Request
	aiClient := &stubAIClient{complete: func(_ context.Context, received ai.Request) (string, error) {
		request = received
		return `{"next_action":" continue ","recommendation":" 同じ重量で続行 ","target_weight":42.5,"target_reps":8,"reason":" 余裕あり ","record_template":"42.5kg x 8","max_weight":999}`, nil
	}}
	service := NewService(repository, aiClient, stubPrompts{}, nil, time.Second)

	result, err := service.Recommend(context.Background(), 1, 2, 3)
	want := Recommendation{NextAction: "CONTINUE", Recommendation: "同じ重量で続行", TargetWeight: 42.5, TargetReps: 8, Reason: "余裕あり", RecordTemplate: "42.5kg x 8", MaxWeight: 77.5}
	if err != nil || result != want {
		t.Fatalf("Recommend() = %#v, %v; want %#v", result, err, want)
	}
	if aiClient.calls != 1 || request.Task != ai.TaskRecommendation || !request.JSONMode || request.SystemPrompt != "recommend_system.txt" || request.UserPrompt != "recommend_user.txt" {
		t.Fatalf("Recommend() AI calls = %d; request = %#v", aiClient.calls, request)
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
	input               SetInput
	recorded            Set
	recordReplayed      bool
	recordErr           error
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

func (r *stubRepository) RecordSet(_ context.Context, _, _ int, key string, input SetInput) (Set, bool, error) {
	r.recordCalls++
	r.key = key
	r.input = input
	return r.recorded, r.recordReplayed, r.recordErr
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
