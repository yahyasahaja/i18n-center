package audit

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/lapakgaming/i18n-center/repository"
)

const (
	queryInsert = `
		INSERT INTO audit_logs (
			id, user_id, username, action, resource_type, resource_id,
			resource_code, changes, ip_address, user_agent, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
	`

	queryListBase = `
		SELECT id, user_id, username, action, resource_type, resource_id,
		       resource_code, changes, ip_address, user_agent, created_at
		FROM audit_logs
	`

	queryCountBase = `SELECT COUNT(*) FROM audit_logs`

	queryHistoryBase = `
		SELECT id, user_id, username, action, resource_type, resource_id,
		       resource_code, changes, ip_address, user_agent, created_at
		FROM audit_logs
		WHERE resource_type = $1 AND resource_id = $2
		ORDER BY created_at DESC
	`
)

type Impl struct{}

func New() Repository { return &Impl{} }

func (r *Impl) Insert(ctx context.Context, q repository.Queryer, l *Log) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	_, err := q.ExecContext(ctx, queryInsert,
		l.ID, l.UserID, l.Username, l.Action, l.ResourceType, l.ResourceID,
		l.ResourceCode, l.Changes, l.IPAddress, l.UserAgent,
	)
	return err
}

func (r *Impl) List(ctx context.Context, q repository.Queryer, f ListFilter) ([]Log, int, error) {
	// Build the WHERE + LIMIT/OFFSET tail dynamically. Static prefix stays in
	// the const so query plans stay stable for common filter combinations.
	sb := strings.Builder{}
	cb := strings.Builder{}
	sb.WriteString(queryListBase)
	cb.WriteString(queryCountBase)
	args := []any{}
	first := true
	i := 1
	add := func(clause string, val any) {
		if first {
			sb.WriteString(" WHERE ")
			cb.WriteString(" WHERE ")
			first = false
		} else {
			sb.WriteString(" AND ")
			cb.WriteString(" AND ")
		}
		fmt.Fprintf(&sb, clause, i)
		fmt.Fprintf(&cb, clause, i)
		args = append(args, val)
		i++
	}
	if f.UserID != uuid.Nil {
		add("user_id = $%d", f.UserID)
	}
	if f.ResourceType != "" {
		add("resource_type = $%d", f.ResourceType)
	}
	if f.ResourceID != uuid.Nil {
		add("resource_id = $%d", f.ResourceID)
	}
	if f.Action != "" {
		add("action = $%d", f.Action)
	}

	countArgs := append([]any(nil), args...)
	var total int
	if err := q.GetContext(ctx, &total, cb.String(), countArgs...); err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	fmt.Fprintf(&sb, " ORDER BY created_at DESC LIMIT $%d OFFSET $%d", i, i+1)
	args = append(args, limit, f.Offset)

	rows := []Log{}
	if err := q.SelectContext(ctx, &rows, sb.String(), args...); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *Impl) History(ctx context.Context, q repository.Queryer, resourceType string, resourceID uuid.UUID, limit int) ([]Log, error) {
	query := queryHistoryBase
	args := []any{resourceType, resourceID}
	if limit > 0 {
		query += " LIMIT $3"
		args = append(args, limit)
	}
	rows := []Log{}
	if err := q.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}
