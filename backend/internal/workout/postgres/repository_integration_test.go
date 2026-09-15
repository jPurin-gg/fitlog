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
	schema     string
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

func TestRecordSetConflictOnDifferentPayload(t *testing.T) {
	fixture := newIntegrationFixture(t)
	otherExerciseID := fixture.exerciseID + "_other"
	if _, err := fixture.db.Exec(`INSERT INTO exercises (id,name) VALUES ($1,'Other Integration Exercise')`, otherExerciseID); err != nil {
		t.Fatal(err)
	}
	recorded := workout.SetInput{ExerciseID: fixture.exerciseID, SetOrder: 1, Weight: 40, Reps: 8, Feeling: "普通"}

	tests := []struct {
		name  string
		input workout.SetInput
	}{
		{name: "weight", input: workout.SetInput{ExerciseID: fixture.exerciseID, SetOrder: 1, Weight: 45, Reps: 8, Feeling: "普通"}},
		{name: "set_order", input: workout.SetInput{ExerciseID: fixture.exerciseID, SetOrder: 2, Weight: 40, Reps: 8, Feeling: "普通"}},
		{name: "reps", input: workout.SetInput{ExerciseID: fixture.exerciseID, SetOrder: 1, Weight: 40, Reps: 9, Feeling: "普通"}},
		{name: "feeling", input: workout.SetInput{ExerciseID: fixture.exerciseID, SetOrder: 1, Weight: 40, Reps: 8, Feeling: "きつい"}},
		{name: "exercise", input: workout.SetInput{ExerciseID: otherExerciseID, SetOrder: 1, Weight: 40, Reps: 8, Feeling: "普通"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workoutID := fixture.createWorkout(t)
			created, replayed, err := fixture.repository.RecordSet(context.Background(), fixture.userID, workoutID, "conflict_key", recorded)
			if err != nil || replayed {
				t.Fatalf("first RecordSet() = (%#v, %v, %v), want created set", created, replayed, err)
			}

			conflicting, replayed, err := fixture.repository.RecordSet(context.Background(), fixture.userID, workoutID, "conflict_key", test.input)
			if !errors.Is(err, workout.ErrConflict) {
				t.Fatalf("conflicting RecordSet() = (%#v, %v, %v), want ErrConflict", conflicting, replayed, err)
			}
			if sets := fixture.storedSets(t, workoutID); len(sets) != 1 || sets[0] != created {
				t.Fatalf("stored sets after conflict = %#v, want only %#v", sets, created)
			}

			replayedSet, replayed, err := fixture.repository.RecordSet(context.Background(), fixture.userID, workoutID, "conflict_key", recorded)
			if err != nil || !replayed || replayedSet.ID != created.ID {
				t.Fatalf("replayed RecordSet() = (%#v, %v, %v), want replay of set %d", replayedSet, replayed, err, created.ID)
			}
		})
	}
}

func TestRecordSetConcurrentConflict(t *testing.T) {
	fixture := newIntegrationFixture(t)
	workoutID := fixture.createWorkout(t)
	payloads := []workout.SetInput{
		{ExerciseID: fixture.exerciseID, SetOrder: 1, Weight: 40, Reps: 8, Feeling: "普通"},
		{ExerciseID: fixture.exerciseID, SetOrder: 1, Weight: 45, Reps: 8, Feeling: "普通"},
	}

	const requests = 12
	start := make(chan struct{})
	outcomes := make(chan recordOutcome, requests)
	var wait sync.WaitGroup
	for index := range requests {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			input := payloads[index%len(payloads)]
			set, replayed, err := fixture.repository.RecordSet(context.Background(), fixture.userID, workoutID, "concurrent_conflict_key", input)
			outcomes <- recordOutcome{input: input, set: set, replayed: replayed, err: err}
		}()
	}
	close(start)
	wait.Wait()
	close(outcomes)

	sets := fixture.storedSets(t, workoutID)
	if len(sets) != 1 {
		t.Fatalf("stored sets = %#v, want exactly one", sets)
	}
	stored := sets[0]
	created := 0
	for outcome := range outcomes {
		if outcome.input.Weight != stored.Weight {
			if !errors.Is(outcome.err, workout.ErrConflict) {
				t.Fatalf("RecordSet(%#v) = (%#v, %v, %v), want ErrConflict against stored %#v", outcome.input, outcome.set, outcome.replayed, outcome.err, stored)
			}
			continue
		}
		if outcome.err != nil || outcome.set.ID != stored.ID {
			t.Fatalf("RecordSet(%#v) = (%#v, %v, %v), want stored set %d", outcome.input, outcome.set, outcome.replayed, outcome.err, stored.ID)
		}
		if !outcome.replayed {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("non-replayed results = %d, want 1", created)
	}
}

