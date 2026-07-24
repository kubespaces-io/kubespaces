-- Tenancy model: organizations → projects → tenants, with users (a thin
-- projection of Keycloak identities), memberships + roles, and invitations.
-- Ported from the pre-open-source login-api, adapted so Keycloak owns identity
-- (no passwords/verification here) and tenants remain backed by Tenant CRs.

-- Users: JIT-provisioned from the OIDC token on first authenticated request.
-- `subject` is the Keycloak `sub` claim; identity/credentials live in Keycloak.
CREATE TABLE users (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  subject      TEXT NOT NULL UNIQUE,
  email        TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  is_admin     BOOLEAN NOT NULL DEFAULT false,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_users_email ON users (email);

CREATE TABLE organizations (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_organizations_name ON organizations (name);

-- Membership + role of a user in an organization. Roles: owner | admin | member.
CREATE TABLE user_organizations (
  user_id         UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  role            TEXT NOT NULL DEFAULT 'member',
  is_default      BOOLEAN NOT NULL DEFAULT false,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, organization_id)
);
CREATE INDEX idx_user_organizations_org ON user_organizations (organization_id);
CREATE INDEX idx_user_organizations_role ON user_organizations (organization_id, role);

CREATE TABLE projects (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  name            TEXT NOT NULL,
  description     TEXT NOT NULL DEFAULT '',
  max_tenants     INT NOT NULL DEFAULT 10,
  max_compute     INT NOT NULL DEFAULT 100,
  max_memory_gb   INT NOT NULL DEFAULT 100,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (organization_id, name)
);
CREATE INDEX idx_projects_org ON projects (organization_id);

-- Membership + role of a user in a project. Roles: maintainer | member.
CREATE TABLE project_users (
  project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
  user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  role       TEXT NOT NULL DEFAULT 'member',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (project_id, user_id)
);
CREATE INDEX idx_project_users_user ON project_users (user_id);
CREATE INDEX idx_project_users_role ON project_users (project_id, role);

-- Email-addressed invitations to join an org or a project. Status:
-- pending | accepted | declined | expired.
CREATE TABLE organization_invitations (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  inviter_user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  invitee_email   TEXT NOT NULL,
  role            TEXT NOT NULL DEFAULT 'member',
  status          TEXT NOT NULL DEFAULT 'pending',
  expires_at      TIMESTAMPTZ NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_org_invitations_org ON organization_invitations (organization_id);
CREATE INDEX idx_org_invitations_email ON organization_invitations (invitee_email);
CREATE INDEX idx_org_invitations_status ON organization_invitations (status);

CREATE TABLE project_invitations (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id      UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
  organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  inviter_user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  invitee_email   TEXT NOT NULL,
  role            TEXT NOT NULL DEFAULT 'member',
  status          TEXT NOT NULL DEFAULT 'pending',
  expires_at      TIMESTAMPTZ NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_project_invitations_project ON project_invitations (project_id);
CREATE INDEX idx_project_invitations_email ON project_invitations (invitee_email);
CREATE INDEX idx_project_invitations_status ON project_invitations (status);

-- Link existing tenants to a project/org. Nullable so tenants created before
-- the tenancy model (or by the operator directly) keep working.
ALTER TABLE tenants ADD COLUMN organization_id UUID REFERENCES organizations (id) ON DELETE SET NULL;
ALTER TABLE tenants ADD COLUMN project_id      UUID REFERENCES projects (id) ON DELETE SET NULL;
CREATE INDEX idx_tenants_organization ON tenants (organization_id);
CREATE INDEX idx_tenants_project ON tenants (project_id);
