package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func main() {
	// docker/env/backend.env から環境変数を読み込む
	if err := godotenv.Load("../docker/env/backend.env"); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// データベース初期化
	initDB()

	// エンドポイント
	// CROS Wrapper
	cors := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			h(w, r)
		}
	}

	http.HandleFunc("/api/recommend", cors(handleRecommend))
	http.HandleFunc("/api/monthly-plan", cors(handleMonthlyPlan))
	http.HandleFunc("/api/dashboard", cors(handleDashboard))
	http.HandleFunc("/api/calendar", cors(handleCalendar))
	http.HandleFunc("/api/alternative", cors(handleAlternative))

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}