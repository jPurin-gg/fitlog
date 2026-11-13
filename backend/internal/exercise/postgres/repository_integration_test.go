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
	"github.com/jPurin-gg/myfitlog-backend/internal/exercise"
	exercisepostgres "github.com/jPurin-gg/myfitlog-backend/internal/exercise/postgres"
)

const defaultIntegrationDSN = "host=127.0.0.1 port=55432 user=fitlog_test password=fitlog_test dbname=fitlog_test sslmode=disable"

func TestCreateUniqueReturnsExistingCatalogExercise(t *testing.T) {
	db := newIntegrationDB(t)
	if _, err := db.Exec(`INSERT INTO exercises (id, name, category, primary_muscles) VALUES ('bench_press', 'ベンチプレス', '筋力トレーニング', '["大胸筋"]')`); err != nil {
		t.Fatal(err)
	}
	repository := exercisepostgres.New(db)

	existing, err := repository.CreateUnique(context.Background(), customExercise("custom_dup", " ベンチ　プレス "))
	if !errors.Is(err, exercise.ErrDuplicateName) || existing.ID != "bench_press" || len(existing.PrimaryMuscles) != 1 {
		t.Fatalf("CreateUnique() = %#v, %v; want existing bench_press", existing, err)
	}
	assertExerciseCount(t, db, 1)
}

func TestCreateUniqueConcurrentSameName(t *testing.T) {
	db := newIntegrationDB(t)
	repository := exercisepostgres.New(db)

	const requests = 8
	start := make(chan struct{})
	var created, duplicates int
	var mutex sync.Mutex
	var wait sync.WaitGroup
	for index := range requests {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := repository.CreateUnique(context.Background(), customExercise(fmt.Sprintf("custom_%d", index), "ブルガリアンスクワット"))
			mutex.Lock()
			defer mutex.Unlock()
			switch {
			case err == nil:
				created++
			case errors.Is(err, exercise.ErrDuplicateName):
				duplicates++
			default:
				t.Errorf("CreateUnique() error = %v", err)
			}
		}()
	}
	close(start)
	wait.Wait()
	if created != 1 || duplicates != requests-1 {
		t.Fatalf("created = %d, duplicates = %d", created, duplicates)
	}
	assertExerciseCount(t, db, 1)
}

func customExercise(id, name string) exercise.Exercise {
	category := "筋力トレーニング"
	return exercise.Exercise{ID: id, Name: name, Category: &category, PrimaryMuscles: []string{"大腿四頭筋"}, Instructions: []string{}, SecondaryMuscles: []string{}, Images: []string{}}
}

func assertExerciseCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM exercises`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("exercise count = %d, want %d", count, want)
	}
}

func newIntegrationDB(t *testing.T) *sql.DB {
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
	schema := fmt.Sprintf("fitlog_ex_it_%d", time.Now().UnixNano())
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
	db.SetMaxOpenConns(10)
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return db
}