func TestRecordSetConflictOnUniqueViolation(t *testing.T) {
	fixture := newIntegrationFixture(t)
	pending := workout.SetInput{ExerciseID: fixture.exerciseID, SetOrder: 1, Weight: 40, Reps: 8, Feeling: "普通"}

	tests := []struct {
		name         string
		input        workout.SetInput
		wantReplayed bool
		wantErr      error
	}{
		{name: "same payload replays", input: pending, wantReplayed: true},
		{name: "different weight conflicts", input: workout.SetInput{ExerciseID: fixture.exerciseID, SetOrder: 1, Weight: 45, Reps: 8, Feeling: "普通"}, wantErr: workout.ErrConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workoutID := fixture.createWorkout(t)
			tx, err := fixture.db.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			// RecordSet holds FOR UPDATE on the workout row, and the FK trigger of a normal insert would block on it,
			// so the uncommitted row is inserted without triggers to keep the key invisible to findSetByKey until the INSERT.
			if _, err := tx.ExecContext(context.Background(), `SET LOCAL session_replication_role = replica`); err != nil {
				t.Fatal(err)
			}
			var pendingID int
			if err := tx.QueryRowContext(context.Background(), `
				INSERT INTO workout_sets (workout_id,exercise_id,weight,reps,set_order,feeling,idempotency_key)
				VALUES ($1,$2,$3,$4,$5,$6,'unique_violation_key') RETURNING id
			`, workoutID, pending.ExerciseID, pending.Weight, pending.Reps, pending.SetOrder, pending.Feeling).Scan(&pendingID); err != nil {
				t.Fatal(err)
			}

			outcomes := make(chan recordOutcome, 1)
			go func() {
				set, replayed, err := fixture.repository.RecordSet(context.Background(), fixture.userID, workoutID, "unique_violation_key", test.input)
				outcomes <- recordOutcome{input: test.input, set: set, replayed: replayed, err: err}
			}()
			deadline := time.Now().Add(10 * time.Second)
			for {
				select {
				case outcome := <-outcomes:
					t.Fatalf("RecordSet() = %#v before the pending insert committed", outcome)
				default:
				}
				var waiting int
				if err := fixture.db.QueryRow(`
					SELECT COUNT(*) FROM pg_stat_activity
					WHERE application_name=$1 AND wait_event_type='Lock' AND query LIKE '%INSERT INTO workout_sets%'
				`, fixture.schema).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting == 1 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("RecordSet() calls waiting on the unique index = %d, want 1", waiting)
				}
				time.Sleep(20 * time.Millisecond)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}

			outcome := <-outcomes
			if !errors.Is(outcome.err, test.wantErr) || outcome.replayed != test.wantReplayed {
				t.Fatalf("RecordSet() = (%#v, %v, %v), want (replayed=%v, err=%v)", outcome.set, outcome.replayed, outcome.err, test.wantReplayed, test.wantErr)
			}
			if test.wantErr == nil && outcome.set.ID != pendingID {
				t.Fatalf("RecordSet() set ID = %d, want %d", outcome.set.ID, pendingID)
			}
			if sets := fixture.storedSets(t, workoutID); len(sets) != 1 || sets[0].ID != pendingID || sets[0].Weight != pending.Weight {
				t.Fatalf("stored sets = %#v, want only set %d with weight %v", sets, pendingID, pending.Weight)
			}
		})
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

