-- +goose Up
-- +goose StatementBegin

-- Initial schema for i18n-center (MySQL 8).
--
-- Canonical schema bootstrap. Run ONCE against a fresh database via
-- `i18n-center migrate up`. Subsequent schema changes go in new numbered
-- files (00002_..., ...) using goose conventions.
--
-- Partial-unique-on-deleted_at semantics are emulated with functional indexes:
-- IF(deleted_at IS NULL, code, NULL). NULL keys never conflict in MySQL
-- unique indexes, so soft-deleted rows freely coexist while live rows enforce
-- the intended uniqueness.

-- ─── Users ───────────────────────────────────────────────────────────────────
CREATE TABLE users (
    id            CHAR(36) NOT NULL PRIMARY KEY,
    username      VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role          VARCHAR(50) NOT NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at    DATETIME(6) NULL,
    UNIQUE KEY idx_users_username ((IF(deleted_at IS NULL, username, NULL))),
    KEY idx_users_deleted_at (deleted_at)
);

-- ─── Applications ────────────────────────────────────────────────────────────
CREATE TABLE applications (
    id                CHAR(36) NOT NULL PRIMARY KEY,
    name              VARCHAR(255) NOT NULL,
    code              VARCHAR(255) NOT NULL,
    description       TEXT NOT NULL,
    openai_key        TEXT NOT NULL,
    enabled_languages JSON NOT NULL,
    created_by        CHAR(36) NOT NULL,
    updated_by        CHAR(36) NOT NULL,
    created_at        DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at        DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at        DATETIME(6) NULL,
    UNIQUE KEY idx_applications_code ((IF(deleted_at IS NULL, code, NULL))),
    KEY idx_applications_created_by (created_by),
    KEY idx_applications_updated_by (updated_by),
    KEY idx_applications_deleted_at (deleted_at)
);

-- ─── Application API Keys ────────────────────────────────────────────────────
CREATE TABLE application_api_keys (
    id             CHAR(36) NOT NULL PRIMARY KEY,
    application_id CHAR(36) NOT NULL,
    key_hash       VARCHAR(64) NOT NULL,
    key_prefix     VARCHAR(20) NOT NULL,
    name           VARCHAR(255) NOT NULL DEFAULT '',
    created_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at     DATETIME(6) NULL,
    UNIQUE KEY idx_application_api_keys_hash ((IF(deleted_at IS NULL, key_hash, NULL))),
    KEY idx_application_api_keys_app_id (application_id),
    KEY idx_application_api_keys_prefix (key_prefix)
);

-- ─── Application Locale Deploys ──────────────────────────────────────────────
CREATE TABLE application_locale_deploys (
    id              CHAR(36) NOT NULL PRIMARY KEY,
    application_id  CHAR(36) NOT NULL,
    locale          VARCHAR(20) NOT NULL,
    stage_completed VARCHAR(50) NOT NULL DEFAULT 'draft',
    created_at      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at      DATETIME(6) NULL,
    UNIQUE KEY idx_app_locale (application_id, (IF(deleted_at IS NULL, locale, NULL)))
);

-- ─── Tags ────────────────────────────────────────────────────────────────────
CREATE TABLE tags (
    id             CHAR(36) NOT NULL PRIMARY KEY,
    application_id CHAR(36) NOT NULL,
    code           VARCHAR(100) NOT NULL,
    created_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at     DATETIME(6) NULL,
    UNIQUE KEY idx_tag_app_code (application_id, (IF(deleted_at IS NULL, code, NULL)))
);

-- ─── Pages ───────────────────────────────────────────────────────────────────
CREATE TABLE pages (
    id             CHAR(36) NOT NULL PRIMARY KEY,
    application_id CHAR(36) NOT NULL,
    code           VARCHAR(100) NOT NULL,
    created_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at     DATETIME(6) NULL,
    UNIQUE KEY idx_page_app_code (application_id, (IF(deleted_at IS NULL, code, NULL)))
);

-- ─── Components ──────────────────────────────────────────────────────────────
CREATE TABLE components (
    id             CHAR(36) NOT NULL PRIMARY KEY,
    application_id CHAR(36) NOT NULL,
    name           VARCHAR(255) NOT NULL,
    code           VARCHAR(255) NOT NULL,
    description    TEXT NOT NULL,
    key_contexts   JSON NULL,
    default_locale VARCHAR(20) NOT NULL,
    created_by     CHAR(36) NOT NULL,
    updated_by     CHAR(36) NOT NULL,
    created_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at     DATETIME(6) NULL,
    UNIQUE KEY idx_component_app_code (application_id, (IF(deleted_at IS NULL, code, NULL))),
    KEY idx_components_created_by (created_by),
    KEY idx_components_updated_by (updated_by),
    KEY idx_components_deleted_at (deleted_at)
);

