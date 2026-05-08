package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

type App struct {
	db *sql.DB
}

func main() {
	// docker/env/backend.env から環境変数を読み込む
	if err := godotenv.Load("../docker/env/backend.env"); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// データベース初期化
	db := initDB()
	app := &App{db: db}

	// エンドポイント
	// CORS Wrapper
	cors := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			allowedOrigin := os.Getenv("FRONTEND_URL")
			if allowedOrigin == "" {
				allowedOrigin = "http://localhost:3000"
			}
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			h(w, r)
		}
	}

	http.HandleFunc("/api/recommend", cors(app.handleRecommend))
	http.HandleFunc("/api/monthly-plan", cors(app.handleMonthlyPlan))
	http.HandleFunc("/api/dashboard", cors(app.handleDashboard))
	http.HandleFunc("/api/calendar", cors(app.handleCalendar))
	http.HandleFunc("/api/alternative", cors(app.handleAlternative))

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}