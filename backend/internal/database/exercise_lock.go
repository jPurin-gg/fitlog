package database

import (
	"context"
	"database/sql"
	"hash/crc32"
	"slices"
)

// LockExerciseHistory serializes history and weight/max_weight changes for each
// user/exercise, including exercises without a user_exercise_stats row yet.
// Writers must use READ COMMITTED, lock their workout row first, then acquire
// ALL old and new exercise locks before changing history or reading aggregates.
// The locks are held until commit/rollback; no other workout rows may be locked
// after acquiring them. Aggregate reads must be separate, subsequent statements
// so they see changes committed while waiting for these locks.
func LockExerciseHistory(ctx context.Context, tx *sql.Tx, userID int, exerciseIDs []string) error {
	for _, key := range exerciseLockKeys(exerciseIDs) {
		// Negative exercise keys keep these locks separate from the positive
		// YYYYMMDD keys used by workout-date locks in the two-int lock space.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, userID, key); err != nil {
			return err
		}
	}
	return nil
}

func exerciseLockKeys(exerciseIDs []string) []int32 {
	keys := make([]int32, 0, len(exerciseIDs))
	for _, id := range exerciseIDs {
		keys = append(keys, int32(crc32.ChecksumIEEE([]byte(id))|(1<<31)))
	}
	// Sort the actual lock keys, rather than IDs: hash collisions may add
	// contention but must never reverse the acquisition order or cause deadlocks.
	slices.Sort(keys)
	return slices.Compact(keys)
}
