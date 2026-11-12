//go:build integration

package app_test

import (
	"context"
	"database/sql"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/body"
	"github.com/jPurin-gg/myfitlog-backend/internal/database"
)

func TestBodyMeasurementsMigrationUpgradesExistingDatabase(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		name := "legacy_without_migration_history"
		if tracked {
			name = "existing_with_migration_history"
		}
		t.Run(name, func(t *testing.T) {
			db := newIntegrationDatabase(t)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			// Build the actual pre-body schema, without applying 00003 first.
			for _, version := range []string{"00001_baseline.sql", "00002_auth_and_idempotency.sql"} {
				if _, err := db.ExecContext(ctx, migrationSQL(t, version)); err != nil {
					t.Fatalf("prepare old schema with %s: %v", version, err)
				}
			}
			if tracked {
				if _, err := db.ExecContext(ctx, `
					CREATE TABLE schema_migrations (
						version TEXT PRIMARY KEY,
						applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
					);
					INSERT INTO schema_migrations (version,applied_at) VALUES
						('00001_baseline.sql','2026-10-01T00:00:00Z'),
						('00002_auth_and_idempotency.sql','2026-10-02T00:00:00Z')
				`); err != nil {
					t.Fatal(err)
				}
			}
			var userID, workoutID, setID int
			if err := db.QueryRowContext(ctx, `INSERT INTO users (username,password_hash)
				VALUES ('migration-user','test-only') RETURNING id`).Scan(&userID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO exercises (id,name) VALUES ('migration-exercise','Migration Exercise')`); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRowContext(ctx, `INSERT INTO workouts (user_id,notes)
				VALUES ($1,'Existing workout') RETURNING id`, userID).Scan(&workoutID); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRowContext(ctx, `INSERT INTO workout_sets (workout_id,exercise_id,weight,reps,set_order,idempotency_key)
				VALUES ($1,'migration-exercise',40,8,1,'existing-set-key') RETURNING id`, workoutID).Scan(&setID); err != nil {
				t.Fatal(err)
			}
			var relation sql.NullString
			if err := db.QueryRowContext(ctx, `SELECT to_regclass('body_measurements')::text`).Scan(&relation); err != nil {
				t.Fatal(err)
			}
			if relation.Valid {
				t.Fatal("old schema already has body_measurements")
			}

			if err := database.Migrate(ctx, db); err != nil {
				t.Fatalf("upgrade existing database: %v", err)
			}
			history := migrationHistory(t, db)
			if _, ok := history["00003_body_measurements.sql"]; !ok {
				t.Fatal("body migration was not recorded")
			}
			if tracked {
				for version, date := range map[string]string{
					"00001_baseline.sql":             "2026-10-01T00:00:00Z",
					"00002_auth_and_idempotency.sql": "2026-10-02T00:00:00Z",
				} {
					if history[version].UTC().Format(time.RFC3339) != date {
						t.Fatalf("upgrade changed the applied time of %s", version)
					}
				}
			}

			measurement := body.Measurement{MeasuredOn: "2026-11-10", WeightKg: 68.5}
			if err := db.QueryRowContext(ctx, `INSERT INTO body_measurements (user_id,measured_on,weight_kg)
				VALUES ($1,$2,$3) RETURNING id`, userID, measurement.MeasuredOn, measurement.WeightKg).Scan(&measurement.ID); err != nil {
				t.Fatalf("use new body table: %v", err)
			}
			var createdAt, updatedAt time.Time
			if err := db.QueryRowContext(ctx, `SELECT created_at,updated_at FROM body_measurements WHERE id=$1`, measurement.ID).Scan(&createdAt, &updatedAt); err != nil {
				t.Fatal(err)
			}
			for attempt := range 2 {
				if err := database.Migrate(ctx, db); err != nil {
					t.Fatalf("repeat Migrate() attempt %d: %v", attempt+1, err)
				}
			}
			// Also verify the migration SQL itself can be replayed against an
			// existing body table and index without replacing stored records.
			if _, err := db.ExecContext(ctx, migrationSQL(t, "00003_body_measurements.sql")); err != nil {
				t.Fatalf("replay body migration SQL: %v", err)
			}
			if got := migrationHistory(t, db); !reflect.DeepEqual(got, history) {
				t.Fatalf("repeated migration changed history: got %v, want %v", got, history)
			}
			assertStoredMeasurement(t, db, measurement)
			var storedUserID, bodyCount, indexCount int
			var storedCreatedAt, storedUpdatedAt time.Time
			if err := db.QueryRowContext(ctx, `SELECT user_id,created_at,updated_at FROM body_measurements WHERE id=$1`, measurement.ID).
				Scan(&storedUserID, &storedCreatedAt, &storedUpdatedAt); err != nil {
				t.Fatal(err)
			}
			if storedUserID != userID || !storedCreatedAt.Equal(createdAt) || !storedUpdatedAt.Equal(updatedAt) {
				t.Fatal("repeated migration changed body ownership or timestamps")
			}
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM body_measurements`).Scan(&bodyCount); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_indexes
				WHERE schemaname=current_schema() AND indexname='body_measurements_user_measured_on_idx'`).Scan(&indexCount); err != nil {
				t.Fatal(err)
			}
			if bodyCount != 1 || indexCount != 1 {
				t.Fatalf("body rows=%d indexes=%d, want 1/1", bodyCount, indexCount)
			}
			var preserved int
			if err := db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM workout_sets ws
				JOIN workouts w ON w.id=ws.workout_id
				JOIN users u ON u.id=w.user_id
				JOIN exercises e ON e.id=ws.exercise_id
				WHERE ws.id=$1 AND w.id=$2 AND u.id=$3
				  AND u.username='migration-user' AND w.notes='Existing workout'
				  AND e.id='migration-exercise' AND e.name='Migration Exercise'
				  AND ws.weight=40 AND ws.reps=8 AND ws.set_order=1 AND ws.idempotency_key='existing-set-key'
			`, setID, workoutID, userID).Scan(&preserved); err != nil {
				t.Fatal(err)
			}
			if preserved != 1 {
				t.Fatal("upgrade or repeated migration changed existing workout data")
			}
		})
	}
}

func migrationSQL(t *testing.T, version string) string {
	t.Helper()
	contents, err := os.ReadFile("../database/migrations/" + version)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func migrationHistory(t *testing.T, db *sql.DB) map[string]time.Time {
	t.Helper()
	rows, err := db.Query(`SELECT version,applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	history := map[string]time.Time{}
	for rows.Next() {
		var version string
		var appliedAt time.Time
		if err := rows.Scan(&version, &appliedAt); err != nil {
			t.Fatal(err)
		}
		history[version] = appliedAt
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return history
}
