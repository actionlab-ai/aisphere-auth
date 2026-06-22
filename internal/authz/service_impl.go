package authz

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/casdoor"
	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/actionlab-ai/aisphere-auth/internal/iam"
	internalprincipal "github.com/actionlab-ai/aisphere-auth/internal/principal"
	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
)

var ErrMissingPermission = errors.New("missing subject/object/action")

type DefaultService struct {
	cfg      config.Config
	enforcer casdoor.Enforcer
	iam      iam.Service
	cache    *DecisionCache
}

func NewDefaultService(cfg config.Config, enforcer casdoor.Enforcer, iamSvc iam.Service) *DefaultService {
	var cache *DecisionCache
	if cfg.Authz.CacheEnabled {
		cache = NewDecisionCache()
	}
	return &DefaultService{cfg: cfg, enforcer: enforcer, iam: iamSvc, cache: cache}
}

func (s *DefaultService) Check(ctx context.Context, req CheckRequest) (*Decision, error) {
	subject := strings.TrimSpace(req.Subject)
	if subject == "" && req.Principal != nil {
		subject = req.Principal.EffectiveSubject()
	}
	object := strings.TrimSpace(req.Object)
	action := strings.TrimSpace(req.Action)
	decision := &Decision{Source: "casdoor", Subject: subject, Object: object, Action: action, App: req.App, TraceID: req.TraceID, OrgID: req.OrgID, ProjectID: req.ProjectID, ResourceType: req.ResourceType, ResourceID: req.ResourceID}
	if subject == "" || object == "" || action == "" {
		decision.Reason = ErrMissingPermission.Error()
		return decision, ErrMissingPermission
	}

	if role, ok := builtinRoleAllows(req.Principal, req.App, object, action); ok {
		decision.Allow = true
		decision.Source = "builtin-role"
		decision.Reason = "matched_builtin_role:" + role
		return decision, nil
	}

	if s.iam != nil {
		grantDecision, err := s.iam.CheckResourceGrant(ctx, aisphereauth.ResourceGrantCheckRequest{
			Principal:    convertPrincipal(req.Principal),
			Subject:      subject,
			App:          req.App,
			OrgID:        req.OrgID,
			ProjectID:    req.ProjectID,
			Object:       object,
			ResourceType: req.ResourceType,
			ResourceID:   req.ResourceID,
			Action:       action,
			TraceID:      req.TraceID,
		})
		if err == nil && grantDecision != nil && grantDecision.Allow {
			decision.Allow = true
			decision.Source = "iam-resource-grant"
			decision.Reason = grantDecision.Reason
			decision.ResourceType = grantDecision.ResourceType
			decision.ResourceID = grantDecision.ResourceID
			if grantDecision.MatchedGrant != nil {
				decision.MatchedGrantID = grantDecision.MatchedGrant.ID
			}
			return decision, nil
		}
		if err == nil && grantDecision != nil && grantDecision.MatchedGrant != nil && !grantDecision.Allow {
			decision.Source = "iam-resource-grant"
			decision.Reason = grantDecision.Reason
			decision.MatchedGrantID = grantDecision.MatchedGrant.ID
			return decision, nil
		}
	}

	cacheKeySubject := strings.Join([]string{strings.TrimSpace(req.App), subject}, "|")
	if s.cache != nil {
		if cached, ok := s.cache.Get(cacheKeySubject, object, action); ok {
			cached.Source = "cache"
			cached.TraceID = req.TraceID
			return &cached, nil
		}
	}
	resp, err := s.enforcer.Enforce(ctx, casdoor.EnforceRequest{PermissionID: s.cfg.Casdoor.PermissionID, Sub: subject, Obj: object, Act: action})
	if err != nil {
		decision.Reason = err.Error()
		return decision, err
	}
	decision.Allow = resp != nil && resp.Allow
	if !decision.Allow {
		decision.Reason = "casdoor_denied"
	}
	if s.cfg.Authz.CacheEnabled {
		ttl := time.Duration(s.cfg.Authz.CacheTTLSeconds) * time.Second
		if ttl > 0 && ttl <= time.Minute {
			cacheDecision := *decision
			cacheDecision.TraceID = ""
			s.cache.Set(cacheKeySubject, object, action, cacheDecision, ttl)
		}
	}
	return decision, nil
}

