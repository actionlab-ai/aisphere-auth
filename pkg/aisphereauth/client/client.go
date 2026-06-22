package client

import (
	"context"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
)

// CheckRequest mirrors the server-side authorization check payload.
// Keep JSON field names stable because this type is consumed by external services.
type CheckRequest struct {
	Principal    *aisphereauth.Principal `json:"principal,omitempty"`
	Subject      string                  `json:"subject,omitempty"`
	Object       string                  `json:"object"`
	Action       string                  `json:"action"`
	App          string                  `json:"app,omitempty"`
	TraceID      string                  `json:"traceId,omitempty"`
	OrgID        string                  `json:"orgId,omitempty"`
	ProjectID    string                  `json:"projectId,omitempty"`
	ResourceType string                  `json:"resourceType,omitempty"`
	ResourceID   string                  `json:"resourceId,omitempty"`
}

// Decision mirrors the server-side authorization decision response.
type Decision struct {
	Allow          bool   `json:"allow"`
	Source         string `json:"source"`
	Reason         string `json:"reason,omitempty"`
	Subject        string `json:"subject"`
	Object         string `json:"object"`
	Action         string `json:"action"`
	TraceID        string `json:"traceId,omitempty"`
	App            string `json:"app,omitempty"`
	OrgID          string `json:"orgId,omitempty"`
	ProjectID      string `json:"projectId,omitempty"`
	ResourceType   string `json:"resourceType,omitempty"`
	ResourceID     string `json:"resourceId,omitempty"`
	MatchedGrantID string `json:"matchedGrantId,omitempty"`
}

// Client is the public SDK contract used by business services.
type Client interface {
	LoginURL(app string, redirect string) string
	LogoutURL(global bool) string
	Introspect(ctx context.Context, sessionID string, app string) (*aisphereauth.Principal, error)
	Check(ctx context.Context, req CheckRequest) (*Decision, error)
	BatchCheck(ctx context.Context, reqs []CheckRequest) ([]Decision, error)
	WriteAudit(ctx context.Context, event aisphereauth.AuditEvent) (*aisphereauth.AuditEvent, error)
	ListAudit(ctx context.Context, req aisphereauth.AuditListRequest) (*aisphereauth.AuditListResponse, error)
}
