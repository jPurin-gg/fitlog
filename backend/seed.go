package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type ExerciseData struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Force            *string  `json:"force"`
	Level            *string  `json:"level"`
	Mechanic         *string  `json:"mechanic"`
	Equipment        *string  `json:"equipment"`
	Category         *string  `json:"category"`
	Instructions     []string `json:"instructions"`
	PrimaryMuscles   []string `json:"primaryMuscles"`
	SecondaryMuscles []string `json:"secondaryMuscles"`
	Images           []string `json:"images"`
}

func seedDatabase(db *sql.DB) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM exercises").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check exercises count: %v", err)
	}

	// 既にデータが入っていればスキップ（最初にだけやる処理）
	if count > 0 {
		log.Println("Exercises table is already seeded. Skipping.")
		return nil
	}

	seedFile := os.Getenv("SEED_FILE_PATH")
	if seedFile == "" {
		seedFile = "tmpkin_jp.json" // 環境変数が設定されていない場合のデフォルト値
	}

	log.Printf("Seeding exercises table from %s...\n", seedFile)

	fileData, err := os.ReadFile(seedFile)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %v", err)
	}

	var exercises []ExerciseData
	if err := json.Unmarshal(fileData, &exercises); err != nil {
		return fmt.Errorf("failed to parse JSON data: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO exercises (
			id, name, force, level, mechanic, equipment, category,
			instructions, primary_muscles, secondary_muscles, images
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	for _, ex := range exercises {
		instructionsJSON, _ := json.Marshal(ex.Instructions)
		primaryMusclesJSON, _ := json.Marshal(ex.PrimaryMuscles)
		secondaryMusclesJSON, _ := json.Marshal(ex.SecondaryMuscles)
		imagesJSON, _ := json.Marshal(ex.Images)

		_, err := stmt.Exec(
			ex.ID, ex.Name, ex.Force, ex.Level, ex.Mechanic, ex.Equipment, ex.Category,
			string(instructionsJSON), string(primaryMusclesJSON), string(secondaryMusclesJSON), string(imagesJSON),
		)
		if err != nil {
			return fmt.Errorf("failed to insert exercise %s: %v", ex.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("Successfully seeded %d exercises.\n", len(exercises))
	return nil
}
