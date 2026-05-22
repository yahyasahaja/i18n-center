package apikey

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/your-org/i18n-center/repository"
)

const (
	queryGetByHash = `
		SELECT id, application_id, key_hash, key_prefix, name, created_at
		FROM application_api_keys
		WHERE key_hash = $1
		  AND deleted_at IS NULL
		LIMIT 1
	`

	queryListByApp = `
		SELECT id, application_id, key_hash, key_prefix, name, created_at
		FROM application_api_keys
		WHERE application_id = $1
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	queryGetByIDForApp = `
		SELECT id, application_id, key_hash, key_prefix, name, created_at
		FROM application_api_keys
		WHERE id = $1
		  AND application_id = $2
		  AND deleted_at IS NULL
		LIMIT 1
	`

	queryInsert = `
		INSERT INTO application_api_keys (id, application_id, key_hash, key_prefix, name, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`

	querySoftDelete = `
		UPDATE application_api_keys
		SET deleted_at = NOW()
		WHERE id = $1
		  AND application_id = $2
		  AND deleted_at IS NULL
	`
)

type Impl struct{}

func New() Repository { return &Impl{} }

func (r *Impl) GetByHash(ctx context.Context, q repository.Queryer, hash string) (*APIKey, error) {
	var k APIKey
	if err := q.GetContext(ctx, &k, queryGetByHash, hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &k, nil
}

func (r *Impl) ListByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]APIKey, error) {
	keys := []APIKey{}
	if err := q.SelectContext(ctx, &keys, queryListByApp, appID); err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *Impl) GetByIDForApp(ctx context.Context, q repository.Queryer, id, appID uuid.UUID) (*APIKey, error) {
	var k APIKey
	if err := q.GetContext(ctx, &k, queryGetByIDForApp, id, appID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &k, nil
}

func (r *Impl) Create(ctx context.Context, q repository.Queryer, k *APIKey) error {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	_, err := q.ExecContext(ctx, queryInsert,
		k.ID, k.ApplicationID, k.KeyHash, k.KeyPrefix, k.Name,
	)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	k.CreatedAt = time.Now()
	return nil
}

func (r *Impl) SoftDelete(ctx context.Context, q repository.Queryer, id, appID uuid.UUID) error {
	result, err := q.ExecContext(ctx, querySoftDelete, id, appID)
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
	return nil
}
