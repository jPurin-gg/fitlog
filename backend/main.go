package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// データベース初期化
	initDB()

	// エンドポイント
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pong")
	})

	http.HandleFunc("/api/recommend", handleRecommend)

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}