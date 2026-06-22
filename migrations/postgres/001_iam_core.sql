CREATE TABLE IF NOT EXISTS iam_resource_grant (
  id BIGSERIAL PRIMARY KEY,
  grant_id TEXT NOT NULL UNIQUE,
  app TEXT NOT NULL DEFAULT 'aihub',
  org_id TEXT,
  project_id TEXT,
  resource_type TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  object TEXT,
  subject_type TEXT NOT NULL,
  subject_id TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'viewer',
  actions JSONB NOT NULL DEFAULT '[]'::jsonb,
  effect TEXT NOT NULL DEFAULT 'allow',
  expires_at TIMESTAMPTZ,
  created_by TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_iam_grant_resource ON iam_resource_grant(app, resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_iam_grant_object ON iam_resource_grant(object);
CREATE INDEX IF NOT EXISTS idx_iam_grant_subject ON iam_resource_grant(subject_type, subject_id);
CREATE INDEX IF NOT EXISTS idx_iam_grant_scope ON iam_resource_grant(org_id, project_id);
CREATE INDEX IF NOT EXISTS idx_iam_grant_actions_gin ON iam_resource_grant USING GIN(actions);
