package iam

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
	"github.com/go-sql-driver/mysql"
)

type MySQLService struct {
	db *sql.DB
}

func NewMySQLService(cfg config.DatabaseConfig) (*MySQLService, error) {
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("database.dsn is required for mysql IAM provider")
	}
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns <= 0 {
		cfg.MaxOpenConns = 30
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 10
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		if cfg.AutoMigrate && isUnknownDatabaseError(err) {
			if createErr := ensureMySQLDatabase(ctx, cfg.DSN); createErr != nil {
				return nil, fmt.Errorf("create mysql database for IAM autoMigrate: %w", createErr)
			}
			db, err = sql.Open("mysql", cfg.DSN)
			if err != nil {
				return nil, err
			}
			db.SetMaxOpenConns(cfg.MaxOpenConns)
			db.SetMaxIdleConns(cfg.MaxIdleConns)
			db.SetConnMaxLifetime(30 * time.Minute)
			if err := db.PingContext(ctx); err != nil {
				_ = db.Close()
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	svc := &MySQLService{db: db}
	if cfg.AutoMigrate {
		if err := svc.AutoMigrate(ctx); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return svc, nil
}

func (s *MySQLService) Close() error { return s.db.Close() }

func isUnknownDatabaseError(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1049
}

func ensureMySQLDatabase(ctx context.Context, dsn string) error {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return err
	}
	dbName := strings.TrimSpace(cfg.DBName)
	if dbName == "" {
		return fmt.Errorf("database name is required in database.dsn")
	}
	cfg.DBName = ""
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+quoteMySQLIdent(dbName)+" DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	return err
}

func quoteMySQLIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func (s *MySQLService) AutoMigrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS iam_org (
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
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS iam_project (
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
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS iam_group (
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
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS iam_membership (
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
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS iam_role_binding (
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
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS iam_resource_grant (
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
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQLService) SaveResourceGrant(ctx context.Context, grant aisphereauth.ResourceGrant) (*aisphereauth.ResourceGrant, error) {
	grant = prepareGrantForSave(grant)
	actions, _ := json.Marshal(grant.Actions)
	_, err := s.db.ExecContext(ctx, `INSERT INTO iam_resource_grant(grant_id,app,org_id,project_id,resource_type,resource_id,object,subject_type,subject_id,role,actions,effect,expires_at,created_by,created_at,updated_at)
        VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
        ON DUPLICATE KEY UPDATE app=VALUES(app),org_id=VALUES(org_id),project_id=VALUES(project_id),resource_type=VALUES(resource_type),resource_id=VALUES(resource_id),object=VALUES(object),subject_type=VALUES(subject_type),subject_id=VALUES(subject_id),role=VALUES(role),actions=VALUES(actions),effect=VALUES(effect),expires_at=VALUES(expires_at),created_by=VALUES(created_by),updated_at=VALUES(updated_at)`,
		grant.ID, grant.App, nullString(grant.OrgID), nullString(grant.ProjectID), grant.ResourceType, grant.ResourceID, nullString(grant.Object), grant.SubjectType, grant.SubjectID, grant.Role, string(actions), grant.Effect, nullTimeMillis(grant.ExpiresAt), nullString(grant.CreatedBy), timeFromMillis(grant.CreatedAt), timeFromMillis(grant.UpdatedAt))
	if err != nil {
		return nil, err
	}
	out := grant
	return &out, nil
}

func (s *MySQLService) DeleteResourceGrant(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM iam_resource_grant WHERE grant_id=?`, strings.TrimSpace(id))
	return err
}

func (s *MySQLService) ListResourceGrants(ctx context.Context, q aisphereauth.ResourceGrantQuery) (*aisphereauth.ResourceGrantListResponse, error) {
	where, args := grantWhere(q)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM iam_resource_grant `+where, args...).Scan(&total); err != nil {
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
	args2 := append(append([]interface{}{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT grant_id,app,org_id,project_id,resource_type,resource_id,object,subject_type,subject_id,role,actions,effect,expires_at,created_by,UNIX_TIMESTAMP(created_at)*1000,UNIX_TIMESTAMP(updated_at)*1000 FROM iam_resource_grant `+where+` ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?`, args2...)
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

func (s *MySQLService) CheckResourceGrant(ctx context.Context, req aisphereauth.ResourceGrantCheckRequest) (*aisphereauth.ResourceGrantCheckDecision, error) {
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
	q := aisphereauth.ResourceGrantQuery{App: decision.App, ResourceType: decision.ResourceType, ResourceID: decision.ResourceID}
	list, err := s.ListResourceGrants(ctx, q)
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

type grantScanner interface {
	Scan(dest ...interface{}) error
}

func scanGrant(row grantScanner) (aisphereauth.ResourceGrant, error) {
	var g aisphereauth.ResourceGrant
	var orgID, projectID, object, actions, createdBy sql.NullString
	var expires sql.NullTime
	if err := row.Scan(&g.ID, &g.App, &orgID, &projectID, &g.ResourceType, &g.ResourceID, &object, &g.SubjectType, &g.SubjectID, &g.Role, &actions, &g.Effect, &expires, &createdBy, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return g, err
	}
	if orgID.Valid {
		g.OrgID = orgID.String
	}
	if projectID.Valid {
		g.ProjectID = projectID.String
	}
	if object.Valid {
		g.Object = object.String
	}
	if createdBy.Valid {
		g.CreatedBy = createdBy.String
	}
	if actions.Valid && actions.String != "" {
		_ = json.Unmarshal([]byte(actions.String), &g.Actions)
	}
	if expires.Valid {
		g.ExpiresAt = expires.Time.UnixMilli()
	}
	return g, nil
}

func grantWhere(q aisphereauth.ResourceGrantQuery) (string, []interface{}) {
	clauses := []string{"1=1"}
	args := []interface{}{}
	add := func(cond string, v string) {
		if strings.TrimSpace(v) != "" {
			clauses = append(clauses, cond)
			args = append(args, strings.TrimSpace(v))
		}
	}
	add("app=?", normalize(q.App))
	add("org_id=?", q.OrgID)
	add("project_id=?", q.ProjectID)
	add("resource_type=?", normalize(q.ResourceType))
	add("resource_id=?", q.ResourceID)
	add("object=?", q.Object)
	add("subject_type=?", normalize(q.SubjectType))
	add("subject_id=?", q.SubjectID)
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func nullString(v string) sql.NullString {
	if strings.TrimSpace(v) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}
func nullTimeMillis(ms int64) sql.NullTime {
	if ms <= 0 {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: time.UnixMilli(ms), Valid: true}
}
func timeFromMillis(ms int64) time.Time {
	if ms <= 0 {
		return time.Now()
	}
	return time.UnixMilli(ms)
}
