// Package user is the data access layer for application users (admin / operator
// / user_manager).
//
// Users are soft-deleted. The `deleted_at IS NULL` filter is applied to every
// read; the partial unique index `idx_users_username` ensures the same username
// can be reused after a soft-delete (because deleted rows are excluded from
// the index).
package user

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
)

// Role constants for User.Role. Stored as TEXT — Postgres has no enum here
// so the application is the source of truth for the allowed set.
const (
	RoleSuperAdmin  = "super_admin"
	RoleOperator    = "operator"
	RoleUserManager = "user_manager"
)

// User is the in-memory representation of a row from the `users` table.
// JSON tags match what the public API has historically returned, so callers
// migrating off the GORM model don't have to update their consumers.
type User struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	Username     string    `db:"username"      json:"username"`
	PasswordHash string    `db:"password_hash" json:"-"`
	Role         string    `db:"role"          json:"role"`
	IsActive     bool      `db:"is_active"     json:"is_active"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// Repository is the contract for user persistence. Every method takes a
// `repository.Queryer` so the same call works inside or outside a transaction.
type Repository interface {
	// GetByID fetches a user by primary key. Returns repository.ErrNotFound if
	// no row matches (or the row is soft-deleted).
	GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*User, error)

	// GetActiveByUsername fetches a user by username, filtered to active and
	// non-deleted. Used by the login flow — never exposes inactive accounts
	// for auth. Returns repository.ErrNotFound when no match.
	GetActiveByUsername(ctx context.Context, q repository.Queryer, username string) (*User, error)

	// List returns every non-deleted user, ordered by created_at DESC.
	// Pagination not required at current scale; revisit if user count grows.
	List(ctx context.Context, q repository.Queryer) ([]User, error)

	// Create inserts a new user. Caller provides PasswordHash already bcrypted.
	// Returns repository.ErrConflict on duplicate username (matched via the
	// partial unique index idx_users_username).
	Create(ctx context.Context, q repository.Queryer, u *User) error

	// Update overwrites mutable fields (role, is_active, password_hash) for
	// the user with the given ID. Username changes are intentionally not
	// supported via Update — they would invalidate audit history and JWT
	// claims. Returns repository.ErrNotFound when the user is missing.
	Update(ctx context.Context, q repository.Queryer, u *User) error
}
