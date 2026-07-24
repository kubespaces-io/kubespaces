// This file adds the tenancy domain to the store: users (a projection of
// Keycloak identities), organizations, projects, and their memberships/roles.
// Tenants are linked to a project/org elsewhere (see tenant queries).
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Membership roles. Organizations use owner/admin/member; projects use
// maintainer/member. Kept as strings to match the schema and the old API.
const (
	RoleOwner      = "owner"
	RoleAdmin      = "admin"
	RoleMember     = "member"
	RoleMaintainer = "maintainer"
)

// User is a thin projection of a Keycloak identity, provisioned on first login.
type User struct {
	ID          string
	Subject     string
	Email       string
	DisplayName string
	IsAdmin     bool
	CreatedAt   time.Time
}

// Organization groups projects and members.
type Organization struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// OrganizationWithRole is an org plus the querying user's membership.
type OrganizationWithRole struct {
	Organization
	Role      string
	IsDefault bool
}

// Member is a user's membership in an org or project (with their profile).
type Member struct {
	UserID      string
	Email       string
	DisplayName string
	Role        string
	IsDefault   bool
}

// Project belongs to an organization and carries tenancy quotas.
type Project struct {
	ID             string
	OrganizationID string
	Name           string
	Description    string
	MaxTenants     int
	MaxCompute     int
	MaxMemoryGB    int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}

// ---------- Users ----------

// UpsertUser provisions or refreshes a user from OIDC claims and returns it.
// is_admin tracks the token's admin claim so Keycloak stays the source of
// truth; the admin API can still flip it via SetUserAdmin.
func (s *Store) UpsertUser(ctx context.Context, subject, email, displayName string, isAdmin bool) (*User, error) {
	const q = `
		INSERT INTO users (subject, email, display_name, is_admin)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (subject) DO UPDATE
		  SET email = EXCLUDED.email,
		      display_name = EXCLUDED.display_name,
		      is_admin = EXCLUDED.is_admin,
		      updated_at = now()
		RETURNING id::text, subject, email, display_name, is_admin, created_at`
	return scanUser(s.pool.QueryRow(ctx, q, subject, email, displayName, isAdmin))
}

// GetUserBySubject looks up a user by OIDC subject.
func (s *Store) GetUserBySubject(ctx context.Context, subject string) (*User, error) {
	const q = `SELECT id::text, subject, email, display_name, is_admin, created_at
		FROM users WHERE subject = $1`
	return scanUser(s.pool.QueryRow(ctx, q, subject))
}

// GetUserByEmail looks up a user by email (used to resolve invitations).
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	const q = `SELECT id::text, subject, email, display_name, is_admin, created_at
		FROM users WHERE email = $1`
	return scanUser(s.pool.QueryRow(ctx, q, email))
}

