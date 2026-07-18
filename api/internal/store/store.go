// Package store persists tenant metadata and the audit log in Postgres.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors surfaced to handlers.
var (
	ErrNotFound = errors.New("tenant not found")
	ErrConflict = errors.New("tenant already exists")
)

// TenantRecord is the persisted metadata for a tenant.
type TenantRecord struct {
	Name        string
	DisplayName string
	Owner       string
	Spec        json.RawMessage
	CreatedAt   time.Time
}

// Store wraps a pgx pool with tenant/audit queries.
type Store struct {
	pool *pgxpool.Pool
}

// New creates a Store backed by the given pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const uniqueViolationCode = "23505"

// CreateTenant inserts a tenant row; returns ErrConflict on duplicate name.
func (s *Store) CreateTenant(ctx context.Context, t TenantRecord) error {
	spec := t.Spec
	if len(spec) == 0 {
		spec = json.RawMessage(`{}`)
	}
	const q = `INSERT INTO tenants (name, display_name, owner, spec) VALUES ($1, $2, $3, $4)`
	_, err := s.pool.Exec(ctx, q, t.Name, t.DisplayName, t.Owner, spec)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("insert tenant: %w", err)
	}
	return nil
}

// GetTenant fetches a live (non-deleted) tenant by name.
func (s *Store) GetTenant(ctx context.Context, name string) (*TenantRecord, error) {
	const q = `SELECT name, display_name, owner, spec, created_at
		FROM tenants WHERE name = $1 AND deleted_at IS NULL`
	var t TenantRecord
	err := s.pool.QueryRow(ctx, q, name).Scan(&t.Name, &t.DisplayName, &t.Owner, &t.Spec, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	return &t, nil
}

// ListTenants returns all live tenants.
func (s *Store) ListTenants(ctx context.Context) ([]TenantRecord, error) {
	const q = `SELECT name, display_name, owner, spec, created_at
		FROM tenants WHERE deleted_at IS NULL ORDER BY name`
	return s.queryTenants(ctx, q)
}

// ListTenantsByOwner returns live tenants owned by the given identity.
func (s *Store) ListTenantsByOwner(ctx context.Context, owner string) ([]TenantRecord, error) {
	const q = `SELECT name, display_name, owner, spec, created_at
		FROM tenants WHERE deleted_at IS NULL AND owner = $1 ORDER BY name`
	return s.queryTenants(ctx, q, owner)
}

func (s *Store) queryTenants(ctx context.Context, q string, args ...any) ([]TenantRecord, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []TenantRecord
	for rows.Next() {
		var t TenantRecord
		if err := rows.Scan(&t.Name, &t.DisplayName, &t.Owner, &t.Spec, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenants: %w", err)
	}
	return tenants, nil
}

// SoftDeleteTenant marks a tenant deleted; returns ErrNotFound if absent.
func (s *Store) SoftDeleteTenant(ctx context.Context, name string) error {
	const q = `UPDATE tenants SET deleted_at = now() WHERE name = $1 AND deleted_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, name)
	if err != nil {
		return fmt.Errorf("soft-delete tenant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// HardDeleteTenant removes a tenant row entirely (create rollback path).
func (s *Store) HardDeleteTenant(ctx context.Context, name string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM tenants WHERE name = $1`, name); err != nil {
		return fmt.Errorf("hard-delete tenant: %w", err)
	}
	return nil
}

// Audit appends an audit log entry.
func (s *Store) Audit(ctx context.Context, actor, action string, detail map[string]any) error {
	payload, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal audit detail: %w", err)
	}
	const q = `INSERT INTO audit_log (actor, action, detail) VALUES ($1, $2, $3)`
	if _, err := s.pool.Exec(ctx, q, actor, action, payload); err != nil {
		return fmt.Errorf("insert audit entry: %w", err)
	}
	return nil
}
