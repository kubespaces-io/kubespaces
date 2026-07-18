CREATE TABLE tenants (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name         TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL DEFAULT '',
  owner        TEXT NOT NULL,             -- OIDC subject or email
  spec         JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at   TIMESTAMPTZ
);
CREATE TABLE audit_log (
  id     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  actor  TEXT NOT NULL,
  action TEXT NOT NULL,                   -- tenant.create | tenant.delete | ...
  detail JSONB NOT NULL DEFAULT '{}'::jsonb
);