// SetUserAdmin toggles a user's admin flag (admin API).
func (s *Store) SetUserAdmin(ctx context.Context, userID string, admin bool) error {
	ct, err := s.pool.Exec(ctx, `UPDATE users SET is_admin = $2, updated_at = now() WHERE id = $1`, userID, admin)
	if err != nil {
		return fmt.Errorf("set user admin: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListUsers returns all users (admin API).
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	const q = `SELECT id::text, subject, email, display_name, is_admin, created_at
		FROM users ORDER BY email`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanUser(row scanner) (*User, error) {
	var u User
	if err := row.Scan(&u.ID, &u.Subject, &u.Email, &u.DisplayName, &u.IsAdmin, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

// ---------- Organizations ----------

// CreateOrganization creates an org and makes the caller its owner. If the
// caller has no default org yet, this one becomes the default.
func (s *Store) CreateOrganization(ctx context.Context, name, description, ownerUserID string) (*Organization, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var org Organization
	err = tx.QueryRow(ctx,
		`INSERT INTO organizations (name, description) VALUES ($1, $2)
		 RETURNING id::text, name, description, created_at, updated_at`,
		name, description,
	).Scan(&org.ID, &org.Name, &org.Description, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert organization: %w", err)
	}

	var hasDefault bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM user_organizations WHERE user_id = $1 AND is_default)`,
		ownerUserID,
	).Scan(&hasDefault); err != nil {
		return nil, fmt.Errorf("check default org: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO user_organizations (user_id, organization_id, role, is_default)
		 VALUES ($1, $2, $3, $4)`,
		ownerUserID, org.ID, RoleOwner, !hasDefault,
	); err != nil {
		return nil, fmt.Errorf("insert owner membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &org, nil
}

// GetOrganization fetches an org by id.
func (s *Store) GetOrganization(ctx context.Context, id string) (*Organization, error) {
	const q = `SELECT id::text, name, description, created_at, updated_at
		FROM organizations WHERE id = $1`
	return scanOrg(s.pool.QueryRow(ctx, q, id))
}

// GetOrganizationByName fetches an org by (non-unique) name — first match.
func (s *Store) GetOrganizationByName(ctx context.Context, name string) (*Organization, error) {
	const q = `SELECT id::text, name, description, created_at, updated_at
		FROM organizations WHERE name = $1 ORDER BY created_at LIMIT 1`
	return scanOrg(s.pool.QueryRow(ctx, q, name))
}

// ListOrganizationsForUser returns the orgs a user belongs to, with their role.
func (s *Store) ListOrganizationsForUser(ctx context.Context, userID string) ([]OrganizationWithRole, error) {
	const q = `
		SELECT o.id::text, o.name, o.description, o.created_at, o.updated_at, uo.role, uo.is_default
		FROM organizations o
		JOIN user_organizations uo ON uo.organization_id = o.id
		WHERE uo.user_id = $1
		ORDER BY o.name`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list orgs for user: %w", err)
	}
	defer rows.Close()
	var out []OrganizationWithRole
	for rows.Next() {
		var o OrganizationWithRole
		if err := rows.Scan(&o.ID, &o.Name, &o.Description, &o.CreatedAt, &o.UpdatedAt, &o.Role, &o.IsDefault); err != nil {
			return nil, fmt.Errorf("scan org: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// UpdateOrganization updates an org's metadata.
func (s *Store) UpdateOrganization(ctx context.Context, id, name, description string) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE organizations SET name = $2, description = $3, updated_at = now() WHERE id = $1`,
		id, name, description)
	if err != nil {
		return fmt.Errorf("update organization: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteOrganization removes an org (cascades to memberships/projects).
func (s *Store) DeleteOrganization(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete organization: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanOrg(row scanner) (*Organization, error) {
	var o Organization
	if err := row.Scan(&o.ID, &o.Name, &o.Description, &o.CreatedAt, &o.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan organization: %w", err)
	}
	return &o, nil
}

// ---------- Organization membership ----------

// AddOrgMember adds (or, on conflict, re-roles) a user in an org.
func (s *Store) AddOrgMember(ctx context.Context, orgID, userID, role string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO user_organizations (user_id, organization_id, role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, organization_id) DO UPDATE SET role = EXCLUDED.role`,
		userID, orgID, role)
	if err != nil {
		return fmt.Errorf("add org member: %w", err)
	}
	return nil
}

// RemoveOrgMember removes a user from an org.
func (s *Store) RemoveOrgMember(ctx context.Context, orgID, userID string) error {
	ct, err := s.pool.Exec(ctx,
		`DELETE FROM user_organizations WHERE organization_id = $1 AND user_id = $2`, orgID, userID)
	if err != nil {
		return fmt.Errorf("remove org member: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateOrgMemberRole changes a member's role.
func (s *Store) UpdateOrgMemberRole(ctx context.Context, orgID, userID, role string) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE user_organizations SET role = $3 WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID, role)
	if err != nil {
		return fmt.Errorf("update org member role: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetOrgMemberRole returns a user's role in an org, or ErrNotFound if not a member.
func (s *Store) GetOrgMemberRole(ctx context.Context, orgID, userID string) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx,
		`SELECT role FROM user_organizations WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get org member role: %w", err)
	}
	return role, nil
}

// ListOrgMembers lists an org's members with their profiles.
func (s *Store) ListOrgMembers(ctx context.Context, orgID string) ([]Member, error) {
	const q = `
		SELECT u.id::text, u.email, u.display_name, uo.role, uo.is_default
		FROM user_organizations uo
		JOIN users u ON u.id = uo.user_id
		WHERE uo.organization_id = $1
		ORDER BY u.email`
	return scanMembers(ctx, s, q, orgID)
}

// SetDefaultOrg marks one org as the user's default (unsets the others).
func (s *Store) SetDefaultOrg(ctx context.Context, userID, orgID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`UPDATE user_organizations SET is_default = false WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear defaults: %w", err)
	}
	ct, err := tx.Exec(ctx,
		`UPDATE user_organizations SET is_default = true WHERE user_id = $1 AND organization_id = $2`,
		userID, orgID)
	if err != nil {
		return fmt.Errorf("set default: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

// GetDefaultOrg returns the user's default org, or ErrNotFound.
func (s *Store) GetDefaultOrg(ctx context.Context, userID string) (*Organization, error) {
	const q = `
		SELECT o.id::text, o.name, o.description, o.created_at, o.updated_at
		FROM organizations o
		JOIN user_organizations uo ON uo.organization_id = o.id
		WHERE uo.user_id = $1 AND uo.is_default
		LIMIT 1`
	return scanOrg(s.pool.QueryRow(ctx, q, userID))
}

// ---------- Projects ----------

// CreateProject creates a project under an org and makes the caller a
// maintainer. Returns ErrConflict if the name is taken within the org.
func (s *Store) CreateProject(ctx context.Context, p Project, creatorUserID string) (*Project, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var out Project
	err = tx.QueryRow(ctx, `
		INSERT INTO projects (organization_id, name, description, max_tenants, max_compute, max_memory_gb)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text, organization_id::text, name, description, max_tenants, max_compute, max_memory_gb, created_at, updated_at`,
		p.OrganizationID, p.Name, p.Description, p.MaxTenants, p.MaxCompute, p.MaxMemoryGB,
	).Scan(&out.ID, &out.OrganizationID, &out.Name, &out.Description, &out.MaxTenants, &out.MaxCompute, &out.MaxMemoryGB, &out.CreatedAt, &out.UpdatedAt)
	if isUniqueViolation(err) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO project_users (project_id, user_id, role) VALUES ($1, $2, $3)`,
		out.ID, creatorUserID, RoleMaintainer,
	); err != nil {
		return nil, fmt.Errorf("insert maintainer membership: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &out, nil
}

// GetProject fetches a project by id.
func (s *Store) GetProject(ctx context.Context, id string) (*Project, error) {
	const q = `SELECT id::text, organization_id::text, name, description, max_tenants, max_compute, max_memory_gb, created_at, updated_at
		FROM projects WHERE id = $1`
	return scanProject(s.pool.QueryRow(ctx, q, id))
}

// ListProjectsForOrg lists all projects in an org.
func (s *Store) ListProjectsForOrg(ctx context.Context, orgID string) ([]Project, error) {
	const q = `SELECT id::text, organization_id::text, name, description, max_tenants, max_compute, max_memory_gb, created_at, updated_at
		FROM projects WHERE organization_id = $1 ORDER BY name`
	return scanProjects(ctx, s, q, orgID)
}

// ListProjectsForUser lists projects a user is a member of, across orgs.
func (s *Store) ListProjectsForUser(ctx context.Context, userID string) ([]Project, error) {
	const q = `
		SELECT p.id::text, p.organization_id::text, p.name, p.description, p.max_tenants, p.max_compute, p.max_memory_gb, p.created_at, p.updated_at
		FROM projects p
		JOIN project_users pu ON pu.project_id = p.id
		WHERE pu.user_id = $1
		ORDER BY p.name`
	return scanProjects(ctx, s, q, userID)
}

// UpdateProject updates a project's name/description.
func (s *Store) UpdateProject(ctx context.Context, id, name, description string) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE projects SET name = $2, description = $3, updated_at = now() WHERE id = $1`,
		id, name, description)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateProjectQuotas adjusts a project's tenancy quotas.
func (s *Store) UpdateProjectQuotas(ctx context.Context, id string, maxTenants, maxCompute, maxMemoryGB int) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE projects SET max_tenants = $2, max_compute = $3, max_memory_gb = $4, updated_at = now() WHERE id = $1`,
		id, maxTenants, maxCompute, maxMemoryGB)
	if err != nil {
		return fmt.Errorf("update project quotas: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteProject removes a project (cascades to memberships).
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanProject(row scanner) (*Project, error) {
	var p Project
	if err := row.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Description, &p.MaxTenants, &p.MaxCompute, &p.MaxMemoryGB, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan project: %w", err)
	}
	return &p, nil
}

func scanProjects(ctx context.Context, s *Store, q string, args ...any) ([]Project, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// ---------- Project membership ----------

// AddProjectMember adds (or re-roles) a user in a project.
func (s *Store) AddProjectMember(ctx context.Context, projectID, userID, role string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO project_users (project_id, user_id, role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		projectID, userID, role)
	if err != nil {
		return fmt.Errorf("add project member: %w", err)
	}
	return nil
}

// RemoveProjectMember removes a user from a project.
func (s *Store) RemoveProjectMember(ctx context.Context, projectID, userID string) error {
	ct, err := s.pool.Exec(ctx,
		`DELETE FROM project_users WHERE project_id = $1 AND user_id = $2`, projectID, userID)
	if err != nil {
		return fmt.Errorf("remove project member: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateProjectMemberRole changes a member's role in a project.
func (s *Store) UpdateProjectMemberRole(ctx context.Context, projectID, userID, role string) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE project_users SET role = $3 WHERE project_id = $1 AND user_id = $2`,
		projectID, userID, role)
	if err != nil {
		return fmt.Errorf("update project member role: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetProjectMemberRole returns a user's role in a project, or ErrNotFound.
func (s *Store) GetProjectMemberRole(ctx context.Context, projectID, userID string) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx,
		`SELECT role FROM project_users WHERE project_id = $1 AND user_id = $2`,
		projectID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get project member role: %w", err)
	}
	return role, nil
}

// ListProjectMembers lists a project's members with their profiles.
func (s *Store) ListProjectMembers(ctx context.Context, projectID string) ([]Member, error) {
	const q = `
		SELECT u.id::text, u.email, u.display_name, pu.role, false
		FROM project_users pu
		JOIN users u ON u.id = pu.user_id
		WHERE pu.project_id = $1
		ORDER BY u.email`
	return scanMembers(ctx, s, q, projectID)
}

func scanMembers(ctx context.Context, s *Store, q string, args ...any) ([]Member, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Email, &m.DisplayName, &m.Role, &m.IsDefault); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
