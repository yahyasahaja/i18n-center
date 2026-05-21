// Package repository holds the data access layer for i18n-center. Each
// resource lives in its own subpackage (e.g. repository/component) with a
// Repository interface, query constants at the top of repository_impl.go,
// and a concrete *sqlx.DB-backed implementation.
//
// Design rules (enforced by convention, not the compiler):
//
//  1. **Raw SQL only.** No ORM. All queries are written by hand.
//  2. **Queries are package-level `const`s** at the top of each impl file.
//     Conditional bits (search filters, dynamic ORDER BY) are appended inside
//     the method, never embedded in a string-concat soup that hides the
//     parameterised version.
//  3. **Soft deletes are explicit.** Every read query includes
//     `WHERE deleted_at IS NULL`. Every write that "deletes" sets
//     `deleted_at = NOW()` instead of DELETE FROM. Hard deletes are reserved
//     for the retention job.
//  4. **Methods take `context.Context`** as the first argument.
//  5. **Repositories work over a `Queryer` interface** (see below) so the same
//     method body works against `*sqlx.DB` or `*sqlx.Tx`. Use `WithTx` for
//     multi-statement consistency.
//  6. **Domain sentinels for "not found"** — use ErrNotFound, ErrConflict, etc.
//     so callers can switch on error kind without parsing SQL state codes.
package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// ─── Sentinel errors ─────────────────────────────────────────────────────────

// ErrNotFound is returned when a query expecting a single row finds none.
// Callers SHOULD `errors.Is(err, ErrNotFound)` rather than checking
// `sql.ErrNoRows` directly — keeps the driver coupling out of the handlers.
var ErrNotFound = errors.New("repository: not found")

// ErrConflict is returned for unique-key violations that the caller is
// expected to handle (e.g. version race on insert). Wraps the underlying
// driver error so the message is preserved.
var ErrConflict = errors.New("repository: conflict")

// ─── Queryer: the interface satisfied by both *sqlx.DB and *sqlx.Tx ──────────

// Queryer is the subset of sqlx that every repository method needs. Both
// *sqlx.DB and *sqlx.Tx satisfy it, so the same method body works inside or
// outside a transaction.
//
// We deliberately keep this small. Methods that need PreparedX or Stmt should
// add to the interface explicitly so the contract stays visible.
type Queryer interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryxContext(ctx context.Context, query string, args ...any) (*sqlx.Rows, error)
	QueryRowxContext(ctx context.Context, query string, args ...any) *sqlx.Row
	NamedExecContext(ctx context.Context, query string, arg any) (sql.Result, error)
	Rebind(query string) string
}

// Compile-time assertions that *sqlx.DB and *sqlx.Tx satisfy Queryer.
var (
	_ Queryer = (*sqlx.DB)(nil)
	_ Queryer = (*sqlx.Tx)(nil)
)

// ─── WithTx: transactional helper ───────────────────────────────────────────

// WithTx runs fn inside a transaction. Commits on nil-error, rolls back on
// non-nil error or panic. Use this in usecases that need atomicity across
// multiple repository calls (e.g. deploy-locale-for-all-components).
//
// Pattern in a usecase:
//
//	return WithTx(ctx, u.db, func(tx Queryer) error {
//	    if err := u.componentRepo.UpdateTx(ctx, tx, c); err != nil { return err }
//	    if err := u.tagRepo.AttachTx(ctx, tx, c.ID, tagIDs); err != nil { return err }
//	    return nil
//	})
//
// Each repository method that wants to participate in an outer transaction
// takes a Queryer arg. Callers passing the global *sqlx.DB get autocommit;
// callers inside WithTx get the *sqlx.Tx.
func WithTx(ctx context.Context, db *sqlx.DB, fn func(tx Queryer) error) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}

// ─── JSONB: custom type for Postgres jsonb columns ───────────────────────────

// JSONB is a Go map serialised to/from Postgres jsonb. database/sql doesn't
// know jsonb natively, so every model with a jsonb column embeds this and
// gets Scan/Value for free.
//
// Nil maps round-trip as SQL NULL.
type JSONB map[string]any

// Value implements driver.Valuer.
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements sql.Scanner.
func (j *JSONB) Scan(src any) error {
	if src == nil {
		*j = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("JSONB.Scan: unsupported source type %T", src)
	}
	if len(b) == 0 {
		*j = nil
		return nil
	}
	return json.Unmarshal(b, j)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// IsUniqueViolation reports whether err is a Postgres SQLSTATE 23505
// (unique_violation). Message-matched so we don't take a hard pgconn dep.
//
// Callers retry on true (e.g. version-race in SaveVersion) or wrap as
// ErrConflict before bubbling up.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "SQLSTATE 23505") ||
		contains(msg, "duplicate key value") ||
		contains(msg, "unique constraint")
}

// contains is a tiny strings.Contains shim to keep this file dependency-free
// (it's already a pretty central package).
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
