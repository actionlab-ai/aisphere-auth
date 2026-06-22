-- AI Sphere IAM core schema. Casdoor remains the IdP; these tables store
-- platform business identity scopes and resource-level collaboration grants.

CREATE TABLE IF NOT EXISTS iam_org (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  org_id VARCHAR(128) NOT NULL,
  parent_org_id VARCHAR(128) DEFAULT NULL,
  display_name VARCHAR(255),
  description TEXT,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  metadata JSON,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_org_id (org_id),
  KEY idx_parent_org (parent_org_id),
  KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS iam_project (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  project_id VARCHAR(128) NOT NULL,
  org_id VARCHAR(128) NOT NULL,
  display_name VARCHAR(255),
  description TEXT,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  metadata JSON,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_project_id (project_id),
  KEY idx_org (org_id),
  KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS iam_group (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  group_id VARCHAR(191) NOT NULL,
  org_id VARCHAR(128) DEFAULT NULL,
  project_id VARCHAR(128) DEFAULT NULL,
  parent_group_id VARCHAR(191) DEFAULT NULL,
  display_name VARCHAR(255),
  description TEXT,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  metadata JSON,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_group_id (group_id),
  KEY idx_org_project (org_id, project_id),
  KEY idx_parent_group (parent_group_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS iam_membership (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  subject_type VARCHAR(32) NOT NULL DEFAULT 'user',
  subject_id VARCHAR(191) NOT NULL,
  scope_type VARCHAR(32) NOT NULL,
  scope_id VARCHAR(191) NOT NULL,
  roles JSON,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_by VARCHAR(191) DEFAULT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_member_scope (subject_type, subject_id, scope_type, scope_id),
  KEY idx_subject (subject_type, subject_id),
  KEY idx_scope (scope_type, scope_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS iam_role_binding (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  binding_id VARCHAR(128) NOT NULL,
  subject_type VARCHAR(32) NOT NULL DEFAULT 'user',
  subject_id VARCHAR(191) NOT NULL,
  role_code VARCHAR(128) NOT NULL,
  scope_type VARCHAR(32) NOT NULL DEFAULT 'global',
  scope_id VARCHAR(191) NOT NULL DEFAULT '*',
  app_code VARCHAR(64) DEFAULT NULL,
  effect VARCHAR(16) NOT NULL DEFAULT 'allow',
  created_by VARCHAR(191) DEFAULT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_binding_id (binding_id),
  UNIQUE KEY uk_subject_role_scope (subject_type, subject_id, role_code, scope_type, scope_id, app_code),
  KEY idx_subject (subject_type, subject_id),
  KEY idx_scope (scope_type, scope_id),
  KEY idx_app_role (app_code, role_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS iam_resource_grant (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  grant_id VARCHAR(128) NOT NULL,
  app VARCHAR(64) NOT NULL DEFAULT 'aihub',
  org_id VARCHAR(128) DEFAULT NULL,
  project_id VARCHAR(128) DEFAULT NULL,
  resource_type VARCHAR(64) NOT NULL,
  resource_id VARCHAR(191) NOT NULL,
  object VARCHAR(512) DEFAULT NULL,
  subject_type VARCHAR(32) NOT NULL,
  subject_id VARCHAR(191) NOT NULL,
  role VARCHAR(64) NOT NULL DEFAULT 'viewer',
  actions JSON,
  effect VARCHAR(16) NOT NULL DEFAULT 'allow',
  expires_at DATETIME NULL,
  created_by VARCHAR(191) DEFAULT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_grant_id (grant_id),
  KEY idx_resource (app, resource_type, resource_id),
  KEY idx_object (object(191)),
  KEY idx_subject (subject_type, subject_id),
  KEY idx_scope (org_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
