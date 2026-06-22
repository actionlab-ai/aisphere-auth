package iam

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
)

type Service interface {
	SaveResourceGrant(ctx context.Context, grant aisphereauth.ResourceGrant) (*aisphereauth.ResourceGrant, error)
	DeleteResourceGrant(ctx context.Context, id string) error
	ListResourceGrants(ctx context.Context, q aisphereauth.ResourceGrantQuery) (*aisphereauth.ResourceGrantListResponse, error)
	CheckResourceGrant(ctx context.Context, req aisphereauth.ResourceGrantCheckRequest) (*aisphereauth.ResourceGrantCheckDecision, error)
}

type MemoryService struct {
	mu     sync.RWMutex
	grants map[string]aisphereauth.ResourceGrant
}

func NewMemoryService() *MemoryService {
	return &MemoryService{grants: map[string]aisphereauth.ResourceGrant{}}
}

func (s *MemoryService) Close() error { return nil }

func (s *MemoryService) SaveResourceGrant(ctx context.Context, grant aisphereauth.ResourceGrant) (*aisphereauth.ResourceGrant, error) {
	grant = prepareGrantForSave(grant)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.grants[grant.ID] = grant
	out := grant
	return &out, nil
}

func (s *MemoryService) DeleteResourceGrant(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.grants, strings.TrimSpace(id))
	return nil
}

