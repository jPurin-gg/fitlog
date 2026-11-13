package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

func TestUpdateSetRecalculatesInsideTransaction(t *testing.T) {
	connection := &correctionConnection{setFound: true}
	weight := 80.0
	set, err := newCorrectionRepository(t, connection).UpdateSet(context.Background(), 1, 42, 7, workout.SetPatch{Weight: &weight})
	if err != nil || set.ID != 7 || set.Weight != 80 {
		t.Fatalf("UpdateSet() = %+v, %v", set, err)
	}
	want := []string{"lock", "update", "pr", "latest", "stats", "select"}
	if !reflect.DeepEqual(connection.steps, want) || !connection.committed {
		t.Fatalf("steps = %v, committed = %v; want %v committed", connection.steps, connection.committed, want)
	}
	if connection.updateArgs[0] != 80.0 || connection.updateArgs[1] != nil || connection.updateArgs[2] != nil {
		t.Fatalf("update args = %#v, want weight only", connection.updateArgs)
	}
}

func TestDeleteSetRecalculatesInsideTransaction(t *testing.T) {
	connection := &correctionConnection{setFound: true}
	if err := newCorrectionRepository(t, connection).DeleteSet(context.Background(), 1, 42, 7); err != nil {
		t.Fatalf("DeleteSet() error = %v", err)
	}
	want := []string{"lock", "delete", "pr", "latest", "stats"}
	if !reflect.DeepEqual(connection.steps, want) || !connection.committed {
		t.Fatalf("steps = %v, committed = %v; want %v committed", connection.steps, connection.committed, want)
	}
}

func TestSetCorrectionRejectsWithoutWriting(t *testing.T) {
	tests := []struct {
		name       string
		connection *correctionConnection
		want       error
		wantSteps  []string
	}{
		{name: "missing_workout", connection: &correctionConnection{workoutMissing: true}, want: workout.ErrNotFound, wantSteps: []string{"lock"}},
		{name: "completed_workout", connection: &correctionConnection{workoutEnded: true, setFound: true}, want: workout.ErrConflict, wantSteps: []string{"lock"}},
		{name: "missing_set", connection: &correctionConnection{}, want: workout.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reps := 8
			_, err := newCorrectionRepository(t, tt.connection).UpdateSet(context.Background(), 1, 42, 7, workout.SetPatch{Reps: &reps})
			if !errors.Is(err, tt.want) || tt.connection.committed || !tt.connection.rolledBack {
				t.Fatalf("UpdateSet() error = %v; committed = %v, rolledBack = %v", err, tt.connection.committed, tt.connection.rolledBack)
			}
			if tt.wantSteps != nil && !reflect.DeepEqual(tt.connection.steps, tt.wantSteps) {
				t.Fatalf("steps = %v, want %v", tt.connection.steps, tt.wantSteps)
			}
			for _, step := range tt.connection.steps {
				if step == "pr" || step == "stats" {
					t.Fatalf("stats recalculated after rejected correction: %v", tt.connection.steps)
				}
			}
		})
	}
}

type correctionConnection struct {
	workoutMissing bool
	workoutEnded   bool
	setFound       bool
	steps          []string
	updateArgs     []driver.Value
	committed      bool
	rolledBack     bool
}

type correctionConnector struct{ connection *correctionConnection }

func (c correctionConnector) Connect(context.Context) (driver.Conn, error) { return c.connection, nil }
func (c correctionConnector) Driver() driver.Driver                        { return c }
func (c correctionConnector) Open(string) (driver.Conn, error)             { return c.connection, nil }

func newCorrectionRepository(t *testing.T, connection *correctionConnection) *Repository {
	t.Helper()
	db := sql.OpenDB(correctionConnector{connection: connection})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return New(db)
}

func (*correctionConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (*correctionConnection) Close() error                { return nil }
func (c *correctionConnection) Begin() (driver.Tx, error) { return c, nil }
func (c *correctionConnection) Commit() error             { c.committed = true; return nil }
func (c *correctionConnection) Rollback() error           { c.rolledBack = true; return nil }

func (c *correctionConnection) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	switch {
	case strings.Contains(query, "WITH ordered_sets"):
		c.steps = append(c.steps, "pr")
	case strings.Contains(query, "INSERT INTO user_exercise_stats"):
		c.steps = append(c.steps, "stats")
	default:
		return nil, errors.New("unexpected exec: " + query)
	}
	return driver.RowsAffected(1), nil
}

func (c *correctionConnection) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "SELECT ended_at FROM workouts"):
		c.steps = append(c.steps, "lock")
		if c.workoutMissing {
			return &correctionRows{columns: 1}, nil
		}
		var endedAt driver.Value
		if c.workoutEnded {
			endedAt = time.Now()
		}
		return &correctionRows{columns: 1, values: [][]driver.Value{{endedAt}}}, nil
	case strings.Contains(query, "UPDATE workout_sets SET"):
		c.steps = append(c.steps, "update")
		for _, arg := range args[:3] {
			c.updateArgs = append(c.updateArgs, arg.Value)
		}
		return c.exerciseRow(), nil
	case strings.Contains(query, "DELETE FROM workout_sets"):
		c.steps = append(c.steps, "delete")
		return c.exerciseRow(), nil
	case strings.Contains(query, "SELECT latest.weight"):
		c.steps = append(c.steps, "latest")
		return &correctionRows{columns: 2, values: [][]driver.Value{{80.0, 80.0}}}, nil
	case strings.Contains(query, "FROM workout_sets WHERE id=$1 AND workout_id=$2"):
		c.steps = append(c.steps, "select")
		return &correctionRows{columns: 9, values: [][]driver.Value{{int64(7), int64(42), "bench", int64(1), 80.0, int64(8), "", false, time.Now()}}}, nil
	}
	return nil, errors.New("unexpected query: " + query)
}

func (c *correctionConnection) exerciseRow() driver.Rows {
	if !c.setFound {
		return &correctionRows{columns: 1}
	}
	return &correctionRows{columns: 1, values: [][]driver.Value{{"bench"}}}
}

type correctionRows struct {
	columns int
	values  [][]driver.Value
}

func (r *correctionRows) Columns() []string { return make([]string, r.columns) }
func (*correctionRows) Close() error        { return nil }
func (r *correctionRows) Next(dest []driver.Value) error {
	if len(r.values) == 0 {
		return io.EOF
	}
	copy(dest, r.values[0])
	r.values = r.values[1:]
	return nil
}
