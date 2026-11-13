//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"hash/crc32"
	"testing"
	"time"

	"github.com/lib/pq"

	"github.com/jPurin-gg/myfitlog-backend/internal/database"
	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

// Pause the first writer after its aggregate SELECT, but before the stats row
// is locked. Without the shared history lock, the other date can commit here
// and the paused writer can subsequently overwrite its stats with stale values.
func TestExerciseStatsSerializeAcrossDates(t *testing.T) {
	for _, useRecordSet := range []bool{false, true} {
		for _, newerFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("RecordSet=%v/newerFirst=%v", useRecordSet, newerFirst), func(t *testing.T) {
				f := newIntegrationFixture(t)
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				oldDate := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
				newDate := oldDate.AddDate(0, 0, 1)
				old := saveStatsCalendar(t, f, oldDate, statsSet(f.exerciseID, 50))
				var newerWorkoutID int
				if useRecordSet {
					if err := f.db.QueryRow(`INSERT INTO workouts (user_id,started_at,notes) VALUES ($1,$2,'Active') RETURNING id`, f.userID, newDate.Add(12*time.Hour)).Scan(&newerWorkoutID); err != nil {
						t.Fatal(err)
					}
					if _, _, err := f.repository.RecordSet(ctx, f.userID, newerWorkoutID, "stats_initial_record", workout.SetInput{ExerciseID: f.exerciseID, Weight: 100, Reps: 8, SetOrder: 1}); err != nil {
						t.Fatal(err)
					}
				} else {
					newerWorkoutID = saveStatsCalendar(t, f, newDate, statsSet(f.exerciseID, 100)).WorkoutID
				}

				older := func() error {
					_, err := f.repository.SaveCalendarWorkout(ctx, f.userID, oldDate, workout.CalendarWorkoutInput{Title: "Older edited", Sets: []workout.CalendarSet{statsSet(f.exerciseID, 60)}})
					return err
				}
				newer := func() error {
					if useRecordSet {
						_, _, err := f.repository.RecordSet(ctx, f.userID, newerWorkoutID, "stats_concurrent_record", workout.SetInput{ExerciseID: f.exerciseID, Weight: 110, Reps: 8, SetOrder: 2})
						return err
					}
					_, err := f.repository.SaveCalendarWorkout(ctx, f.userID, newDate, workout.CalendarWorkoutInput{Title: "Newer edited", Sets: []workout.CalendarSet{statsSet(f.exerciseID, 110)}})
					return err
				}
				first, second, pauseWeight := older, newer, float64(100)
				if newerFirst {
					first, second, pauseWeight = newer, older, 110
				}
				gate := pauseStatsWrite(t, f, ctx, pauseWeight)
				firstDone := startStatsOperation(first)
				waitForStatsLock(t, f, ctx, "%INSERT INTO user_exercise_stats%", 1, firstDone)
				secondDone := startStatsOperation(second)
				waitForStatsLock(t, f, ctx, "%pg_advisory_xact_lock%", 1, secondDone)
				if err := gate.Commit(); err != nil {
					t.Fatal(err)
				}
				finishStatsOperation(t, ctx, firstDone)
				finishStatsOperation(t, ctx, secondDone)
				assertStoredWeights(t, f, old.WorkoutID, 60)
				if useRecordSet {
					assertStoredWeights(t, f, newerWorkoutID, 100, 110)
				} else {
					assertStoredWeights(t, f, newerWorkoutID, 110)
				}
				assertStatsMatchHistory(t, f, f.exerciseID, 110, 110)
			})
		}
	}
}

func TestExerciseStatsSerializeBeforeFirstStatsRow(t *testing.T) {
	for _, useRecordSet := range []bool{false, true} {
		t.Run(fmt.Sprintf("RecordSet=%v", useRecordSet), func(t *testing.T) {
			f := newIntegrationFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			oldDate := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
			newDate := oldDate.AddDate(0, 0, 1)
			var newerWorkoutID int
			if useRecordSet {
				if err := f.db.QueryRow(`INSERT INTO workouts (user_id,started_at,notes) VALUES ($1,$2,'Active') RETURNING id`, f.userID, newDate.Add(12*time.Hour)).Scan(&newerWorkoutID); err != nil {
					t.Fatal(err)
				}
			}
			// There is no stats row to lock yet. Pause the first INSERT before
			// PostgreSQL's unique index can serialize the two stats writes.
			gate := pauseStatsWrite(t, f, ctx, 50)
			firstDone := startStatsOperation(func() error {
				_, err := f.repository.SaveCalendarWorkout(ctx, f.userID, oldDate, workout.CalendarWorkoutInput{Title: "First history", Sets: []workout.CalendarSet{statsSet(f.exerciseID, 50)}})
				return err
			})
			waitForStatsLock(t, f, ctx, "%INSERT INTO user_exercise_stats%", 1, firstDone)
			secondDone := startStatsOperation(func() error {
				if useRecordSet {
					_, _, err := f.repository.RecordSet(ctx, f.userID, newerWorkoutID, "first_stats_record", workout.SetInput{ExerciseID: f.exerciseID, Weight: 110, Reps: 8, SetOrder: 1})
					return err
				}
				_, err := f.repository.SaveCalendarWorkout(ctx, f.userID, newDate, workout.CalendarWorkoutInput{Title: "Second history", Sets: []workout.CalendarSet{statsSet(f.exerciseID, 110)}})
				return err
			})
			waitForStatsLock(t, f, ctx, "%pg_advisory_xact_lock%", 1, secondDone)
			if err := gate.Commit(); err != nil {
				t.Fatal(err)
			}
			finishStatsOperation(t, ctx, firstDone)
			finishStatsOperation(t, ctx, secondDone)
			for _, date := range []time.Time{oldDate, newDate} {
				record, err := f.repository.CalendarWorkout(ctx, f.userID, date)
				if err != nil {
					t.Fatal(err)
				}
				want := float64(50)
				if date.Equal(newDate) {
					want = 110
				}
				assertStoredWeights(t, f, record.WorkoutID, want)
			}
			assertStatsMatchHistory(t, f, f.exerciseID, 110, 110)
		})
	}
}

