//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

// recordedWorkout は、その場で記録して終了したワークアウトを表す検証データ。
type recordedWorkout struct {
	id        int
	date      time.Time
	startedAt time.Time
	endedAt   time.Time
	setIDs    []int
}

type storedCalendarSet struct {
	id             int
	weight         float64
	reps           int
	feeling        string
	isPR           bool
	createdAt      time.Time
	idempotencyKey string
}

func TestSaveCalendarWorkoutKeepsRecordedTimesAndSummaryComment(t *testing.T) {
	fixture := newIntegrationFixture(t)
	ctx := context.Background()
	recorded := fixture.createRecordedWorkout(t, time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC))
	before := fixture.storedCalendarSets(t, recorded.id)

	current, err := fixture.repository.CalendarWorkout(ctx, fixture.userID, recorded.date)
	if err != nil || len(current.Sets) != 2 || current.Sets[0].ID != recorded.setIDs[0] || current.Sets[1].ID != recorded.setIDs[1] {
		t.Fatalf("CalendarWorkout() = (%+v, %v), want the two recorded sets", current, err)
	}
	// 2セット目の回数と感想だけを直して保存する。
	input := workout.CalendarWorkoutInput{Title: "胸の日（修正）", Sets: current.Sets}
	input.Sets[1].Reps = 9
	input.Sets[1].Feeling = "ぎりぎり"
	saved, err := fixture.repository.SaveCalendarWorkout(ctx, fixture.userID, recorded.date, input)
	if err != nil || saved.WorkoutID != recorded.id || saved.Title != "胸の日（修正）" || len(saved.Sets) != 2 {
		t.Fatalf("SaveCalendarWorkout() = (%+v, %v), want the same workout with two sets", saved, err)
	}

	fixture.assertWorkoutTimes(t, recorded)
	var comment string
	if err := fixture.db.QueryRow(`SELECT COALESCE(summary_comment,'') FROM workouts WHERE id=$1`, recorded.id).Scan(&comment); err != nil {
		t.Fatal(err)
	}
	if comment != "保存済みの総評" {
		t.Fatalf("summary comment after edit = %q, want the stored comment", comment)
	}

	after := fixture.storedCalendarSets(t, recorded.id)
	if len(after) != 2 || after[0] != before[0] {
		t.Fatalf("sets after edit = %+v, want the untouched first set %+v", after, before[0])
	}
	want := before[1]
	want.reps, want.feeling = 9, "ぎりぎり"
	if after[1] != want {
		t.Fatalf("edited set = %+v, want %+v (same ID, time, idempotency key and PR flag)", after[1], want)
	}

	// 所要時間の集計は、開始・終了時刻が変わらないので47分のまま。
	detail, err := fixture.repository.Detail(ctx, fixture.userID, recorded.id)
	if err != nil || detail.Summary.DurationMin != 47 || detail.Summary.AIComment != "保存済みの総評" || detail.Summary.TotalReps != 19 {
		t.Fatalf("Detail() after edit = (%+v, %v)", detail, err)
	}
}

func TestSaveCalendarWorkoutAddsAndRemovesSetsWithoutMovingTimes(t *testing.T) {
	fixture := newIntegrationFixture(t)
	ctx := context.Background()
	recorded := fixture.createRecordedWorkout(t, time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC))
	before := fixture.storedCalendarSets(t, recorded.id)

	// 1セット目（60kg）を消し、2セット目を残して、40kgのセットを追加する。
	input := workout.CalendarWorkoutInput{Title: "胸の日", Sets: []workout.CalendarSet{
		{ID: recorded.setIDs[1], ExerciseID: fixture.exerciseID, Weight: 50, Reps: 8, SetOrder: 1, Feeling: "重い"},
		{ExerciseID: fixture.exerciseID, Weight: 40, Reps: 12, SetOrder: 2, Feeling: "追加"},
	}}
	saved, err := fixture.repository.SaveCalendarWorkout(ctx, fixture.userID, recorded.date, input)
	if err != nil || len(saved.Sets) != 2 || saved.Sets[0].ID != recorded.setIDs[1] || saved.Sets[1].Weight != 40 {
		t.Fatalf("SaveCalendarWorkout() = (%+v, %v), want the kept set followed by the added set", saved, err)
	}

	fixture.assertWorkoutTimes(t, recorded)
	after := fixture.storedCalendarSets(t, recorded.id)
	if len(after) != 2 || after[0] != before[1] {
		t.Fatalf("sets after edit = %+v, want the kept set %+v first", after, before[1])
	}
	// 追加したセットは直前のセットと同じ時刻になり、ワークアウトの時間内に収まる。
	if added := after[1]; added.id == recorded.setIDs[0] || !added.createdAt.Equal(before[1].createdAt) || added.idempotencyKey != "" || added.isPR {
		t.Fatalf("added set = %+v", added)
	}
	// 消したセットが最大重量だったので、残りの記録から再計算する。
	fixture.assertStats(t, fixture.exerciseID, 40, 50)
}

func TestSaveCalendarWorkoutWithoutSetIDsStillKeepsTimes(t *testing.T) {
	fixture := newIntegrationFixture(t)
	recorded := fixture.createRecordedWorkout(t, time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC))

	// IDを送らない古い画面からの保存でも、開始・終了時刻は書き換えない。
	input := workout.CalendarWorkoutInput{Title: "胸の日", Sets: []workout.CalendarSet{
		{ExerciseID: fixture.exerciseID, Weight: 55, Reps: 8, SetOrder: 1},
	}}
	if _, err := fixture.repository.SaveCalendarWorkout(context.Background(), fixture.userID, recorded.date, input); err != nil {
		t.Fatalf("SaveCalendarWorkout() error = %v", err)
	}

	fixture.assertWorkoutTimes(t, recorded)
	after := fixture.storedCalendarSets(t, recorded.id)
	if len(after) != 1 || after[0].weight != 55 || !after[0].createdAt.Equal(recorded.startedAt) {
		t.Fatalf("sets after edit = %+v, want one replaced set at the workout start", after)
	}
}