-- ─── Component <-> Tag junction (many-to-many) ───────────────────────────────
CREATE TABLE component_tags (
    component_id CHAR(36) NOT NULL,
    tag_id       CHAR(36) NOT NULL,
    PRIMARY KEY (component_id, tag_id),
    KEY idx_component_tags_tag_id (tag_id)
);

-- ─── Component <-> Page junction (many-to-many) ──────────────────────────────
CREATE TABLE component_pages (
    component_id CHAR(36) NOT NULL,
    page_id      CHAR(36) NOT NULL,
    PRIMARY KEY (component_id, page_id),
    KEY idx_component_pages_page_id (page_id)
);

-- ─── Translation Versions ────────────────────────────────────────────────────
CREATE TABLE translation_versions (
    id            CHAR(36) NOT NULL PRIMARY KEY,
    component_id  CHAR(36) NOT NULL,
    locale        VARCHAR(20) NOT NULL,
    stage         VARCHAR(50) NOT NULL,
    version       INT NOT NULL DEFAULT 1,
    data          JSON NOT NULL,
    source_locale VARCHAR(20) NOT NULL DEFAULT '',
    source_data   JSON NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_by    CHAR(36) NOT NULL,
    updated_by    CHAR(36) NOT NULL,
    created_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at    DATETIME(6) NULL,
    UNIQUE KEY idx_tv_unique_version (
        component_id,
        locale,
        stage,
        (IF(deleted_at IS NULL, version, NULL))
    ),
    KEY idx_tv_lookup (component_id, locale, stage, version),
    KEY idx_translation_versions_deleted_at (deleted_at)
);

-- ─── Add Language Jobs ───────────────────────────────────────────────────────
CREATE TABLE add_language_jobs (
    id                   CHAR(36) NOT NULL PRIMARY KEY,
    application_id       CHAR(36) NOT NULL,
    locale               VARCHAR(20) NOT NULL,
    auto_translate      BOOLEAN NOT NULL,
    status               VARCHAR(50) NOT NULL DEFAULT 'pending',
    total_components     INT NOT NULL DEFAULT 0,
    completed_components INT NOT NULL DEFAULT 0,
    error_message        TEXT NOT NULL,
    error_detail         TEXT NOT NULL,
    claimed_by           VARCHAR(255) NOT NULL DEFAULT '',
    created_by           CHAR(36) NOT NULL,
    created_at           DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at           DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at           DATETIME(6) NULL,
    KEY idx_add_language_jobs_app_id (application_id),
    KEY idx_add_language_jobs_status (status),
    KEY idx_add_language_jobs_created_by (created_by)
);

-- ─── Translate Jobs ──────────────────────────────────────────────────────────
-- target_locales stored as JSON array. `first_target_locale` is a stored
-- generated column extracting the first element — participates in the dedupe
-- index (MySQL cannot subscript a JSON expression inside a functional index
-- reliably, so materialise it).
CREATE TABLE translate_jobs (
    id                   CHAR(36) NOT NULL PRIMARY KEY,
    application_id       CHAR(36) NOT NULL,
    component_id         CHAR(36) NOT NULL,
    job_type             VARCHAR(50) NOT NULL,
    source_locale        VARCHAR(20) NOT NULL,
    target_locales       JSON NOT NULL,
    first_target_locale  VARCHAR(20) GENERATED ALWAYS AS (
        JSON_UNQUOTE(JSON_EXTRACT(target_locales, '$[0]'))
    ) STORED,
    status               VARCHAR(50) NOT NULL DEFAULT 'pending',
    error_message        TEXT NOT NULL,
    error_detail         TEXT NOT NULL,
    claimed_by           VARCHAR(255) NOT NULL DEFAULT '',
    created_by           CHAR(36) NOT NULL,
    created_at           DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at           DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at           DATETIME(6) NULL,
    KEY idx_translate_jobs_app_id (application_id),
    KEY idx_translate_jobs_component_id (component_id),
    KEY idx_translate_jobs_status (status),
    -- Idempotency on (component, source, first target, type) among rows that
    -- are still pending/running and not soft-deleted. Rows outside that set
    -- get NULL in every functional slot and don't participate in uniqueness.
    UNIQUE KEY idx_translate_jobs_dedupe (
        component_id,
        source_locale,
        (IF(deleted_at IS NULL AND status IN ('pending','running'), first_target_locale, NULL)),
        (IF(deleted_at IS NULL AND status IN ('pending','running'), job_type, NULL))
    )
);

