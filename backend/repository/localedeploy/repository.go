// Package localedeploy is the data access layer for `application_locale_deploys` —
// the per-application, per-locale deploy-progress ledger. One row per locale
// tracks where in the draft → staging → production pipeline that locale sits.
package localedeploy

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
)

// Stage values stored in StageCompleted.
const (
	StageDraft      = "draft"
	StageStaging    = "staging"
	StageProduction = "production"
)

// Deploy is one row from application_locale_deploys.
type Deploy struct {
	ID             uuid.UUID `db:"id"              json:"id"`
	ApplicationID  uuid.UUID `db:"application_id"  json:"application_id"`
	Locale         string    `db:"locale"          json:"locale"`
	StageCompleted string    `db:"stage_completed" json:"stage_completed"`
	CreatedAt      time.Time `db:"created_at"      json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"      json:"updated_at"`
}

type Repository interface {
	GetByAppLocale(ctx context.Context, q repository.Queryer, appID uuid.UUID, locale string) (*Deploy, error)
	// ListPendingByApp returns every locale that hasn't reached production yet.
	ListPendingByApp(ctx context.Context, q repository.Queryer, appID uuid.UUID) ([]Deploy, error)
	// Upsert creates the row if missing, otherwise updates StageCompleted.
	// Used by both the AddLanguage worker (initial creation) and the
	// DeployLocale handler (progression).
	Upsert(ctx context.Context, q repository.Queryer, d *Deploy) error
	// SetStage updates StageCompleted for (appID, locale).
	SetStage(ctx context.Context, q repository.Queryer, appID uuid.UUID, locale, stage string) error
	// Delete removes the row hard (no soft-delete needed — these are derived state).
	Delete(ctx context.Context, q repository.Queryer, appID uuid.UUID, locale string) error
}