func builtinRoleAllows(p *internalprincipal.Principal, app, object, action string) (string, bool) {
	if p == nil {
		return "", false
	}
	app = strings.ToLower(strings.TrimSpace(app))
	if app == "" {
		app = "aihub"
	}
	action = strings.ToLower(strings.TrimSpace(action))
	object = strings.ToLower(strings.TrimSpace(object))
	for _, role := range p.Roles {
		normalized := normalizeRole(role)
		if normalized == "" {
			continue
		}
		if isPlatformAdminRole(normalized) {
			return normalized, true
		}
		if roleMatchesApp(normalized, app, "admin") {
			return normalized, true
		}
		if roleMatchesApp(normalized, app, "editor") {
			if isEditorAction(action) {
				return normalized, true
			}
		}
		if roleMatchesApp(normalized, app, "viewer") || roleMatchesApp(normalized, app, "reader") {
			if isReadAction(action) {
				return normalized, true
			}
		}
		if strings.Contains(normalized, "admin") && (strings.HasPrefix(object, app+":") || (app == "aihub" && strings.HasPrefix(object, "aihub:"))) {
			return normalized, true
		}
	}
	return "", false
}

func normalizeRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	role = strings.ReplaceAll(role, "-", "_")
	if strings.Contains(role, "/") {
		parts := strings.Split(role, "/")
		role = parts[len(parts)-1]
	}
	return role
}

func isPlatformAdminRole(role string) bool {
	switch role {
	case "admin", "role_admin", "platform_admin", "role_platform_admin", "platform_super_admin", "role_platform_super_admin", "super_admin", "role_super_admin", "aisphere_admin", "role_aisphere_admin":
		return true
	}
	return false
}

func roleMatchesApp(role, app, suffix string) bool {
	return role == app+"_"+suffix || role == "role_"+app+"_"+suffix || role == app+"_"+suffix+"_role" || role == "app_"+suffix || role == "role_app_"+suffix
}

func isReadAction(action string) bool {
	switch action {
	case "read", "view", "list", "download", "admin:read":
		return true
	}
	return strings.HasSuffix(action, ":read")
}

func isEditorAction(action string) bool {
	if isReadAction(action) {
		return true
	}
	switch action {
	case "write", "create", "update", "delete", "publish", "rollback", "approve", "reject", "review", "run", "cancel", "retry":
		return true
	}
	return strings.HasSuffix(action, ":write")
}

func (s *DefaultService) BatchCheck(ctx context.Context, reqs []CheckRequest) ([]Decision, error) {
	out := make([]Decision, 0, len(reqs))
	for _, req := range reqs {
		decision, err := s.Check(ctx, req)
		if decision != nil {
			out = append(out, *decision)
		}
		if err != nil && s.cfg.Authz.FailClosed {
			return out, err
		}
	}
	return out, nil
}

func convertPrincipal(p *internalprincipal.Principal) *aisphereauth.Principal {
	if p == nil {
		return nil
	}
	out := &aisphereauth.Principal{
		SubjectID:      p.SubjectID,
		CasdoorSubject: p.CasdoorSubject,
		Username:       p.Username,
		DisplayName:    p.DisplayName,
		Email:          p.Email,
		Organization:   p.Organization,
		Roles:          append([]string(nil), p.Roles...),
		Groups:         append([]string(nil), p.Groups...),
		OrgID:          p.OrgID,
		ProjectIDs:     append([]string(nil), p.ProjectIDs...),
		Claims:         p.Claims,
		App:            p.App,
		SessionID:      p.SessionID,
		TokenID:        p.TokenID,
		AuthProvider:   p.AuthProvider,
		AuthTimeUnix:   p.AuthTimeUnix,
		ExpiresAtUnix:  p.ExpiresAtUnix,
	}
	return out
}
