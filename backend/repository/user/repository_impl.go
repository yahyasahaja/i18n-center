package user

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
)

// ─── Queries ─────────────────────────────────────────────────────────────────

const (
	queryGetByID = `
		SELECT id, username, password_hash, role, is_active, created_at, updated_at
		FROM users
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	queryGetActiveByUsername = `
		SELECT id, username, password_hash, role, is_active, created_at, updated_at
		FROM users
		WHERE username = $1
		  AND is_active = TRUE
		  AND deleted_at IS NULL
		LIMIT 1
	`

	queryList = `
		SELECT id, username, password_hash, role, is_active, created_at, updated_at
		FROM users
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`

	queryInsert = `
		INSERT INTO users (id, username, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
	`

	queryUpdate = `
		UPDATE users
		SET role = $2,
		    is_active = $3,
		    password_hash = $4,
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`
)

// ─── Implementation ──────────────────────────────────────────────────────────

// Impl is the concrete Repository backed by a sqlx-compatible Queryer.
// Empty struct — all state is passed in via the Queryer arg so the same Impl
// instance is safe for concurrent use.
type Impl struct{}

// New returns a Repository implementation. Currently a zero-state struct;
// future per-instance config (e.g. statement cache) would attach here.
func New() Repository { return &Impl{} }

func (r *Impl) GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*User, error) {
	var u User
	if err := q.GetContext(ctx, &u, queryGetByID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *Impl) GetActiveByUsername(ctx context.Context, q repository.Queryer, username string) (*User, error) {
	var u User
	if err := q.GetContext(ctx, &u, queryGetActiveByUsername, username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *Impl) List(ctx context.Context, q repository.Queryer) ([]User, error) {
	users := []User{}
	if err := q.SelectContext(ctx, &users, queryList); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *Impl) Create(ctx context.Context, q repository.Queryer, u *User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	u.UpdatedAt = u.CreatedAt
	_, err := q.ExecContext(ctx, queryInsert,
		u.ID, u.Username, u.PasswordHash, u.Role, u.IsActive, u.CreatedAt,
	)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *Impl) Update(ctx context.Context, q repository.Queryer, u *User) error {
	result, err := q.ExecContext(ctx, queryUpdate,
		u.ID, u.Role, u.IsActive, u.PasswordHash,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return repository.ErrNotFound
	}
	u.UpdatedAt = time.Now()
	return nil
}
