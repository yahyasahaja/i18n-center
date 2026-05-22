package cms

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/your-org/i18n-center/repository"
)

// ─── Template queries ────────────────────────────────────────────────────────

const (
	queryGetTemplateByID = `
		SELECT id, application_id, name, code, description,
		       created_by, updated_by, created_at, updated_at
		FROM cms_templates
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	queryListTemplatesByApp = `
		SELECT id, application_id, name, code, description,
		       created_by, updated_by, created_at, updated_at
		FROM cms_templates
		WHERE application_id = $1
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	queryInsertTemplate = `
		INSERT INTO cms_templates (
			id, application_id, name, code, description,
			created_by, updated_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $6, NOW(), NOW())
	`

	queryUpdateTemplate = `
		UPDATE cms_templates
		SET name = $2,
		    description = $3,
		    updated_by = $4,
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	querySoftDeleteTemplate = `
		UPDATE cms_templates
		SET deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	queryLoadFields = `
		SELECT id, template_id, key, label, value_type, required, sort_order, created_at, updated_at
		FROM cms_template_fields
		WHERE template_id = $1
		ORDER BY sort_order, key
	`

	queryDeleteAllFields = `
		DELETE FROM cms_template_fields WHERE template_id = $1
	`

	queryInsertField = `
		INSERT INTO cms_template_fields (
			id, template_id, key, label, value_type, required, sort_order, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`

	queryCountItemsForTemplate = `
		SELECT COUNT(*) FROM cms_items
		WHERE template_id = $1
		  AND deleted_at IS NULL
	`
)

// ─── Template implementation ─────────────────────────────────────────────────

type templateImpl struct{}

// NewTemplateRepository returns the default Template+Field repository.
func NewTemplateRepository() TemplateRepository { return &templateImpl{} }

func (r *templateImpl) GetByID(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Template, error) {
	var t Template
	if err := q.GetContext(ctx, &t, queryGetTemplateByID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *templateImpl) GetByIDWithFields(ctx context.Context, q repository.Queryer, id uuid.UUID) (*Template, error) {
	t, err := r.GetByID(ctx, q, id)
	if err != nil {
		return nil, err
	}
	fields, err := r.LoadFields(ctx, q, t.ID)
	if err != nil {
		return nil, err
	}
	t.Fields = fields
	return t, nil
}

func (r *templateImpl) ListByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]Template, error) {
	out := []Template{}
	if err := q.SelectContext(ctx, &out, queryListTemplatesByApp, appID); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *templateImpl) Create(ctx context.Context, q repository.Queryer, t *Template) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if _, err := q.ExecContext(ctx, queryInsertTemplate,
		t.ID, t.ApplicationID, t.Name, t.Code, t.Description, t.CreatedBy,
	); err != nil {
		if repository.IsUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *templateImpl) Update(ctx context.Context, q repository.Queryer, t *Template) error {
	result, err := q.ExecContext(ctx, queryUpdateTemplate, t.ID, t.Name, t.Description, t.UpdatedBy)
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

func (r *templateImpl) SoftDelete(ctx context.Context, q repository.Queryer, id uuid.UUID) error {
	result, err := q.ExecContext(ctx, querySoftDeleteTemplate, id)
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

func (r *templateImpl) LoadFields(ctx context.Context, q repository.Queryer, templateID uuid.UUID) ([]TemplateField, error) {
	out := []TemplateField{}
	if err := q.SelectContext(ctx, &out, queryLoadFields, templateID); err != nil {
		return nil, err
	}
	return out, nil
}

// ReplaceFields wipes existing fields and inserts the provided list inside a
// single sqlx call sequence. The caller is expected to wrap this in
// repository.WithTx if they care about atomicity across the delete + bulk
// insert; with autocommit, a crash between the two leaves zero fields, which
// is the right "fail closed" behaviour anyway.
func (r *templateImpl) ReplaceFields(ctx context.Context, q repository.Queryer, templateID uuid.UUID, fields []TemplateField) error {
	if _, err := q.ExecContext(ctx, queryDeleteAllFields, templateID); err != nil {
		return err
	}
	for i := range fields {
		f := &fields[i]
		if !IsValidValueType(f.ValueType) {
			return fmt.Errorf("invalid value_type %q for field %q", f.ValueType, f.Key)
		}
		if f.ID == uuid.Nil {
			f.ID = uuid.New()
		}
		f.TemplateID = templateID
		if _, err := q.ExecContext(ctx, queryInsertField,
			f.ID, f.TemplateID, f.Key, f.Label, f.ValueType, f.Required, f.SortOrder,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *templateImpl) CountItemsForTemplate(ctx context.Context, q repository.Queryer, templateID uuid.UUID) (int, error) {
	var n int
	if err := q.GetContext(ctx, &n, queryCountItemsForTemplate, templateID); err != nil {
		return 0, err
	}
	return n, nil
}
