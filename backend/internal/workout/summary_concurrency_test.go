package workout

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/ai/openaicompat"
	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
	"github.com/jPurin-gg/myfitlog-backend/internal/config"
	"github.com/jPurin-gg/myfitlog-backend/internal/prompt"
)

// Exercise the real prompts and HTTP AI adapter, with persistence in memory so
// this regression runs without PostgreSQL or an external AI service.
func TestSummaryCommentDiscardsDelayedResponseAfterCalendarEdit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	requests := make(chan string, 2)
	release := make(chan struct{})
	var unblock sync.Once
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Messages) != 2 {
			http.Error(w, "invalid AI request", http.StatusBadRequest)
			return
		}
		select {
		case requests <- body.Messages[1].Content:
		case <-r.Context().Done():
			return
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"comment\":\"生成した総評\"}"}}]}`)
	}))
	defer provider.Close()
	defer unblock.Do(func() { close(release) })

	repository := &summaryMemoryRepository{data: SummaryContext{
		Detail:      Detail{ID: 2, Status: "completed", Summary: Summary{TotalSets: 1, TotalReps: 10, TotalVolume: 400}},
		WorkoutSets: []HistorySet{{ExerciseName: "ベンチプレス", SetOrder: 1, Weight: 40, Reps: 10}},
		Version:     1,
	}}
	aiClient := openaicompat.New(config.AIConfig{
		APIKey: "local-test", URL: provider.URL, Model: "stub", MaxAttempts: 1, Timeout: 5 * time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service := NewService(repository, aiClient, prompt.NewRenderer("../../prompts"), clock.Fixed{Time: time.Now()}, 5*time.Second)
	type outcome struct {
		result SummaryCommentResult
		err    error
	}
	finished := make(chan outcome, 1)
	go func() {
		result, err := service.SummaryComment(ctx, 1, 2)
		finished <- outcome{result, err}
	}()

	assertPrompt := func(wantSets, wantVolume string) {
		t.Helper()
		select {
		case text := <-requests:
			if !strings.Contains(text, wantSets) || !strings.Contains(text, wantVolume) {
				t.Fatalf("AI prompt = %q, want %q and %q", text, wantSets, wantVolume)
			}
		case <-ctx.Done():
			t.Fatal("AI stub did not receive a request")
		}
	}
	assertPrompt("40.0kg x 10回", `"total_volume":400`)
	// The edit must finish while the AI response is still blocked.
	_, err := service.SaveCalendarWorkout(ctx, 1, "2026-11-13", CalendarWorkoutInput{
		Title: "編集済み", Sets: []CalendarSet{{ExerciseID: "bench", ExerciseName: "ベンチプレス", Weight: 55, Reps: 6, SetOrder: 1}},
	})
	if err != nil {
		t.Fatalf("SaveCalendarWorkout() error = %v", err)
	}
	unblock.Do(func() { close(release) })
	select {
	case got := <-finished:
		if got.err == nil || apperr.As(got.err).Status != http.StatusConflict || got.result.Comment != "" {
			t.Fatalf("delayed SummaryComment() = %#v, %v; want conflict without comment", got.result, got.err)
		}
	case <-ctx.Done():
		t.Fatal("delayed SummaryComment() did not finish")
	}
	detail, _ := repository.Detail(ctx, 1, 2)
	if detail.Summary.AIComment != "" {
		t.Fatalf("stale comment was persisted: %q", detail.Summary.AIComment)
	}

	result, err := service.SummaryComment(ctx, 1, 2)
	if err != nil || result.Replayed || result.Comment != "生成した総評" {
		t.Fatalf("retry SummaryComment() = %#v, %v", result, err)
	}
	assertPrompt("55.0kg x 6回", `"total_volume":330`)
	detail, _ = repository.Detail(ctx, 1, 2)
	if detail.Summary.AIComment != result.Comment {
		t.Fatalf("retry comment was not persisted: %q", detail.Summary.AIComment)
	}
	replayed, err := service.SummaryComment(ctx, 1, 2)
	if err != nil || !replayed.Replayed || replayed.Comment != result.Comment {
		t.Fatalf("cached SummaryComment() = %#v, %v", replayed, err)
	}
	if len(requests) != 0 {
		t.Fatal("cached SummaryComment() called AI again")
	}
}

type summaryMemoryRepository struct {
	Repository
	mu   sync.Mutex
	data SummaryContext
}

func (r *summaryMemoryRepository) SummaryContext(context.Context, int, int) (SummaryContext, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.data, nil
}

func (r *summaryMemoryRepository) Detail(ctx context.Context, userID, workoutID int) (Detail, error) {
	data, err := r.SummaryContext(ctx, userID, workoutID)
	return data.Detail, err
}

func (r *summaryMemoryRepository) SaveSummaryComment(_ context.Context, _, _ int, version int64, comment string) (string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.data.Version != version {
		return "", false, ErrSummaryChanged
	}
	if saved := r.data.Detail.Summary.AIComment; saved != "" {
		return saved, true, nil
	}
	r.data.Detail.Summary.AIComment = comment
	return comment, false, nil
}

func (r *summaryMemoryRepository) SaveCalendarWorkout(_ context.Context, _ int, _ time.Time, input CalendarWorkoutInput) (CalendarWorkout, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data.Version++
	r.data.Detail.Summary = Summary{TotalSets: len(input.Sets)}
	r.data.WorkoutSets = nil
	for _, set := range input.Sets {
		r.data.Detail.Summary.TotalReps += set.Reps
		r.data.Detail.Summary.TotalVolume += set.Weight * float64(set.Reps)
		r.data.WorkoutSets = append(r.data.WorkoutSets, HistorySet{
			ExerciseName: set.ExerciseName, Weight: set.Weight, Reps: set.Reps, SetOrder: set.SetOrder,
		})
	}
	return CalendarWorkout{WorkoutID: r.data.Detail.ID, Title: input.Title, Sets: input.Sets}, nil
}
