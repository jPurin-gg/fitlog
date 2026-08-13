package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

const passwordHashIterations = 120000

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrNicknameTaken = errors.New("nickname already exists")
)

type User struct {
	ID       int    `json:"id"`
	Nickname string `json:"nickname"`
}

type StoredUser struct {
	User
	PasswordHash string
}

type Repository interface {
	FindByNickname(ctx context.Context, nickname string) (StoredUser, error)
	FindByID(ctx context.Context, userID int) (User, error)
	Create(ctx context.Context, nickname, passwordHash string) (User, error)
	SetPasswordHash(ctx context.Context, userID int, passwordHash string) error
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Login(ctx context.Context, nickname, password string) (User, bool, error) {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" || password == "" {
		return User{}, false, apperr.Validation("ニックネームとパスワードを入力してください。", map[string]string{
			"nickname": "required",
			"password": "required",
		})
	}
	fields := map[string]string{}
	if len([]rune(nickname)) > 80 {
		fields["nickname"] = "must be at most 80 characters"
	}
	if len(password) > 256 {
		fields["password"] = "must be at most 256 bytes"
	}
	if len(fields) > 0 {
		return User{}, false, apperr.Validation("ニックネームまたはパスワードが長すぎます。", fields)
	}

	stored, err := s.repository.FindByNickname(ctx, nickname)
	switch {
	case err == nil:
		if stored.PasswordHash == "" {
			hash, hashErr := HashPassword(password)
			if hashErr != nil {
				return User{}, false, apperr.Internal(hashErr)
			}
			if updateErr := s.repository.SetPasswordHash(ctx, stored.ID, hash); updateErr != nil {
				return User{}, false, apperr.Internal(updateErr)
			}
			return stored.User, false, nil
		}
		valid, verifyErr := VerifyPassword(password, stored.PasswordHash)
		if verifyErr != nil {
			return User{}, false, apperr.Internal(verifyErr)
		}
		if !valid {
			return User{}, false, apperr.Unauthenticated("ニックネームまたはパスワードが違います。")
		}
		return stored.User, false, nil
	case !errors.Is(err, ErrUserNotFound):
		return User{}, false, apperr.Internal(err)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return User{}, false, apperr.Internal(err)
	}
	user, err := s.repository.Create(ctx, nickname, hash)
	if err != nil {
		if errors.Is(err, ErrNicknameTaken) {
			return User{}, false, apperr.Conflict("そのニックネームは直前に登録されました。もう一度ログインしてください。")
		}
		return User{}, false, apperr.Internal(err)
	}
	return user, true, nil
}

func (s *Service) Me(ctx context.Context, userID int) (User, error) {
	user, err := s.repository.FindByID(ctx, userID)
	if errors.Is(err, ErrUserNotFound) {
		return User{}, apperr.Unauthenticated("セッションのユーザーが存在しません。")
	}
	if err != nil {
		return User{}, apperr.Internal(err)
	}
	return user, nil
}

type TokenSigner struct {
	secret []byte
	ttl    time.Duration
}

type tokenPayload struct {
	Version int   `json:"v"`
	UserID  int   `json:"uid"`
	Expires int64 `json:"exp"`
}

func NewTokenSigner(secret []byte, ttl time.Duration) *TokenSigner {
	return &TokenSigner{secret: append([]byte(nil), secret...), ttl: ttl}
}

func (s *TokenSigner) Sign(userID int, now time.Time) (string, error) {
	payload, err := json.Marshal(tokenPayload{Version: 1, UserID: userID, Expires: now.Add(s.ttl).Unix()})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + signature, nil
}

func (s *TokenSigner) Verify(token string, now time.Time) (int, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, errors.New("invalid token format")
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(actual, expected) {
		return 0, errors.New("invalid token signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, errors.New("invalid token payload")
	}
	var payload tokenPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil || payload.Version != 1 || payload.UserID <= 0 {
		return 0, errors.New("invalid token payload")
	}
	if now.Unix() >= payload.Expires {
		return 0, errors.New("token expired")
	}
	return payload.UserID, nil
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := pbkdf2SHA256([]byte(password), salt, passwordHashIterations, 32)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", passwordHashIterations, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(password, stored string) (bool, error) {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false, nil
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 || iterations > 1_000_000 {
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
	if len(salt) < 8 || len(salt) > 64 || len(expected) < 16 || len(expected) > 64 {
		return false, nil
	}
	actual := pbkdf2SHA256([]byte(password), salt, iterations, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	hashLen := sha256.Size
	blocks := (keyLen + hashLen - 1) / hashLen
	output := make([]byte, 0, blocks*hashLen)
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for iteration := 1; iteration < iterations; iteration++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for index := range t {
				t[index] ^= u[index]
			}
		}
		output = append(output, t...)
	}
	return output[:keyLen]
}
