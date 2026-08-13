package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("secret-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	valid, err := VerifyPassword("secret-password", hash)
	if err != nil || !valid {
		t.Fatalf("VerifyPassword() = %v, %v; want true, nil", valid, err)
	}
	valid, err = VerifyPassword("wrong-password", hash)
	if err != nil || valid {
		t.Fatalf("VerifyPassword(wrong) = %v, %v; want false, nil", valid, err)
	}
}

func TestTokenSignerRejectsTamperingAndExpiry(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	signer := NewTokenSigner([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	token, err := signer.Sign(42, now)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	userID, err := signer.Verify(token, now.Add(30*time.Minute))
	if err != nil || userID != 42 {
		t.Fatalf("Verify() = %d, %v; want 42, nil", userID, err)
	}
	if _, err := signer.Verify(token+"x", now); err == nil {
		t.Fatal("Verify(tampered) error = nil")
	}
	if _, err := signer.Verify(token, now.Add(time.Hour)); err == nil {
		t.Fatal("Verify(expired) error = nil")
	}
}

func TestLoginRegistersAndClaimsLegacyUser(t *testing.T) {
	repository := &memoryRepository{users: map[string]StoredUser{}, nextID: 1}
	service := NewService(repository)

	created, wasCreated, err := service.Login(context.Background(), " Mitsuki ", "password")
	if err != nil || !wasCreated || created.Nickname != "Mitsuki" {
		t.Fatalf("first Login() = %#v, %v, %v", created, wasCreated, err)
	}
	loggedIn, wasCreated, err := service.Login(context.Background(), "Mitsuki", "password")
	if err != nil || wasCreated || loggedIn.ID != created.ID {
		t.Fatalf("second Login() = %#v, %v, %v", loggedIn, wasCreated, err)
	}
	if _, _, err := service.Login(context.Background(), "Mitsuki", "wrong"); err == nil {
		t.Fatal("Login(wrong password) error = nil")
	}

	repository.users["Legacy"] = StoredUser{User: User{ID: 99, Nickname: "Legacy"}}
	legacy, wasCreated, err := service.Login(context.Background(), "Legacy", "claimed-password")
	if err != nil || wasCreated || legacy.ID != 99 || repository.users["Legacy"].PasswordHash == "" {
		t.Fatalf("legacy Login() = %#v, %v, %v", legacy, wasCreated, err)
	}
}

type memoryRepository struct {
	users  map[string]StoredUser
	nextID int
}

func (r *memoryRepository) FindByNickname(_ context.Context, nickname string) (StoredUser, error) {
	user, ok := r.users[nickname]
	if !ok {
		return StoredUser{}, ErrUserNotFound
	}
	return user, nil
}

func (r *memoryRepository) FindByID(_ context.Context, userID int) (User, error) {
	for _, user := range r.users {
		if user.ID == userID {
			return user.User, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (r *memoryRepository) Create(_ context.Context, nickname, passwordHash string) (User, error) {
	if _, exists := r.users[nickname]; exists {
		return User{}, errors.New("duplicate user")
	}
	user := User{ID: r.nextID, Nickname: nickname}
	r.nextID++
	r.users[nickname] = StoredUser{User: user, PasswordHash: passwordHash}
	return user, nil
}

func (r *memoryRepository) SetPasswordHash(_ context.Context, userID int, passwordHash string) error {
	for nickname, user := range r.users {
		if user.ID == userID {
			user.PasswordHash = passwordHash
			r.users[nickname] = user
			return nil
		}
	}
	return ErrUserNotFound
}