func TestSaveCalendarWorkoutCreatesPlaceholderTimesForNewDate(t *testing.T) {
	fixture := newIntegrationFixture(t)
	date := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	input := workout.CalendarWorkoutInput{Title: "あとから記録", Sets: []workout.CalendarSet{
		{ExerciseID: fixture.exerciseID, Weight: 30, Reps: 10, SetOrder: 1},
		{ExerciseID: fixture.exerciseID, Weight: 35, Reps: 8, SetOrder: 2},
	}}
	saved, err := fixture.repository.SaveCalendarWorkout(context.Background(), fixture.userID, date, input)
	if err != nil || saved.WorkoutID <= 0 || len(saved.Sets) != 2 {
		t.Fatalf("SaveCalendarWorkout() = (%+v, %v), want a new workout", saved, err)
	}

	// 記録のなかった日は実際の時刻が分からないため、正午開始・15分＋3分×セット数を仮置きする。
	noon := date.Add(12 * time.Hour)
	fixture.assertWorkoutTimes(t, recordedWorkout{id: saved.WorkoutID, startedAt: noon, endedAt: noon.Add(21 * time.Minute)})
	sets := fixture.storedCalendarSets(t, saved.WorkoutID)
	if len(sets) != 2 || !sets[0].isPR || !sets[1].isPR || !sets[0].createdAt.Equal(noon) || !sets[1].createdAt.Equal(noon.Add(time.Minute)) {
		t.Fatalf("stored sets = %+v", sets)
	}
	fixture.assertStats(t, fixture.exerciseID, 35, 35)
}

// createRecordedWorkout は、19:30開始・47分間で2セットを記録し、AI総評まで保存済みの状態を作る。
func (f integrationFixture) createRecordedWorkout(t *testing.T, date time.Time) recordedWorkout {
	t.Helper()
	recorded := recordedWorkout{date: date, startedAt: date.Add(19*time.Hour + 30*time.Minute)}
	recorded.endedAt = recorded.startedAt.Add(47 * time.Minute)
	if err := f.db.QueryRow(`
		INSERT INTO workouts (user_id,started_at,ended_at,notes,summary_comment)
		VALUES ($1,$2,$3,'胸の日','保存済みの総評') RETURNING id
	`, f.userID, recorded.startedAt, recorded.endedAt).Scan(&recorded.id); err != nil {
		t.Fatal(err)
	}
	for index, set := range []struct {
		weight  float64
		reps    int
		feeling string
		isPR    bool
		key     string
		offset  time.Duration
	}{
		{weight: 60, reps: 10, feeling: "普通", isPR: true, key: "recorded_set_001", offset: 5 * time.Minute},
		{weight: 50, reps: 8, feeling: "重い", isPR: false, key: "recorded_set_002", offset: 47 * time.Minute},
	} {
		var id int
		if err := f.db.QueryRow(`
			INSERT INTO workout_sets (workout_id,exercise_id,weight,reps,set_order,feeling,is_pr,idempotency_key,created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id
		`, recorded.id, f.exerciseID, set.weight, set.reps, index+1, set.feeling, set.isPR, set.key, recorded.startedAt.Add(set.offset)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		recorded.setIDs = append(recorded.setIDs, id)
	}
	return recorded
}

func (f integrationFixture) storedCalendarSets(t *testing.T, workoutID int) []storedCalendarSet {
	t.Helper()
	rows, err := f.db.Query(`
		SELECT id,weight,reps,COALESCE(feeling,''),is_pr,created_at,COALESCE(idempotency_key,'')
		FROM workout_sets WHERE workout_id=$1 ORDER BY created_at,id
	`, workoutID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	sets := []storedCalendarSet{}
	for rows.Next() {
		var set storedCalendarSet
		if err := rows.Scan(&set.id, &set.weight, &set.reps, &set.feeling, &set.isPR, &set.createdAt, &set.idempotencyKey); err != nil {
			t.Fatal(err)
		}
		// 同じ時刻を値として比較できるよう、タイムゾーン表現をそろえる。
		set.createdAt = set.createdAt.UTC()
		sets = append(sets, set)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return sets
}

func (f integrationFixture) assertWorkoutTimes(t *testing.T, want recordedWorkout) {
	t.Helper()
	var startedAt, endedAt time.Time
	if err := f.db.QueryRow(`SELECT started_at,ended_at FROM workouts WHERE id=$1`, want.id).Scan(&startedAt, &endedAt); err != nil {
		t.Fatal(err)
	}
	if !startedAt.Equal(want.startedAt) || !endedAt.Equal(want.endedAt) {
		t.Fatalf("workout times = %s - %s, want %s - %s", startedAt, endedAt, want.startedAt, want.endedAt)
	}
}

func (f integrationFixture) assertStats(t *testing.T, exerciseID string, wantLatest, wantMax float64) {
	t.Helper()
	var latestWeight, maxWeight float64
	if err := f.db.QueryRow(`
		SELECT COALESCE(weight,-1),COALESCE(max_weight,-1) FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2
	`, f.userID, exerciseID).Scan(&latestWeight, &maxWeight); err != nil {
		t.Fatal(err)
	}
	if latestWeight != wantLatest || maxWeight != wantMax {
		t.Fatalf("stats: weight=%v max_weight=%v, want %v and %v", latestWeight, maxWeight, wantLatest, wantMax)
	}
}