func TestCalendarStatsLocksExercisesInOneOrder(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	other := f.exerciseID + "_other"
	if _, err := f.db.Exec(`INSERT INTO exercises (id,name) VALUES ($1,'Other exercise')`, other); err != nil {
		t.Fatal(err)
	}
	low, high := f.exerciseID, other
	if int32(crc32.ChecksumIEEE([]byte(low))|(1<<31)) > int32(crc32.ChecksumIEEE([]byte(high))|(1<<31)) {
		low, high = high, low
	}
	oldDate := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	newDate := oldDate.AddDate(0, 0, 1)
	old := saveStatsCalendar(t, f, oldDate, statsSet(low, 150), statsSet(high, 60))
	newer := saveStatsCalendar(t, f, newDate, statsSet(high, 120), statsSet(low, 100))
	gate := lockStatsHistory(t, f, ctx, low)
	firstDone := startStatsOperation(func() error {
		_, err := f.repository.SaveCalendarWorkout(ctx, f.userID, oldDate, workout.CalendarWorkoutInput{Title: "Old", Sets: []workout.CalendarSet{statsSet(low, 120), statsSet(high, 70)}})
		return err
	})
	waitForStatsLock(t, f, ctx, "%pg_advisory_xact_lock%", 1, firstDone)
	secondDone := startStatsOperation(func() error {
		_, err := f.repository.SaveCalendarWorkout(ctx, f.userID, newDate, workout.CalendarWorkoutInput{Title: "New", Sets: []workout.CalendarSet{statsSet(high, 130), statsSet(low, 110)}})
		return err
	})
	// If input order were used, the second request would hold high while
	// waiting for low; releasing the gate would then deadlock the requests.
	waitForStatsLock(t, f, ctx, "%pg_advisory_xact_lock%", 2, secondDone)
	if err := gate.Commit(); err != nil {
		t.Fatal(err)
	}
	finishStatsOperation(t, ctx, firstDone)
	finishStatsOperation(t, ctx, secondDone)
	assertStoredWeights(t, f, old.WorkoutID, 120, 70)
	assertStoredWeights(t, f, newer.WorkoutID, 130, 110)
	assertStatsMatchHistory(t, f, low, 110, 120)
	assertStatsMatchHistory(t, f, high, 130, 130)
}

