package database

import (
	"context"
	"database/sql"
	"time"
)

// LockWorkoutDate serializes workout creation and replacement, even when no row
// exists yet. Acquire it before plan/workout row locks and hold it until commit.
func LockWorkoutDate(ctx context.Context, tx *sql.Tx, userID int, date time.Time) error {
	// Reserve the two-integer advisory lock space for (user ID, local YYYYMMDD).
	// Migration locks use the separate single-bigint advisory lock space.
	dateKey := date.Year()*10000 + int(date.Month())*100 + date.Day()
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, userID, dateKey)
	return err
}
