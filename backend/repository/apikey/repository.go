// Package apikey is the data access layer for `application_api_keys` —
// application-scoped secret keys used by client apps (FE1, FE2, Go services)
// to authenticate to the public translation/CMS read endpoints.
//
// Only the SHA-256 hash is persisted. The full key is shown once at creation
// time and never recoverable thereafter. Lookup-by-hash is the hot path for
// every API-key-authenticated request, so KeyHash is uniquely indexed.
package apikey

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/your-org/i18n-center/repository"
)

// APIKey is the in-memory representation of `application_api_keys`. The full
// key value (the `sk_…` string) is NEVER stored — only its hash and a short
// display prefix. The literal key is returned exactly once by the create
// handler.
type APIKey struct {
	ID            uuid.UUID `db:"id"             json:"id"`
	ApplicationID uuid.UUID `db:"application_id" json:"application_id"`
	KeyHash       string    `db:"key_hash"       json:"-"`
	KeyPrefix     string    `db:"key_prefix"     json:"key_prefix"`
	Name          string    `db:"name"           json:"name"`
	CreatedAt     time.Time `db:"created_at"     json:"created_at"`
}

// Repository is the contract for API key persistence.
type Repository interface {
	// GetByHash looks up a key by its SHA-256 hex hash. Used by the auth
	// middleware on every API-key-authenticated request — must be cheap.
	// ErrNotFound when no match or soft-deleted.
	GetByHash(ctx context.Context, q repository.Queryer, hash string) (*APIKey, error)

	// ListByApp returns every non-deleted key for an application, newest
	// first. Used by the admin UI's keys-management page.
	ListByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]APIKey, error)

	// Create inserts a new key. The plaintext key is hashed by the caller
	// before reaching this layer — this repository never sees it. ErrConflict
	// on duplicate hash (astronomically unlikely with a 256-bit secret but
	// we still handle it).
	Create(ctx context.Context, q repository.Queryer, k *APIKey) error

	// SoftDelete by ID, scoped to the application that owns the key.
	// ErrNotFound if the key doesn't belong to this app (prevents one app
	// from deleting another's keys via a guessed UUID).
	SoftDelete(ctx context.Context, q repository.Queryer, id, appID uuid.UUID) error

	// GetByIDForApp fetches a key by ID, scoped to the application that
	// owns it. Used by Delete before the audit log emits the row state.
	// ErrNotFound when the key isn't under this app.
	GetByIDForApp(ctx context.Context, q repository.Queryer, id, appID uuid.UUID) (*APIKey, error)
}
