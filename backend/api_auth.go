package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const passwordHashIterations = 120000

type LoginRequest struct {
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

type LoginResponse struct {
	UserID   int    `json:"user_id"`
	Nickname string `json:"nickname"`
	Created  bool   `json:"created"`
}

func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" || req.Password == "" {
		http.Error(w, "ニックネームとパスワードを入力してください。", http.StatusBadRequest)
		return
	}

	rows, err := app.db.Query(`
		SELECT id, COALESCE(password_hash, '')
		FROM users
		WHERE username = $1
		ORDER BY id
	`, nickname)
	if err != nil {
		http.Error(w, "ログインに失敗しました。", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var firstLegacyUserID int
	for rows.Next() {
		var userID int
		var storedHash string
		if err := rows.Scan(&userID, &storedHash); err != nil {
			http.Error(w, "ログインに失敗しました。", http.StatusInternalServerError)
			return
		}
		if storedHash == "" {
			if firstLegacyUserID == 0 {
				firstLegacyUserID = userID
			}
			continue
		}
		ok, err := verifyPassword(req.Password, storedHash)
		if err != nil {
			http.Error(w, "ログインに失敗しました。", http.StatusInternalServerError)
			return
		}
		if ok {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(LoginResponse{UserID: userID, Nickname: nickname, Created: false})
			return
		}
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "ログインに失敗しました。", http.StatusInternalServerError)
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		http.Error(w, "ログインに失敗しました。", http.StatusInternalServerError)
		return
	}

	if firstLegacyUserID > 0 {
		if _, err := app.db.Exec("UPDATE users SET password_hash = $1 WHERE id = $2", hash, firstLegacyUserID); err != nil {
			http.Error(w, "ログインに失敗しました。", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{UserID: firstLegacyUserID, Nickname: nickname, Created: false})
		return
	}

	var userID int
	if err := app.db.QueryRow(`
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
		RETURNING id
	`, nickname, hash).Scan(&userID); err != nil {
		http.Error(w, "ユーザー作成に失敗しました。", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{UserID: userID, Nickname: nickname, Created: true})
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := pbkdf2SHA256([]byte(password), salt, passwordHashIterations, 32)
	return fmt.Sprintf(
		"pbkdf2_sha256$%d$%s$%s",
		passwordHashIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func verifyPassword(password, stored string) (bool, error) {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false, nil
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, err
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, err
	}

	actual := pbkdf2SHA256([]byte(password), salt, iterations, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	hashLen := sha256.Size
	numBlocks := (keyLen + hashLen - 1) / hashLen
	output := make([]byte, 0, numBlocks*hashLen)

	for block := 1; block <= numBlocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)

		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		output = append(output, t...)
	}

	return output[:keyLen]
}
