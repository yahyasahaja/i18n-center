package cms

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
)

const (
	queryGetItemByID = `
		SELECT id, application_id, template_id, identifier, name, description,
		       created_by, updated_by, created_at, updated_at
		FROM cms_items
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	queryGetItemByAppIdentifier = `
		SELECT id, application_id, template_id, identifier, name, description,
		       created_by, updated_by, created_at, updated_at
		FROM cms_items
		WHERE application_id = $1
		  AND identifier = $2
		  AND deleted_at IS NULL
		LIMIT 1
	`

	queryListItemsByApp = `
		SELECT id, application_id, template_id, identifier, name, description,
		       created_by, updated_by, created_at, updated_at
		FROM cms_items
		WHERE application_id = $1
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	queryInsertItem = `
		INSERT INTO cms_items (
			id, application_id, template_id, identifier, name, description,
			created_by, updated_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $7, NOW(), NOW())
	`

	queryUpdateItem = `
		UPDATE cms_items
		SET template_id = $2,
		    name = $3,
		    description = $4,
		    updated_by = $5,
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	querySoftDeleteItem = `
		UPDATE cms_items
		SET deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`
)

type itemImpl struct {
	templates TemplateRepository
}

// NewItemRepository returns the default Item repository. The injected
// TemplateRepository powers GetByIDWithTemplate so we don't duplicate the
// preload logic across packages.
func NewItemRepository(templates TemplateRepository) ItemRepository {
	return &itemImpl{templates: templates}
}

func (r *itemImpl) GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Item, error) {
	var i Item
	if err := q.GetContext(ctx, &i, queryGetItemByID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &i, nil
}

func (r *itemImpl) GetByIDWithTemplate(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Item, error) {
	i, err := r.GetByID(ctx, q, id)
	if err != nil {
		return nil, err
	}
	t, err := r.templates.GetByIDWithFields(ctx, q, i.TemplateID)
	if err != nil {
		// Item exists but template doesn't — orphaned data. Surface as not-found
		// rather than 500ing so the caller can clean up.
		return nil, err
	}
	i.Template = t
	return i, nil
}

func (r *itemImpl) GetByAppIdentifier(ctx context.Context, q repository.Queryer, appID uuid.UUID, identifier string) (*Item, error) {
	var i Item
	if err := q.GetContext(ctx, &i, queryGetItemByAppIdentifier, appID, identifier); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &i, nil
}

func (r *itemImpl) ListByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]Item, error) {
	out := []Item{}
	if err := q.SelectContext(ctx, &out, queryListItemsByApp, appID); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *itemImpl) Create(ctx context.Context, q repository.Queryer, i *Item) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	if _, err := q.ExecContext(ctx, queryInsertItem,
		i.ID, i.ApplicationID, i.TemplateID, i.Identifier, i.Name, i.Description, i.CreatedBy,
	); err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *itemImpl) Update(ctx context.Context, q repository.Queryer, i *Item) error {
	result, err := q.ExecContext(ctx, queryUpdateItem,
		i.ID, i.TemplateID, i.Name, i.Description, i.UpdatedBy,
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
	return nil
}

func (r *itemImpl) SoftDelete(ctx context.Context, q repository.Queryer, id uuid.UUID) error {
	result, err := q.ExecContext(ctx, querySoftDeleteItem, id)
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
