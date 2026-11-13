package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestExerciseLockKeysCanonicalOrder(t *testing.T) {
	// All exercise keys must stay negative, separate from YYYYMMDD date keys.
	want := []int32{-1792090616, -764940413, -531390641}
	for _, ids := range [][]string{
		{"bench_press", "deadlift", "squat"},
		{"squat", "deadlift", "bench_press"},
		{"bench_press", "squat", "bench_press", "deadlift", "squat"},
	} {
		if got := exerciseLockKeys(ids); !slices.Equal(got, want) {
			t.Errorf("exerciseLockKeys(%v) = %v, want %v", ids, got, want)
		}
	}
	if got := exerciseLockKeys(nil); len(got) != 0 {
		t.Errorf("exerciseLockKeys(nil) = %v, want no locks", got)
	}
}

func TestExerciseLockKeysHashCollision(t *testing.T) {
	// These distinct IDs have the same CRC32. Both paths must acquire the same
	// sorted lock set, even though the IDs have different lexical positions.
	const first, second = "exercise_eis9kjf22l5f", "exercise_hm8pwzdjtmlh"
	want := []int32{-1345510237, -764940413, -531390641}
	for _, ids := range [][]string{
		{"squat", first, "bench_press"},
		{"bench_press", second, "squat"},
		{second, "squat", first, "bench_press", second},
	} {
		if got := exerciseLockKeys(ids); !slices.Equal(got, want) {
			t.Errorf("exerciseLockKeys(%v) = %v, want %v", ids, got, want)
		}
	}
}

func TestLockExerciseHistoryAcquisition(t *testing.T) {
	lockErr := errors.New("lock acquisition failed")
	for _, test := range []struct {
		name    string
		failAt  int
		want    []int32
		wantErr error
	}{
		{name: "all locks in canonical order", want: []int32{-1792090616, -764940413, -531390641}},
		{name: "stop at first failed lock", failAt: 2, want: []int32{-1792090616, -764940413}, wantErr: lockErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			conn := &exerciseLockConn{t: t, failAt: test.failAt, lockErr: lockErr}
			db := sql.OpenDB(exerciseLockConnector{conn})
			defer db.Close()
			tx, err := db.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			err = LockExerciseHistory(context.Background(), tx, 42, []string{"bench_press", "squat", "deadlift", "bench_press"})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("LockExerciseHistory() error = %v, want %v", err, test.wantErr)
			}
			if !slices.Equal(conn.keys, test.want) {
				t.Errorf("acquired lock keys = %v, want %v", conn.keys, test.want)
			}
		})
	}
}

// This driver records SQL acquisition attempts without a PostgreSQL connection.
type exerciseLockConnector struct{ conn *exerciseLockConn }

func (c exerciseLockConnector) Connect(context.Context) (driver.Conn, error) { return c.conn, nil }
func (c exerciseLockConnector) Driver() driver.Driver                        { return c }
func (c exerciseLockConnector) Open(string) (driver.Conn, error)             { return c.conn, nil }

type exerciseLockConn struct {
	t       *testing.T
	keys    []int32
	failAt  int
	lockErr error
}

func (*exerciseLockConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected Prepare")
}
func (*exerciseLockConn) Close() error              { return nil }
func (*exerciseLockConn) Begin() (driver.Tx, error) { return exerciseLockTx{}, nil }
func (c *exerciseLockConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if !strings.Contains(query, "pg_advisory_xact_lock") || len(args) != 2 || args[0].Value != int64(42) {
		c.t.Fatalf("unexpected lock query %q with arguments %v", query, args)
	}
	c.keys = append(c.keys, int32(args[1].Value.(int64)))
	if len(c.keys) == c.failAt {
		return nil, c.lockErr
	}
	return driver.RowsAffected(1), nil
}

type exerciseLockTx struct{}

func (exerciseLockTx) Commit() error   { return nil }
func (exerciseLockTx) Rollback() error { return nil }