-- ─── Audit Logs ──────────────────────────────────────────────────────────────
CREATE TABLE audit_logs (
    id            CHAR(36) NOT NULL PRIMARY KEY,
    user_id       CHAR(36) NOT NULL,
    username      VARCHAR(255) NOT NULL,
    action        VARCHAR(50) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id   CHAR(36) NOT NULL,
    resource_code VARCHAR(255) NOT NULL DEFAULT '',
    changes       JSON NULL,
    ip_address    VARCHAR(45) NOT NULL DEFAULT '',
    user_agent    TEXT NOT NULL,
    created_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    KEY idx_audit_logs_user_id (user_id),
    KEY idx_audit_logs_action (action),
    KEY idx_audit_logs_resource_type (resource_type),
    KEY idx_audit_logs_resource_id (resource_id),
    KEY idx_audit_logs_resource_code (resource_code),
    KEY idx_audit_logs_created_at (created_at)
);

-- ─── CMS: Templates ──────────────────────────────────────────────────────────
CREATE TABLE cms_templates (
    id             CHAR(36) NOT NULL PRIMARY KEY,
    application_id CHAR(36) NOT NULL,
    name           VARCHAR(255) NOT NULL,
    code           VARCHAR(100) NOT NULL,
    description    TEXT NOT NULL,
    created_by     CHAR(36) NOT NULL,
    updated_by     CHAR(36) NOT NULL,
    created_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at     DATETIME(6) NULL,
    UNIQUE KEY idx_cms_template_app_code (application_id, (IF(deleted_at IS NULL, code, NULL)))
);

-- ─── CMS: Template Fields ────────────────────────────────────────────────────
CREATE TABLE cms_template_fields (
    id          CHAR(36) NOT NULL PRIMARY KEY,
    template_id CHAR(36) NOT NULL,
    `key`       VARCHAR(100) NOT NULL,
    label       VARCHAR(255) NOT NULL,
    value_type  VARCHAR(50) NOT NULL,
    required    BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    KEY idx_cms_template_fields_template_id (template_id)
);

-- ─── CMS: Items ──────────────────────────────────────────────────────────────
CREATE TABLE cms_items (
    id             CHAR(36) NOT NULL PRIMARY KEY,
    application_id CHAR(36) NOT NULL,
    template_id    CHAR(36) NOT NULL,
    identifier     VARCHAR(100) NOT NULL,
    name           VARCHAR(255) NOT NULL,
    description    TEXT NOT NULL,
    created_by     CHAR(36) NOT NULL,
    updated_by     CHAR(36) NOT NULL,
    created_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at     DATETIME(6) NULL,
    UNIQUE KEY idx_cms_item_app_identifier (application_id, (IF(deleted_at IS NULL, identifier, NULL))),
    KEY idx_cms_items_template_id (template_id)
);

-- ─── CMS: Localizations ──────────────────────────────────────────────────────
CREATE TABLE cms_localizations (
    id            CHAR(36) NOT NULL PRIMARY KEY,
    cms_item_id   CHAR(36) NOT NULL,
    locale        VARCHAR(20) NOT NULL,
    stage         VARCHAR(50) NOT NULL,
    version       INT NOT NULL DEFAULT 1,
    data          JSON NOT NULL,
    source_locale VARCHAR(20) NOT NULL DEFAULT '',
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_by    CHAR(36) NOT NULL,
    updated_by    CHAR(36) NOT NULL,
    created_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at    DATETIME(6) NULL,
    UNIQUE KEY idx_cms_loc_unique_version (
        cms_item_id,
        locale,
        stage,
        (IF(deleted_at IS NULL, version, NULL))
    ),
    KEY idx_cms_loc_lookup (cms_item_id, locale, stage, version)
);

-- ─── CMS: Translate Jobs ─────────────────────────────────────────────────────
CREATE TABLE cms_translate_jobs (
    id             CHAR(36) NOT NULL PRIMARY KEY,
    application_id CHAR(36) NOT NULL,
    cms_item_id    CHAR(36) NOT NULL,
    source_locale  VARCHAR(20) NOT NULL,
    target_locale  VARCHAR(20) NOT NULL,
    stage          VARCHAR(50) NOT NULL,
    status         VARCHAR(50) NOT NULL DEFAULT 'pending',
    error_message  TEXT NOT NULL,
    error_detail   TEXT NOT NULL,
    claimed_by     VARCHAR(255) NOT NULL DEFAULT '',
    created_by     CHAR(36) NOT NULL,
    created_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at     DATETIME(6) NULL,
    KEY idx_cms_translate_jobs_app_id (application_id),
    KEY idx_cms_translate_jobs_item_id (cms_item_id),
    KEY idx_cms_translate_jobs_status (status),
    UNIQUE KEY idx_cms_translate_jobs_dedupe (
        cms_item_id,
        source_locale,
        target_locale,
        (IF(deleted_at IS NULL AND status IN ('pending','running'), stage, NULL))
    )
);

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

-- +goose StatementEnd
