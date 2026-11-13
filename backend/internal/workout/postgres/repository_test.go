package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
	"time"
)

func TestDetailPreservesLargeRepetitionTotals(t *testing.T) {
	connection := &summaryConnection{exercises: [][]driver.Value{
		{"bench", "Bench", int64(2), int64(4294967294), float64(80), float64(343597383520), int64(1)},
		{"squat", "Squat", int64(1), int64(2147483647), float64(100), float64(214748364700), int64(1)},
	}}
	detail, err := newSummaryRepository(t, connection).Detail(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Summary.TotalReps != 6442450941 || detail.Summary.Exercises[0].TotalReps != 4294967294 || detail.Summary.TotalVolume != 558345748220 {
		t.Fatalf("Detail() summary = %+v", detail.Summary)
	}
}

func TestFinishAggregatesBeforeCommit(t *testing.T) {
	for _, reps := range []int64{2000, 4294967294} {
		connection := &summaryConnection{exercises: [][]driver.Value{
			{"bench", "Bench", int64(2), reps, float64(2000), 2000 * float64(reps), int64(1)},
		}}
		detail, err := newSummaryRepository(t, connection).Finish(context.Background(), 1, 2)
		if err != nil || !connection.committed || connection.rolledBack || detail.Status != "completed" || detail.EndedAt == "" || detail.Summary.TotalReps != reps || detail.Summary.TotalVolume != 2000*float64(reps) {
			t.Fatalf("Finish() = %+v, %v; committed=%v, rolledBack=%v", detail, err, connection.committed, connection.rolledBack)
		}
	}
}

func TestFinishRollsBackOnAggregationFailure(t *testing.T) {
	tests := []struct {
		name       string
		queryErr   error
		rowErr     error
		bestWeight float64
		volume     float64
	}{
		{name: "query_failure", queryErr: errors.New("aggregate failed")},
		{name: "row_failure", rowErr: errors.New("rows failed")},
		{name: "nan_weight", bestWeight: math.NaN()},
		{name: "infinite_weight", bestWeight: math.Inf(1)},
		{name: "nan_volume", volume: math.NaN()},
		{name: "infinite_volume", volume: math.Inf(1)},
		{name: "overflow_across_exercises", volume: math.MaxFloat64},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			connection := &summaryConnection{aggregateErr: test.queryErr, rowErr: test.rowErr, exercises: [][]driver.Value{
				{"bench", "Bench", int64(1), int64(1), test.bestWeight, test.volume, int64(0)},
				{"squat", "Squat", int64(1), int64(1), test.bestWeight, test.volume, int64(0)},
			}}
			_, err := newSummaryRepository(t, connection).Finish(context.Background(), 1, 2)
			if err == nil || connection.committed || !connection.rolledBack {
				t.Fatalf("Finish() error = %v; committed=%v, rolledBack=%v", err, connection.committed, connection.rolledBack)
			}
		})
	}
}

// This driver supplies result rows and transaction outcomes without a real DB.
// PostgreSQL's arithmetic itself is covered by the integration-tagged tests.
type summaryConnection struct {
	exercises    [][]driver.Value
	aggregateErr error
	rowErr       error
	completed    bool
	committed    bool
	rolledBack   bool
}

type summaryConnector struct{ connection *summaryConnection }

func (c summaryConnector) Driver() driver.Driver            { return c }
func (c summaryConnector) Open(string) (driver.Conn, error) { return c.connection, nil }
func (c summaryConnector) Connect(context.Context) (driver.Conn, error) {
	return c.connection, nil
}

func newSummaryRepository(t *testing.T, connection *summaryConnection) *Repository {
	t.Helper()
	db := sql.OpenDB(summaryConnector{connection: connection})
	t.Cleanup(func() { db.Close() })
	return New(db)
}

func (*summaryConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected Prepare")
}
func (*summaryConnection) Close() error                { return nil }
func (c *summaryConnection) Begin() (driver.Tx, error) { return c, nil }
func (c *summaryConnection) Commit() error             { c.committed = true; return nil }
func (c *summaryConnection) Rollback() error           { c.rolledBack = true; return nil }

func (c *summaryConnection) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if !strings.Contains(query, "UPDATE workouts") && !strings.Contains(query, "UPDATE workout_plans") {
		return nil, errors.New("unexpected ExecContext")
	}
	c.completed = true
	return driver.RowsAffected(1), nil
}

func (c *summaryConnection) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.committed {
		return nil, errors.New("query after commit")
	}
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	switch {
	case strings.Contains(query, "FOR UPDATE"):
		return &summaryRows{values: [][]driver.Value{{nil}}, columns: 1}, nil
	case strings.Contains(query, "SELECT id,COALESCE(notes"):
		var endedAt driver.Value
		status := "active"
		if c.completed {
			endedAt, status = now, "completed"
		}
		return &summaryRows{values: [][]driver.Value{{int64(2), "Workout", now, endedAt, status}}, columns: 5}, nil
	case strings.Contains(query, "SELECT GREATEST"):
		return &summaryRows{values: [][]driver.Value{{int64(1), ""}}, columns: 2}, nil
	case strings.Contains(query, "SUM(ws.reps)"):
		return &summaryRows{values: c.exercises, columns: 7, err: c.rowErr}, c.aggregateErr
	default:
		return nil, errors.New("unexpected QueryContext")
	}
}

type summaryRows struct {
	values  [][]driver.Value
	columns int
	err     error
}

func (r *summaryRows) Columns() []string { return make([]string, r.columns) }
func (*summaryRows) Close() error        { return nil }
func (r *summaryRows) Next(dest []driver.Value) error {
	if len(r.values) == 0 {
		if r.err != nil {
			return r.err
		}
		return io.EOF
	}
	copy(dest, r.values[0])
	r.values = r.values[1:]
	return nil
}
