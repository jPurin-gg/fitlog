package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"github.com/jPurin-gg/myfitlog-backend/internal/auth"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) FindByNickname(ctx context.Context, nickname string) (auth.StoredUser, error) {
	var user auth.StoredUser
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, COALESCE(password_hash, '')
		FROM users
		WHERE username = $1
		LIMIT 1
	`, nickname).Scan(&user.ID, &user.Nickname, &user.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.StoredUser{}, auth.ErrUserNotFound
	}
	return user, err
}

// FindSimilarNicknames compares nicknames after NFKC normalization and lowercasing, which folds
// full-width/half-width forms and letter case. Two rows are enough for callers to see ambiguity.
func (r *Repository) FindSimilarNicknames(ctx context.Context, nickname string) ([]auth.StoredUser, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, username, COALESCE(password_hash, '')
		FROM users
		WHERE lower(normalize(username, NFKC)) = lower(normalize($1::text, NFKC))
		ORDER BY id
		LIMIT 2
	`, nickname)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []auth.StoredUser
	for rows.Next() {
		var user auth.StoredUser
		if err := rows.Scan(&user.ID, &user.Nickname, &user.PasswordHash); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *Repository) FindByID(ctx context.Context, userID int) (auth.User, error) {
	var user auth.User
	err := r.db.QueryRowContext(ctx, `SELECT id, username FROM users WHERE id = $1`, userID).Scan(&user.ID, &user.Nickname)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, auth.ErrUserNotFound
	}
	return user, err
}

func (r *Repository) Create(ctx context.Context, nickname, passwordHash string) (auth.User, error) {
	var user auth.User
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
		RETURNING id, username
	`, nickname, passwordHash).Scan(&user.ID, &user.Nickname)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return auth.User{}, auth.ErrNicknameTaken
	}
	return user, err
}

func (r *Repository) SetPasswordHash(ctx context.Context, userID int, passwordHash string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, passwordHash, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return auth.ErrUserNotFound
	}
	return nil
}