func TestSaveSummaryCommentRejectsActiveWorkout(t *testing.T) {
	fixture := newIntegrationFixture(t)
	workoutID := fixture.createWorkout(t)

	comment, replayed, err := fixture.repository.SaveSummaryComment(context.Background(), fixture.userID, workoutID, "進行中の総評")
	if !errors.Is(err, workout.ErrConflict) {
		t.Fatalf("active SaveSummaryComment() = (%q, %v, %v), want ErrConflict", comment, replayed, err)
	}
	var stored string
	if err := fixture.db.QueryRow(`SELECT COALESCE(summary_comment,'') FROM workouts WHERE id=$1`, workoutID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != "" {
		t.Fatalf("stored summary comment on active workout = %q, want empty", stored)
	}

	if _, err := fixture.repository.Finish(context.Background(), fixture.userID, workoutID); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	comment, replayed, err = fixture.repository.SaveSummaryComment(context.Background(), fixture.userID, workoutID, "完了後の総評")
	if err != nil || replayed || comment != "完了後の総評" {
		t.Fatalf("finished SaveSummaryComment() = (%q, %v, %v), want saved comment", comment, replayed, err)
	}
}

func TestSaveSummaryCommentKeepsFirstComment(t *testing.T) {
	fixture := newIntegrationFixture(t)
	workoutID := fixture.createWorkout(t)
	if _, err := fixture.repository.Finish(context.Background(), fixture.userID, workoutID); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	first, replayed, err := fixture.repository.SaveSummaryComment(context.Background(), fixture.userID, workoutID, "最初の総評")
	if err != nil || replayed || first != "最初の総評" {
		t.Fatalf("first SaveSummaryComment() = (%q, %v, %v), want saved comment", first, replayed, err)
	}
	second, replayed, err := fixture.repository.SaveSummaryComment(context.Background(), fixture.userID, workoutID, "二番目の総評")
	if err != nil || !replayed || second != "最初の総評" {
		t.Fatalf("second SaveSummaryComment() = (%q, %v, %v), want replay of %q", second, replayed, err, "最初の総評")
	}

	var stored string
	if err := fixture.db.QueryRow(`SELECT COALESCE(summary_comment,'') FROM workouts WHERE id=$1`, workoutID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != "最初の総評" {
		t.Fatalf("stored summary comment = %q, want %q", stored, "最初の総評")
	}
}

func TestWorkoutRepositoryDoesNotExposeAnotherUsersWorkout(t *testing.T) {
	fixture := newIntegrationFixture(t)
	workoutID := fixture.createWorkout(t)
	input := workout.SetInput{ExerciseID: fixture.exerciseID, SetOrder: 1, Weight: 40, Reps: 8}
	created, _, err := fixture.repository.RecordSet(context.Background(), fixture.userID, workoutID, "owner_set_key", input)
	if err != nil {
		t.Fatalf("owner RecordSet() error = %v", err)
	}

	var otherUserID int
	if err := fixture.db.QueryRow(`INSERT INTO users (username,password_hash) VALUES ('other_user','test') RETURNING id`).Scan(&otherUserID); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.repository.Detail(context.Background(), otherUserID, workoutID); !errors.Is(err, workout.ErrNotFound) {
		t.Fatalf("other user Detail() error = %v, want ErrNotFound", err)
	}
	if _, _, err := fixture.repository.RecordSet(context.Background(), otherUserID, workoutID, "other_user_key", input); !errors.Is(err, workout.ErrNotFound) {
		t.Fatalf("other user RecordSet() error = %v, want ErrNotFound", err)
	}
	if _, err := fixture.repository.RecommendationContext(context.Background(), otherUserID, workoutID, created.ID); !errors.Is(err, workout.ErrNotFound) {
		t.Fatalf("other user RecommendationContext() error = %v, want ErrNotFound", err)
	}
	if _, err := fixture.repository.Finish(context.Background(), otherUserID, workoutID); !errors.Is(err, workout.ErrNotFound) {
		t.Fatalf("other user Finish() error = %v, want ErrNotFound", err)
	}
	if _, _, err := fixture.repository.SaveSummaryComment(context.Background(), otherUserID, workoutID, "見えてはいけない総評"); !errors.Is(err, workout.ErrNotFound) {
		t.Fatalf("other user SaveSummaryComment() error = %v, want ErrNotFound", err)
	}

	detail, err := fixture.repository.Detail(context.Background(), fixture.userID, workoutID)
	if err != nil || detail.Summary.TotalSets != 1 {
		t.Fatalf("owner Detail() = (%+v, %v), want one unchanged set", detail, err)
	}
}

func newIntegrationFixture(t *testing.T) integrationFixture {
	t.Helper()
	dsn, explicit := os.Getenv("FITLOG_TEST_DATABASE_DSN"), true
	if dsn == "" {
		dsn, explicit = defaultIntegrationDSN, false
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		admin.Close()
		if explicit {
			t.Fatalf("integration PostgreSQL from FITLOG_TEST_DATABASE_DSN is unavailable: %v", err)
		}
		t.Skipf("integration PostgreSQL is unavailable: %v", err)
	}
	schema := fmt.Sprintf("fitlog_it_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+pq.QuoteIdentifier(schema)); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	admin.Close()
	t.Cleanup(func() {
		admin, err := sql.Open("postgres", dsn)
		if err != nil {
			t.Errorf("open admin connection to drop schema %s: %v", schema, err)
			return
		}
		defer admin.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(ctx, `DROP SCHEMA `+pq.QuoteIdentifier(schema)+` CASCADE`); err != nil {
			t.Errorf("drop schema %s: %v", schema, err)
		}
	})

	db, err := sql.Open("postgres", dsn+" search_path="+schema+" application_name="+schema)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(20)
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	fixture := integrationFixture{db: db, repository: workoutpostgres.New(db), schema: schema, exerciseID: "integration_exercise_" + schema}
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

func (f integrationFixture) storedSets(t *testing.T, workoutID int) []workout.Set {
	t.Helper()
	rows, err := f.db.Query(`
		SELECT id,workout_id,exercise_id,set_order,weight,reps,COALESCE(feeling,''),is_pr,created_at
		FROM workout_sets WHERE workout_id=$1 ORDER BY id
	`, workoutID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	sets := []workout.Set{}
	for rows.Next() {
		var set workout.Set
		var createdAt time.Time
		if err := rows.Scan(&set.ID, &set.WorkoutID, &set.ExerciseID, &set.SetOrder, &set.Weight, &set.Reps, &set.Feeling, &set.IsPR, &createdAt); err != nil {
			t.Fatal(err)
		}
		set.CreatedAt = createdAt.Format(time.RFC3339)
		sets = append(sets, set)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return sets
}

type recordOutcome struct {
	input    workout.SetInput
	set      workout.Set
	replayed bool
	err      error
}
