//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/lib/pq"

	"github.com/jPurin-gg/myfitlog-backend/internal/database"
	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
	workoutpostgres "github.com/jPurin-gg/myfitlog-backend/internal/workout/postgres"
)

const defaultIntegrationDSN = "host=127.0.0.1 port=55432 user=fitlog_test password=fitlog_test dbname=fitlog_test sslmode=disable"

type integrationFixture struct {
	db         *sql.DB
	repository *workoutpostgres.Repository
	userID     int
	exerciseID string
}

func TestRecordSetConcurrentReplay(t *testing.T) {
	fixture := newIntegrationFixture(t)
	workoutID := fixture.createWorkout(t)
	input := workout.SetInput{ExerciseID: fixture.exerciseID, SetOrder: 1, Weight: 42.5, Reps: 8, Feeling: "余裕"}

	const requests = 12
	start := make(chan struct{})
	results := make(chan workout.Set, requests)
	errorsFound := make(chan error, requests)
	var wait sync.WaitGroup
	for range requests {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			set, _, err := fixture.repository.RecordSet(context.Background(), fixture.userID, workoutID, "concurrent_replay_key", input)
			if err != nil {
				errorsFound <- err
				return
			}
			results <- set
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsFound)

	for err := range errorsFound {
		t.Fatalf("RecordSet() concurrent replay error = %v", err)
	}
	firstID := 0
	for result := range results {
		if firstID == 0 {
			firstID = result.ID
		}
		if result.ID != firstID {
			t.Fatalf("RecordSet() IDs differ: got %d, want %d", result.ID, firstID)
		}
	}
	if firstID == 0 {
		t.Fatal("RecordSet() returned no result")
	}

	var count int
	if err := fixture.db.QueryRow(`SELECT COUNT(*) FROM workout_sets WHERE workout_id=$1`, workoutID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("workout_sets count = %d, want 1", count)
	}
}

func TestRecordSetReplayStillWorksAfterFinish(t *testing.T) {
	fixture := newIntegrationFixture(t)
	workoutID := fixture.createWorkout(t)
	input := workout.SetInput{ExerciseID: fixture.exerciseID, SetOrder: 1, Weight: 30, Reps: 10}

	created, replayed, err := fixture.repository.RecordSet(context.Background(), fixture.userID, workoutID, "finished_replay_key", input)
	if err != nil || replayed {
		t.Fatalf("first RecordSet() = (%+v, %v, %v), want created set", created, replayed, err)
	}
	if _, err := fixture.repository.Finish(context.Background(), fixture.userID, workoutID); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	replayedSet, replayed, err := fixture.repository.RecordSet(context.Background(), fixture.userID, workoutID, "finished_replay_key", input)
	if err != nil || !replayed {
		t.Fatalf("replayed RecordSet() = (%+v, %v, %v), want replay", replayedSet, replayed, err)
	}
	if replayedSet.ID != created.ID {
		t.Fatalf("replayed set ID = %d, want %d", replayedSet.ID, created.ID)
	}
}

