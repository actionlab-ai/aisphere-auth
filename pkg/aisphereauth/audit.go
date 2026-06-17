package aisphereauth

import "time"

const (
	AuditResultSuccess = "success"
	AuditResultFailure = "failure"
	AuditResultAllow   = "allow"
	AuditResultDeny    = "deny"
)

// AuditEvent is the public audit record contract shared by SDKs and the auth service.
// Business modules should fill actor/resource/action/result and let the auth service
// fill ID / CreatedAt / TraceID when they are not provided.
type AuditEvent struct {
	ID            string            `json:"id,omitempty"`
	TraceID       string            `json:"traceId,omitempty"`
	ActorSubject  string            `json:"actorSubject"`
	ActorName     string            `json:"actorName,omitempty"`
	App           string            `json:"app,omitempty"`
	ResourceType  string            `json:"resourceType"`
	ResourceID    string            `json:"resourceId,omitempty"`
	Action        string            `json:"action"`
	Result        string            `json:"result"`
	Reason        string            `json:"reason,omitempty"`
	IP            string            `json:"ip,omitempty"`
	UserAgent     string            `json:"userAgent,omitempty"`
	RequestPath   string            `json:"requestPath,omitempty"`
	RequestMethod string            `json:"requestMethod,omitempty"`
	CreatedAt     time.Time         `json:"createdAt,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// AuditListRequest filters audit events. All string filters are exact-match filters.
type AuditListRequest struct {
	TraceID      string `json:"traceId,omitempty"`
	ActorSubject string `json:"actorSubject,omitempty"`
	App          string `json:"app,omitempty"`
	ResourceType string `json:"resourceType,omitempty"`
	ResourceID   string `json:"resourceId,omitempty"`
	Action       string `json:"action,omitempty"`
	Result       string `json:"result,omitempty"`
	Limit        int    `json:"limit,omitempty"`
	Offset       int    `json:"offset,omitempty"`
}

// AuditListResponse returns newest events first.
type AuditListResponse struct {
	Items  []AuditEvent `json:"items"`
	Total  int          `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}

// NewAuditEventFromPrincipal creates a minimal audit event from a Principal.
func NewAuditEventFromPrincipal(p *Principal, resourceType, resourceID, action, result string) AuditEvent {
	event := AuditEvent{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		Result:       result,
	}
	if p == nil {
		return event
	}
	event.ActorSubject = p.EffectiveSubject()
	event.ActorName = p.DisplayName
	if event.ActorName == "" {
		event.ActorName = p.Username
	}
	event.App = p.App
	return event
}
