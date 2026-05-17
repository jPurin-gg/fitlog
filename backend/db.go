package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func initDB() *sql.DB {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}

	// Retry logic for DB connection
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		err = db.Ping()
		if err == nil {
			fmt.Println("Successfully connected to database")
			return db
		}
		log.Printf("Could not connect to db, retrying in 2 seconds... (%v)", err)
		time.Sleep(2 * time.Second)
	}

	log.Fatalf("Failed to connect to database after %d attempts: %v", maxRetries, err)
	return nil
}

func ensureSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS monthly_plans (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			plan_month TEXT NOT NULL,
			plan_name TEXT NOT NULL,
			frequency TEXT NOT NULL,
			description TEXT,
			rationale TEXT,
			recommended_days JSONB NOT NULL DEFAULT '[]'::jsonb,
			weekly_routine JSONB NOT NULL DEFAULT '[]'::jsonb,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (user_id, plan_month)
		);

		CREATE TABLE IF NOT EXISTS workout_plans (
			id SERIAL PRIMARY KEY,
			workout_id INTEGER REFERENCES workouts(id),
			user_id INTEGER REFERENCES users(id),
			plan_date DATE NOT NULL,
			title TEXT NOT NULL,
			estimated_duration_min INTEGER,
			status TEXT NOT NULL DEFAULT 'active',
			plan JSONB NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (user_id, plan_date, status)
		);

		ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;
		ALTER TABLE workouts ADD COLUMN IF NOT EXISTS summary_comment TEXT;
	`)
	return err
}