func TestRecordSetAndFinishSerialize(t *testing.T) {
	fixture := newIntegrationFixture(t)

	for iteration := range 20 {
		workoutID := fixture.createWorkout(t)
		input := workout.SetInput{ExerciseID: fixture.exerciseID, SetOrder: 1, Weight: float64(20 + iteration), Reps: 10}
		start := make(chan struct{})
		var wait sync.WaitGroup
		var recordErr, finishErr error
		var finished workout.Detail

		wait.Add(2)
		go func() {
			defer wait.Done()
			<-start
			_, _, recordErr = fixture.repository.RecordSet(context.Background(), fixture.userID, workoutID, fmt.Sprintf("finish_race_%08d", iteration), input)
		}()
		go func() {
			defer wait.Done()
			<-start
			finished, finishErr = fixture.repository.Finish(context.Background(), fixture.userID, workoutID)
		}()
		close(start)
		wait.Wait()

		if finishErr != nil {
			t.Fatalf("iteration %d: Finish() error = %v", iteration, finishErr)
		}
		var count int
		if err := fixture.db.QueryRow(`SELECT COUNT(*) FROM workout_sets WHERE workout_id=$1`, workoutID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if recordErr == nil {
			if count != 1 || finished.Summary.TotalSets != 1 {
				t.Fatalf("iteration %d: successful set was not included in finish: count=%d summary=%d", iteration, count, finished.Summary.TotalSets)
			}
			continue
		}
		if !errors.Is(recordErr, workout.ErrNotFound) {
			t.Fatalf("iteration %d: RecordSet() error = %v, want ErrNotFound", iteration, recordErr)
		}
		if count != 0 || finished.Summary.TotalSets != 0 {
			t.Fatalf("iteration %d: rejected set was persisted: count=%d summary=%d", iteration, count, finished.Summary.TotalSets)
		}
	}
}

func TestSaveSummaryCommentConcurrentReplay(t *testing.T) {
	fixture := newIntegrationFixture(t)
	workoutID := fixture.createWorkout(t)
	if _, err := fixture.repository.Finish(context.Background(), fixture.userID, workoutID); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	const requests = 12
	start := make(chan struct{})
	comments := make(chan string, requests)
	replayedResults := make(chan bool, requests)
	errorsFound := make(chan error, requests)
	var wait sync.WaitGroup
	for index := range requests {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			comment, replayed, err := fixture.repository.SaveSummaryComment(
				context.Background(),
				fixture.userID,
				workoutID,
				fmt.Sprintf("comment-%d", index),
			)
			if err != nil {
				errorsFound <- err
				return
			}
			comments <- comment
			replayedResults <- replayed
		}()
	}
	close(start)
	wait.Wait()
	close(comments)
	close(replayedResults)
	close(errorsFound)

	for err := range errorsFound {
		t.Fatalf("SaveSummaryComment() concurrent error = %v", err)
	}
	winningComment := ""
	for comment := range comments {
		if winningComment == "" {
			winningComment = comment
		}
		if comment != winningComment {
			t.Fatalf("SaveSummaryComment() comments differ: got %q, want %q", comment, winningComment)
		}
	}
	created := 0
	for replayed := range replayedResults {
		if !replayed {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("non-replayed results = %d, want 1", created)
	}
	if winningComment == "" {
		t.Fatal("SaveSummaryComment() returned no comment")
	}

	var stored string
	if err := fixture.db.QueryRow(`SELECT COALESCE(summary_comment,'') FROM workouts WHERE id=$1`, workoutID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != winningComment {
		t.Fatalf("stored summary comment = %q, want %q", stored, winningComment)
	}
}

func newIntegrationFixture(t *testing.T) integrationFixture {
	t.Helper()
	dsn := os.Getenv("FITLOG_TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = defaultIntegrationDSN
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		admin.Close()
		t.Skipf("integration PostgreSQL is unavailable: %v", err)
	}
	schema := fmt.Sprintf("fitlog_it_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+pq.QuoteIdentifier(schema)); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	admin.Close()

	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(20)
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	fixture := integrationFixture{db: db, repository: workoutpostgres.New(db), exerciseID: "integration_exercise_" + schema}
	if err := db.QueryRowContext(ctx, `INSERT INTO users (username,password_hash) VALUES ($1,'test') RETURNING id`, "integration_user_"+schema).Scan(&fixture.userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO exercises (id,name) VALUES ($1,'Integration Exercise')`, fixture.exerciseID); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f integrationFixture) createWorkout(t *testing.T) int {
	t.Helper()
	var workoutID int
	if err := f.db.QueryRow(`INSERT INTO workouts (user_id,notes) VALUES ($1,'Integration Workout') RETURNING id`, f.userID).Scan(&workoutID); err != nil {
		t.Fatal(err)
	}
	return workoutID
}