func TestCalendarReplacementLocksRemovedExerciseAndRollsBackCancellation(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	other := f.exerciseID + "_replacement"
	if _, err := f.db.Exec(`INSERT INTO exercises (id,name) VALUES ($1,'Replacement exercise')`, other); err != nil {
		t.Fatal(err)
	}
	date := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	original := saveStatsCalendar(t, f, date, statsSet(f.exerciseID, 50))
	gate := lockStatsHistory(t, f, ctx, f.exerciseID)
	writeCtx, cancelWrite := context.WithCancel(ctx)
	defer cancelWrite()
	input := workout.CalendarWorkoutInput{Title: "Replaced", Sets: []workout.CalendarSet{statsSet(other, 60)}}
	done := startStatsOperation(func() error {
		_, err := f.repository.SaveCalendarWorkout(writeCtx, f.userID, date, input)
		return err
	})
	waitForStatsLock(t, f, ctx, "%pg_advisory_xact_lock%", 1, done)
	cancelWrite()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled SaveCalendarWorkout() succeeded")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	assertStoredWeights(t, f, original.WorkoutID, 50)
	assertStatsMatchHistory(t, f, f.exerciseID, 50, 50)
	var replacementStats int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2`, f.userID, other).Scan(&replacementStats); err != nil {
		t.Fatal(err)
	}
	if replacementStats != 0 {
		t.Fatal("canceled replacement left partial stats")
	}
	if err := gate.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.SaveCalendarWorkout(ctx, f.userID, date, input); err != nil {
		t.Fatal(err)
	}
	assertStoredWeights(t, f, original.WorkoutID, 60)
	assertStatsMatchHistory(t, f, other, 60, 60)
	var weight, maxWeight sql.NullFloat64
	if err := f.db.QueryRow(`SELECT weight,max_weight FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2`, f.userID, f.exerciseID).Scan(&weight, &maxWeight); err != nil {
		t.Fatal(err)
	}
	if weight.Valid || maxWeight.Valid {
		t.Fatalf("removed exercise stats = (%v,%v), want NULL", weight, maxWeight)
	}
}

func statsSet(exerciseID string, weight float64) workout.CalendarSet {
	return workout.CalendarSet{ExerciseID: exerciseID, Weight: weight, Reps: 8, SetOrder: 1}
}

func saveStatsCalendar(t *testing.T, f integrationFixture, date time.Time, sets ...workout.CalendarSet) workout.CalendarWorkout {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := f.repository.SaveCalendarWorkout(ctx, f.userID, date, workout.CalendarWorkoutInput{Title: "Stats test", Sets: sets})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func lockStatsHistory(t *testing.T, f integrationFixture, ctx context.Context, ids ...string) *sql.Tx {
	t.Helper()
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback() })
	if err := database.LockExerciseHistory(ctx, tx, f.userID, ids); err != nil {
		t.Fatal(err)
	}
	return tx
}

func pauseStatsWrite(t *testing.T, f integrationFixture, ctx context.Context, weight float64) *sql.Tx {
	t.Helper()
	key := time.Now().UnixNano()
	if _, err := f.db.ExecContext(ctx, fmt.Sprintf(`
		CREATE FUNCTION pause_stats_write() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.user_id=%d AND NEW.exercise_id=%s AND NEW.weight=%g THEN
				PERFORM pg_advisory_xact_lock(%d::bigint);
			END IF;
			RETURN NEW;
		END $$;
		CREATE TRIGGER pause_stats_write BEFORE INSERT ON user_exercise_stats
		FOR EACH ROW EXECUTE FUNCTION pause_stats_write();
	`, f.userID, pq.QuoteLiteral(f.exerciseID), weight, key)); err != nil {
		t.Fatal(err)
	}
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback() })
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, key); err != nil {
		t.Fatal(err)
	}
	return tx
}

func startStatsOperation(operation func() error) <-chan error {
	done := make(chan error, 1)
	go func() { done <- operation() }()
	return done
}

func finishStatsOperation(t *testing.T, ctx context.Context, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func waitForStatsLock(t *testing.T, f integrationFixture, ctx context.Context, query string, want int, done <-chan error) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting int
		if err := f.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock' AND query LIKE $2`, f.schema, query).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting == want {
			return
		}
		select {
		case err := <-done:
			t.Fatalf("operation finished before %d lock waiters appeared: %v", want, err)
		case <-ctx.Done():
			t.Fatalf("lock waiters = %d, want %d: %v", waiting, want, ctx.Err())
		case <-ticker.C:
		}
	}
}

func assertStoredWeights(t *testing.T, f integrationFixture, workoutID int, want ...float64) {
	t.Helper()
	sets := f.storedSets(t, workoutID)
	if len(sets) != len(want) {
		t.Fatalf("workout %d sets = %#v, want weights %v", workoutID, sets, want)
	}
	for i := range want {
		if sets[i].Weight != want[i] {
			t.Fatalf("workout %d set %d weight = %v, want %v", workoutID, i, sets[i].Weight, want[i])
		}
	}
}

func assertStatsMatchHistory(t *testing.T, f integrationFixture, exerciseID string, wantLatest, wantMax float64) {
	t.Helper()
	var latest, maxWeight, storedLatest, storedMax float64
	if err := f.db.QueryRow(`
		SELECT
			(SELECT ws.weight FROM workout_sets ws JOIN workouts w ON w.id=ws.workout_id
			 WHERE w.user_id=$1 AND ws.exercise_id=$2 ORDER BY w.started_at DESC,ws.created_at DESC,ws.id DESC LIMIT 1),
			(SELECT MAX(ws.weight) FROM workout_sets ws JOIN workouts w ON w.id=ws.workout_id WHERE w.user_id=$1 AND ws.exercise_id=$2),
			weight,max_weight
		FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2
	`, f.userID, exerciseID).Scan(&latest, &maxWeight, &storedLatest, &storedMax); err != nil {
		t.Fatal(err)
	}
	if latest != wantLatest || maxWeight != wantMax || storedLatest != latest || storedMax != maxWeight {
		t.Fatalf("exercise %q: history=(%v,%v), stats=(%v,%v), want=(%v,%v)", exerciseID, latest, maxWeight, storedLatest, storedMax, wantLatest, wantMax)
	}
}