func (s *MemoryService) ListResourceGrants(ctx context.Context, q aisphereauth.ResourceGrantQuery) (*aisphereauth.ResourceGrantListResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]aisphereauth.ResourceGrant, 0, len(s.grants))
	for _, g := range s.grants {
		if !grantMatchesQuery(g, q) {
			continue
		}
		items = append(items, g)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt > items[j].CreatedAt })
	total := len(items)
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	limit := q.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset > total {
		items = []aisphereauth.ResourceGrant{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		items = items[offset:end]
	}
	return &aisphereauth.ResourceGrantListResponse{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *MemoryService) CheckResourceGrant(ctx context.Context, req aisphereauth.ResourceGrantCheckRequest) (*aisphereauth.ResourceGrantCheckDecision, error) {
	decision := &aisphereauth.ResourceGrantCheckDecision{
		Allow:        false,
		Reason:       "no_matching_grant",
		App:          normalize(req.App),
		Subject:      effectiveSubject(req),
		Object:       strings.TrimSpace(req.Object),
		ResourceType: normalize(req.ResourceType),
		ResourceID:   strings.TrimSpace(req.ResourceID),
		Action:       normalizeAction(req.Action),
		TraceID:      req.TraceID,
	}
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, g := range s.grants {
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

func prepareGrantForSave(grant aisphereauth.ResourceGrant) aisphereauth.ResourceGrant {
	now := time.Now().UnixMilli()
	grant.ID = strings.TrimSpace(grant.ID)
	if grant.ID == "" {
		grant.ID = "grant_" + randomHex(12)
	}
	grant.App = normalize(grant.App)
	if grant.App == "" {
		grant.App = "aihub"
	}
	grant.OrgID = strings.TrimSpace(grant.OrgID)
	grant.ProjectID = strings.TrimSpace(grant.ProjectID)
	grant.ResourceType = normalize(grant.ResourceType)
	grant.ResourceID = strings.TrimSpace(grant.ResourceID)
	grant.Object = strings.TrimSpace(grant.Object)
	if grant.Object == "" && grant.App != "" && grant.ResourceType != "" && grant.ResourceID != "" {
		grant.Object = grant.App + ":" + grant.ResourceType + ":" + grant.ResourceID
	}
	grant.SubjectType = normalize(grant.SubjectType)
	grant.SubjectID = strings.TrimSpace(grant.SubjectID)
	grant.Role = normalize(defaultString(grant.Role, "viewer"))
	grant.Effect = normalize(defaultString(grant.Effect, "allow"))
	for i := range grant.Actions {
		grant.Actions[i] = normalizeAction(grant.Actions[i])
	}
	if grant.CreatedAt == 0 {
		grant.CreatedAt = now
	}
	grant.UpdatedAt = now
	if grant.SubjectType == "public" && grant.SubjectID == "" {
		grant.SubjectID = "*"
	}
	return grant
}

func grantMatchesQuery(g aisphereauth.ResourceGrant, q aisphereauth.ResourceGrantQuery) bool {
	if q.App != "" && normalize(q.App) != g.App {
		return false
	}
	if q.ResourceType != "" && normalize(q.ResourceType) != g.ResourceType {
		return false
	}
	if q.ResourceID != "" && strings.TrimSpace(q.ResourceID) != g.ResourceID {
		return false
	}
	if q.Object != "" && strings.TrimSpace(q.Object) != g.Object {
		return false
	}
	if q.SubjectType != "" && normalize(q.SubjectType) != g.SubjectType {
		return false
	}
	if q.SubjectID != "" && strings.TrimSpace(q.SubjectID) != g.SubjectID {
		return false
	}
	return true
}

func grantTargetsResource(g aisphereauth.ResourceGrant, app, object, typ, id string) bool {
	if g.App != "" && g.App != "*" && app != "" && g.App != app {
		return false
	}
	if g.Object != "" && object != "" {
		return objectMatch(g.Object, object)
	}
	if g.ResourceType != "" && typ != "" && g.ResourceType != typ {
		return false
	}
	if g.ResourceID != "" && g.ResourceID != "*" && id != "" && g.ResourceID != id {
		return false
	}
	return true
}

func objectMatch(pattern, object string) bool {
	pattern = strings.TrimSpace(pattern)
	object = strings.TrimSpace(object)
	if pattern == "*" || pattern == object {
		return true
	}
	if strings.HasSuffix(pattern, ":*") {
		return strings.HasPrefix(object, strings.TrimSuffix(pattern, "*"))
	}
	return false
}

func grantTargetsSubject(g aisphereauth.ResourceGrant, req aisphereauth.ResourceGrantCheckRequest) bool {
	subjType := normalize(g.SubjectType)
	if subjType == "public" {
		return true
	}
	p := req.Principal
	subjects := candidateSubjects(req)
	switch subjType {
	case "user", "human", "service", "agent":
		return containsAny(subjects, g.SubjectID)
	case "group":
		if p == nil {
			return false
		}
		for _, group := range p.Groups {
			if hierarchyMatch(g.SubjectID, group) {
				return true
			}
		}
	case "org", "organization":
		orgs := []string{req.OrgID}
		if p != nil {
			orgs = append(orgs, p.OrgID, p.Organization)
		}
		return containsAny(orgs, g.SubjectID)
	case "project":
		projects := []string{req.ProjectID}
		if p != nil {
			projects = append(projects, p.ProjectIDs...)
			if p.Claims != nil {
				if v, ok := p.Claims["projectId"].(string); ok {
					projects = append(projects, v)
				}
			}
		}
		return containsAny(projects, g.SubjectID)
	}
	return false
}

func candidateSubjects(req aisphereauth.ResourceGrantCheckRequest) []string {
	out := []string{req.Subject}
	if p := req.Principal; p != nil {
		out = append(out, p.EffectiveSubject(), p.SubjectID, p.CasdoorSubject, p.Username)
	}
	return out
}

func containsAny(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, v := range values {
		if strings.TrimSpace(v) == target {
			return true
		}
	}
	return false
}

func hierarchyMatch(grantGroup, actualGroup string) bool {
	grantGroup = strings.Trim(strings.TrimSpace(grantGroup), "/")
	actualGroup = strings.Trim(strings.TrimSpace(actualGroup), "/")
	if grantGroup == "" || actualGroup == "" {
		return false
	}
	return actualGroup == grantGroup || strings.HasPrefix(actualGroup, grantGroup+"/")
}

func grantAllowsAction(g aisphereauth.ResourceGrant, action string) bool {
	action = normalizeAction(action)
	if action == "" {
		return false
	}
	for _, a := range g.Actions {
		if actionMatches(a, action) {
			return true
		}
	}
	for _, a := range roleActions(g.Role) {
		if actionMatches(a, action) {
			return true
		}
	}
	return false
}

func actionMatches(pattern, action string) bool {
	pattern = normalizeAction(pattern)
	if pattern == "*" || pattern == action {
		return true
	}
	if pattern == "run" && action == "use" {
		return true
	}
	if pattern == "write" {
		return action == "create" || action == "update" || action == "delete" || action == "write" || action == "publish" || action == "rollback"
	}
	if strings.HasSuffix(pattern, ":*") {
		return strings.HasPrefix(action, strings.TrimSuffix(pattern, "*"))
	}
	return false
}

func roleActions(role string) []string {
	switch normalize(role) {
	case "viewer", "reader":
		return []string{"read", "view", "list", "download"}
	case "runner", "executor":
		return []string{"read", "view", "list", "run"}
	case "editor", "developer", "contributor":
		return []string{"read", "view", "list", "download", "write", "create", "update", "delete", "publish", "rollback"}
	case "reviewer", "approver":
		return []string{"read", "view", "list", "download", "approve", "reject", "review"}
	case "admin", "owner":
		return []string{"*"}
	default:
		return []string{"read"}
	}
}

func grantExpired(g aisphereauth.ResourceGrant) bool {
	return g.ExpiresAt > 0 && time.Now().UnixMilli() > g.ExpiresAt
}

func effectiveSubject(req aisphereauth.ResourceGrantCheckRequest) string {
	if strings.TrimSpace(req.Subject) != "" {
		return strings.TrimSpace(req.Subject)
	}
	if req.Principal != nil {
		return req.Principal.EffectiveSubject()
	}
	return ""
}

func parseObject(object string) (app, typ, id string) {
	parts := strings.Split(strings.TrimSpace(object), ":")
	if len(parts) >= 3 {
		return normalize(parts[0]), normalize(parts[1]), strings.Join(parts[2:], ":")
	}
	return "", "", ""
}

func normalize(v string) string       { return strings.ToLower(strings.TrimSpace(v)) }
func normalizeAction(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
func defaultString(v, d string) string {
	if strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return d
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strings.ReplaceAll(time.Now().Format("20060102150405.000000"), ".", "")
	}
	return hex.EncodeToString(b)
}
