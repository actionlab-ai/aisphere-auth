package iam

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresService struct {
	pool *pgxpool.Pool
}

func NewPostgresService(cfg config.DatabaseConfig) (*PostgresService, error) {
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("database.dsn is required for postgres IAM provider")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		pcfg.MaxConns = int32(cfg.MaxOpenConns)
	} else if pcfg.MaxConns == 0 {
		pcfg.MaxConns = 30
	}
	pcfg.MaxConnLifetime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	svc := &PostgresService{pool: pool}
	if cfg.AutoMigrate {
		if err := svc.AutoMigrate(ctx); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return svc, nil
}

func (s *PostgresService) Close() error { s.pool.Close(); return nil }

func (s *PostgresService) AutoMigrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS iam_org (
          id BIGSERIAL PRIMARY KEY,
          org_id TEXT NOT NULL UNIQUE,
          parent_org_id TEXT,
          display_name TEXT,
          description TEXT,
          status TEXT NOT NULL DEFAULT 'active',
          metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
          created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
          updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
		`CREATE INDEX IF NOT EXISTS idx_iam_org_parent ON iam_org(parent_org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_org_status ON iam_org(status)`,
		`CREATE TABLE IF NOT EXISTS iam_project (
          id BIGSERIAL PRIMARY KEY,
          project_id TEXT NOT NULL UNIQUE,
          org_id TEXT NOT NULL,
          display_name TEXT,
          description TEXT,
          status TEXT NOT NULL DEFAULT 'active',
          metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
          created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
          updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
		`CREATE INDEX IF NOT EXISTS idx_iam_project_org ON iam_project(org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_project_status ON iam_project(status)`,
		`CREATE TABLE IF NOT EXISTS iam_group (
          id BIGSERIAL PRIMARY KEY,
          group_id TEXT NOT NULL UNIQUE,
          org_id TEXT,
          project_id TEXT,
          parent_group_id TEXT,
          display_name TEXT,
          description TEXT,
          status TEXT NOT NULL DEFAULT 'active',
          metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
          created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
          updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
		`CREATE INDEX IF NOT EXISTS idx_iam_group_scope ON iam_group(org_id, project_id)`,
		`CREATE TABLE IF NOT EXISTS iam_membership (
          id BIGSERIAL PRIMARY KEY,
          subject_type TEXT NOT NULL DEFAULT 'user',
          subject_id TEXT NOT NULL,
          scope_type TEXT NOT NULL,
          scope_id TEXT NOT NULL,
          roles JSONB NOT NULL DEFAULT '[]'::jsonb,
          status TEXT NOT NULL DEFAULT 'active',
          created_by TEXT,
          created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
          updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
          UNIQUE(subject_type, subject_id, scope_type, scope_id)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_iam_membership_subject ON iam_membership(subject_type, subject_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_membership_scope ON iam_membership(scope_type, scope_id)`,
		`CREATE TABLE IF NOT EXISTS iam_role_binding (
          id BIGSERIAL PRIMARY KEY,
          binding_id TEXT NOT NULL UNIQUE,
          subject_type TEXT NOT NULL DEFAULT 'user',
          subject_id TEXT NOT NULL,
          role_code TEXT NOT NULL,
          scope_type TEXT NOT NULL DEFAULT 'global',
          scope_id TEXT NOT NULL DEFAULT '*',
          app_code TEXT,
          effect TEXT NOT NULL DEFAULT 'allow',
          created_by TEXT,
          created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
          updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
          UNIQUE(subject_type, subject_id, role_code, scope_type, scope_id, app_code)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_iam_role_binding_subject ON iam_role_binding(subject_type, subject_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_role_binding_scope ON iam_role_binding(scope_type, scope_id)`,
		`CREATE TABLE IF NOT EXISTS iam_resource_grant (
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
        )`,
		`CREATE INDEX IF NOT EXISTS idx_iam_grant_resource ON iam_resource_grant(app, resource_type, resource_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_grant_object ON iam_resource_grant(object)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_grant_subject ON iam_resource_grant(subject_type, subject_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_grant_scope ON iam_resource_grant(org_id, project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_grant_actions_gin ON iam_resource_grant USING GIN(actions)`,
	}
	for _, stmt := range stmts {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresService) SaveResourceGrant(ctx context.Context, grant aisphereauth.ResourceGrant) (*aisphereauth.ResourceGrant, error) {
	grant = prepareGrantForSave(grant)
	actions, _ := json.Marshal(grant.Actions)
	_, err := s.pool.Exec(ctx, `INSERT INTO iam_resource_grant(grant_id,app,org_id,project_id,resource_type,resource_id,object,subject_type,subject_id,role,actions,effect,expires_at,created_by,created_at,updated_at)
        VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12,$13,$14,$15,$16)
        ON CONFLICT (grant_id) DO UPDATE SET app=EXCLUDED.app,org_id=EXCLUDED.org_id,project_id=EXCLUDED.project_id,resource_type=EXCLUDED.resource_type,resource_id=EXCLUDED.resource_id,object=EXCLUDED.object,subject_type=EXCLUDED.subject_type,subject_id=EXCLUDED.subject_id,role=EXCLUDED.role,actions=EXCLUDED.actions,effect=EXCLUDED.effect,expires_at=EXCLUDED.expires_at,created_by=EXCLUDED.created_by,updated_at=EXCLUDED.updated_at`,
		grant.ID, grant.App, nullString(grant.OrgID), nullString(grant.ProjectID), grant.ResourceType, grant.ResourceID, nullString(grant.Object), grant.SubjectType, grant.SubjectID, grant.Role, string(actions), grant.Effect, nullTimeMillis(grant.ExpiresAt), nullString(grant.CreatedBy), timeFromMillis(grant.CreatedAt), timeFromMillis(grant.UpdatedAt))
	if err != nil {
		return nil, err
	}
	out := grant
	return &out, nil
}

func (s *PostgresService) DeleteResourceGrant(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM iam_resource_grant WHERE grant_id=$1`, strings.TrimSpace(id))
	return err
}

func (s *PostgresService) ListResourceGrants(ctx context.Context, q aisphereauth.ResourceGrantQuery) (*aisphereauth.ResourceGrantListResponse, error) {
	where, args := pgGrantWhere(q)
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM iam_resource_grant `+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	limit := q.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, `SELECT grant_id,app,org_id,project_id,resource_type,resource_id,object,subject_type,subject_id,role,actions::text,effect,expires_at,created_by,(EXTRACT(EPOCH FROM created_at)*1000)::bigint,(EXTRACT(EPOCH FROM updated_at)*1000)::bigint FROM iam_resource_grant `+where+` ORDER BY updated_at DESC,id DESC LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []aisphereauth.ResourceGrant{}
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &aisphereauth.ResourceGrantListResponse{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *PostgresService) CheckResourceGrant(ctx context.Context, req aisphereauth.ResourceGrantCheckRequest) (*aisphereauth.ResourceGrantCheckDecision, error) {
	decision := &aisphereauth.ResourceGrantCheckDecision{Allow: false, Reason: "no_matching_grant", App: normalize(req.App), Subject: effectiveSubject(req), Object: strings.TrimSpace(req.Object), ResourceType: normalize(req.ResourceType), ResourceID: strings.TrimSpace(req.ResourceID), Action: normalizeAction(req.Action), TraceID: req.TraceID}
	if decision.ResourceType == "" || decision.ResourceID == "" {
		app, typ, id := parseObject(decision.Object)
		if decision.App == "" {
			decision.App = app
		}
		if decision.ResourceType == "" {
			decision.ResourceType = typ
		}
		if decision.ResourceID == "" {
			decision.ResourceID = id
		}
	}
	if decision.Action == "" {
		decision.Reason = "missing_action"
		return decision, nil
	}
	list, err := s.ListResourceGrants(ctx, aisphereauth.ResourceGrantQuery{App: decision.App, ResourceType: decision.ResourceType, ResourceID: decision.ResourceID, Limit: 500})
	if err != nil {
		return nil, err
	}
	for _, g := range list.Items {
		if grantExpired(g) || !grantTargetsResource(g, decision.App, decision.Object, decision.ResourceType, decision.ResourceID) {
			continue
		}
		if !grantTargetsSubject(g, req) {
			continue
		}
		if !grantAllowsAction(g, decision.Action) {
			continue
		}
		grant := g
		decision.MatchedGrant = &grant
		decision.Allow = strings.ToLower(g.Effect) != "deny"
		if decision.Allow {
			decision.Reason = "matched_resource_grant"
		} else {
			decision.Reason = "denied_by_resource_grant"
		}
		return decision, nil
	}
	return decision, nil
}

func pgGrantWhere(q aisphereauth.ResourceGrantQuery) (string, []interface{}) {
	clauses := []string{"1=1"}
	args := []interface{}{}
	add := func(col string, v string) {
		if strings.TrimSpace(v) == "" {
			return
		}
		args = append(args, strings.TrimSpace(v))
		clauses = append(clauses, fmt.Sprintf("%s=$%d", col, len(args)))
	}
	add("app", normalize(q.App))
	add("org_id", q.OrgID)
	add("project_id", q.ProjectID)
	add("resource_type", normalize(q.ResourceType))
	add("resource_id", q.ResourceID)
	add("object", q.Object)
	add("subject_type", normalize(q.SubjectType))
	add("subject_id", q.SubjectID)
	return "WHERE " + strings.Join(clauses, " AND "), args
}
