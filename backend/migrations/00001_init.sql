-- +goose Up
-- +goose StatementBegin

-- Initial schema for i18n-center.
--
-- This file is the canonical schema bootstrap. It is intended to be run ONCE
-- against a fresh database via `i18n-center migrate up`. Subsequent schema
-- changes go in new numbered files (00002_..., 00003_...) using goose
-- conventions and Postgres safe-pattern playbook (see migrations/README.md).

-- ─── Extensions ──────────────────────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ─── Users ───────────────────────────────────────────────────────────────────
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role          VARCHAR(50) NOT NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_users_username ON users (username) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_deleted_at ON users (deleted_at) WHERE deleted_at IS NOT NULL;

-- ─── Applications ────────────────────────────────────────────────────────────
CREATE TABLE applications (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL,
    code              TEXT NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    openai_key        TEXT NOT NULL DEFAULT '',          -- encrypted in prod; never returned in JSON
    enabled_languages TEXT[] NOT NULL DEFAULT '{}',
    created_by        UUID NOT NULL,
    updated_by        UUID NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_applications_code ON applications (code) WHERE deleted_at IS NULL;
CREATE INDEX idx_applications_created_by ON applications (created_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_applications_updated_by ON applications (updated_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_applications_deleted_at ON applications (deleted_at) WHERE deleted_at IS NOT NULL;

-- ─── Application API Keys ────────────────────────────────────────────────────
CREATE TABLE application_api_keys (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL,
    key_hash       VARCHAR(64) NOT NULL,                 -- SHA-256 hex of the full key
    key_prefix     VARCHAR(20) NOT NULL,                 -- first 12 chars for display (sk_abc12345)
    name           VARCHAR(255) NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_application_api_keys_hash ON application_api_keys (key_hash) WHERE deleted_at IS NULL;
CREATE INDEX idx_application_api_keys_app_id ON application_api_keys (application_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_application_api_keys_prefix ON application_api_keys (key_prefix) WHERE deleted_at IS NULL;

-- ─── Application Locale Deploys ──────────────────────────────────────────────
CREATE TABLE application_locale_deploys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL,
    locale          VARCHAR(20) NOT NULL,
    stage_completed VARCHAR(50) NOT NULL DEFAULT 'draft', -- draft | staging | production
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_app_locale ON application_locale_deploys (application_id, locale) WHERE deleted_at IS NULL;

-- ─── Tags ────────────────────────────────────────────────────────────────────
CREATE TABLE tags (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL,
    code           VARCHAR(100) NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_tag_app_code ON tags (application_id, code) WHERE deleted_at IS NULL;

-- ─── Pages ───────────────────────────────────────────────────────────────────
CREATE TABLE pages (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL,
    code           VARCHAR(100) NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_page_app_code ON pages (application_id, code) WHERE deleted_at IS NULL;

-- ─── Components ──────────────────────────────────────────────────────────────
-- NOTE: Component.Structure (jsonb) was removed during the GORM→raw-SQL
-- rewrite. It was stored on every Component and audited but never read at
-- runtime. If a future schema validation feature needs it, add it back via
-- a new migration.
CREATE TABLE components (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL,
    name           TEXT NOT NULL,
    code           TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    key_contexts   JSONB,                                 -- optional flat {dot.path: hint} map for AI translation hints
    default_locale TEXT NOT NULL,
    created_by     UUID NOT NULL,
    updated_by     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_component_app_code ON components (application_id, code) WHERE deleted_at IS NULL;
CREATE INDEX idx_components_created_by ON components (created_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_components_updated_by ON components (updated_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_components_name_trgm ON components USING GIN (name gin_trgm_ops);
CREATE INDEX idx_components_code_trgm ON components USING GIN (code gin_trgm_ops);
CREATE INDEX idx_components_deleted_at ON components (deleted_at) WHERE deleted_at IS NOT NULL;

-- ─── Component <-> Tag junction (many-to-many) ───────────────────────────────
CREATE TABLE component_tags (
    component_id UUID NOT NULL,
    tag_id       UUID NOT NULL,
    PRIMARY KEY (component_id, tag_id)
);
CREATE INDEX idx_component_tags_tag_id ON component_tags (tag_id);

-- ─── Component <-> Page junction (many-to-many) ──────────────────────────────
CREATE TABLE component_pages (
    component_id UUID NOT NULL,
    page_id      UUID NOT NULL,
    PRIMARY KEY (component_id, page_id)
);
CREATE INDEX idx_component_pages_page_id ON component_pages (page_id);

-- ─── Translation Versions ────────────────────────────────────────────────────
CREATE TABLE translation_versions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    component_id  UUID NOT NULL,
    locale        TEXT NOT NULL,
    stage         VARCHAR(50) NOT NULL,                   -- draft | staging | production
    version       INTEGER NOT NULL DEFAULT 1,             -- 1, 2, 3, ... monotonic per (component, locale, stage)
    data          JSONB NOT NULL,
    source_locale VARCHAR(10) NOT NULL DEFAULT '',        -- empty for manual edits
    source_data   JSONB,                                  -- snapshot of source at AI translate time; nil for manual edits
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_by    UUID NOT NULL,
    updated_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);
-- Hot read path: latest active version per (component, locale, stage). Partial-on-deleted
-- keeps the index lean and the WHERE deleted_at IS NULL filter free.
CREATE INDEX idx_tv_lookup ON translation_versions (component_id, locale, stage, version DESC) WHERE deleted_at IS NULL;
-- Eliminates the read-MAX-then-insert race in services.SaveVersion. Concurrent
-- writers that pick the same nextVersion hit this index and retry.
CREATE UNIQUE INDEX idx_tv_unique_version ON translation_versions (component_id, locale, stage, version) WHERE deleted_at IS NULL;
CREATE INDEX idx_translation_versions_deleted_at ON translation_versions (deleted_at) WHERE deleted_at IS NOT NULL;

-- ─── Add Language Jobs ───────────────────────────────────────────────────────
CREATE TABLE add_language_jobs (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id       UUID NOT NULL,
    locale               VARCHAR(20) NOT NULL,
    auto_translate       BOOLEAN NOT NULL,
    status               VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending | running | completed | failed
    total_components     INTEGER NOT NULL DEFAULT 0,
    completed_components INTEGER NOT NULL DEFAULT 0,
    error_message        TEXT NOT NULL DEFAULT '',
    error_detail         TEXT NOT NULL DEFAULT '',
    claimed_by           VARCHAR(255) NOT NULL DEFAULT '',       -- K8s HOSTNAME of the claiming pod
    created_by           UUID NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ
);
CREATE INDEX idx_add_language_jobs_app_id ON add_language_jobs (application_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_add_language_jobs_status ON add_language_jobs (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_add_language_jobs_created_by ON add_language_jobs (created_by) WHERE deleted_at IS NULL;

-- ─── Translate Jobs ──────────────────────────────────────────────────────────
CREATE TABLE translate_jobs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL,
    component_id   UUID NOT NULL,
    job_type       VARCHAR(50) NOT NULL,                          -- auto_translate | backfill
    source_locale  VARCHAR(20) NOT NULL,
    target_locales TEXT[] NOT NULL DEFAULT '{}',
    status         VARCHAR(50) NOT NULL DEFAULT 'pending',
    error_message  TEXT NOT NULL DEFAULT '',
    error_detail   TEXT NOT NULL DEFAULT '',
    claimed_by     VARCHAR(255) NOT NULL DEFAULT '',
    created_by     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);
CREATE INDEX idx_translate_jobs_app_id ON translate_jobs (application_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_translate_jobs_component_id ON translate_jobs (component_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_translate_jobs_status ON translate_jobs (status) WHERE deleted_at IS NULL;
-- Idempotency on (component, source, first target locale, type). Catches
-- double-clicks that would otherwise queue duplicate OpenAI work.
CREATE UNIQUE INDEX idx_translate_jobs_dedupe
    ON translate_jobs (component_id, source_locale, (target_locales[1]), job_type)
    WHERE deleted_at IS NULL AND status IN ('pending', 'running');

-- ─── Audit Logs ──────────────────────────────────────────────────────────────
CREATE TABLE audit_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL,
    username      TEXT NOT NULL,
    action        VARCHAR(50) NOT NULL,                  -- CREATE | UPDATE | DELETE | DEPLOY | AUTO_TRANSLATE | ...
    resource_type VARCHAR(50) NOT NULL,                  -- application | component | translation | user | cms_item | ...
    resource_id   UUID NOT NULL,
    resource_code TEXT NOT NULL DEFAULT '',
    changes       JSONB,                                 -- {before: ..., after: ...}
    ip_address    VARCHAR(45) NOT NULL DEFAULT '',       -- IPv6-capable
    user_agent    TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_logs_user_id ON audit_logs (user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs (action);
CREATE INDEX idx_audit_logs_resource_type ON audit_logs (resource_type);
CREATE INDEX idx_audit_logs_resource_id ON audit_logs (resource_id);
CREATE INDEX idx_audit_logs_resource_code ON audit_logs (resource_code);
CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at DESC);

-- ─── CMS: Templates ──────────────────────────────────────────────────────────
CREATE TABLE cms_templates (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL,
    name           TEXT NOT NULL,
    code           VARCHAR(100) NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    created_by     UUID NOT NULL,
    updated_by     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_cms_template_app_code ON cms_templates (application_id, code) WHERE deleted_at IS NULL;

-- ─── CMS: Template Fields ────────────────────────────────────────────────────
CREATE TABLE cms_template_fields (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID NOT NULL,
    key         VARCHAR(100) NOT NULL,
    label       VARCHAR(255) NOT NULL,
    value_type  VARCHAR(50) NOT NULL,                   -- text | textarea | rich_text | json
    required    BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cms_template_fields_template_id ON cms_template_fields (template_id);

-- ─── CMS: Items ──────────────────────────────────────────────────────────────
-- identifier is case-folded to lowercase on create (see normalizeIdentifier
-- in handlers/cms_item_handler.go) so SDK lookups always match.
CREATE TABLE cms_items (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL,
    template_id    UUID NOT NULL,
    identifier     VARCHAR(100) NOT NULL,
    name           TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    created_by     UUID NOT NULL,
    updated_by     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_cms_item_app_identifier ON cms_items (application_id, identifier) WHERE deleted_at IS NULL;
CREATE INDEX idx_cms_items_template_id ON cms_items (template_id) WHERE deleted_at IS NULL;

-- ─── CMS: Localizations ──────────────────────────────────────────────────────
CREATE TABLE cms_localizations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cms_item_id   UUID NOT NULL,
    locale        VARCHAR(20) NOT NULL,
    stage         VARCHAR(50) NOT NULL,
    version       INTEGER NOT NULL DEFAULT 1,
    data          JSONB NOT NULL,
    source_locale VARCHAR(20) NOT NULL DEFAULT '',
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_by    UUID NOT NULL,
    updated_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);
-- Hot read path mirror of idx_tv_lookup.
CREATE INDEX idx_cms_loc_lookup ON cms_localizations (cms_item_id, locale, stage, version DESC) WHERE deleted_at IS NULL;
-- Race guard mirror of idx_tv_unique_version. services.SaveCmsLocalizationVersion
-- retries on collision.
CREATE UNIQUE INDEX idx_cms_loc_unique_version ON cms_localizations (cms_item_id, locale, stage, version) WHERE deleted_at IS NULL;

-- ─── CMS: Translate Jobs ─────────────────────────────────────────────────────
CREATE TABLE cms_translate_jobs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL,
    cms_item_id    UUID NOT NULL,
    source_locale  VARCHAR(20) NOT NULL,
    target_locale  VARCHAR(20) NOT NULL,
    stage          VARCHAR(50) NOT NULL,
    status         VARCHAR(50) NOT NULL DEFAULT 'pending',
    error_message  TEXT NOT NULL DEFAULT '',
    error_detail   TEXT NOT NULL DEFAULT '',
    claimed_by     VARCHAR(255) NOT NULL DEFAULT '',
    created_by     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);
CREATE INDEX idx_cms_translate_jobs_app_id ON cms_translate_jobs (application_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_cms_translate_jobs_item_id ON cms_translate_jobs (cms_item_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_cms_translate_jobs_status ON cms_translate_jobs (status) WHERE deleted_at IS NULL;
-- Idempotency mirror of idx_translate_jobs_dedupe (cms has single target_locale).
CREATE UNIQUE INDEX idx_cms_translate_jobs_dedupe
    ON cms_translate_jobs (cms_item_id, source_locale, target_locale, stage)
    WHERE deleted_at IS NULL AND status IN ('pending', 'running');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS cms_translate_jobs;
DROP TABLE IF EXISTS cms_localizations;
DROP TABLE IF EXISTS cms_items;
DROP TABLE IF EXISTS cms_template_fields;
DROP TABLE IF EXISTS cms_templates;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS translate_jobs;
DROP TABLE IF EXISTS add_language_jobs;
DROP TABLE IF EXISTS translation_versions;
DROP TABLE IF EXISTS component_pages;
DROP TABLE IF EXISTS component_tags;
DROP TABLE IF EXISTS components;
DROP TABLE IF EXISTS pages;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS application_locale_deploys;
DROP TABLE IF EXISTS application_api_keys;
DROP TABLE IF EXISTS applications;
DROP TABLE IF EXISTS users;

-- Extensions intentionally left in place (other services in the shared
-- Cloud SQL instance may rely on them).

-- +goose StatementEnd
