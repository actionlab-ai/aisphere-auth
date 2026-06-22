package aisphereauth

// ResourceGrant is the shared IAM contract for resource-level sharing and collaboration.
// It is intentionally app-agnostic so Skill / SkillSet / Agent / Workflow / KnowledgeBase / ModelKey
// can all use the same sharing model.
type ResourceGrant struct {
	ID           string            `json:"id,omitempty"`
	App          string            `json:"app"`
	OrgID        string            `json:"orgId,omitempty"`
	ProjectID    string            `json:"projectId,omitempty"`
	ResourceType string            `json:"resourceType"`
	ResourceID   string            `json:"resourceId"`
	Object       string            `json:"object,omitempty"`
	SubjectType  string            `json:"subjectType"` // user / group / org / project / public
	SubjectID    string            `json:"subjectId"`
	Role         string            `json:"role"` // viewer / runner / editor / reviewer / admin / owner
	Actions      []string          `json:"actions,omitempty"`
	Effect       string            `json:"effect,omitempty"` // allow / deny; empty defaults to allow
	ExpiresAt    int64             `json:"expiresAt,omitempty"`
	CreatedBy    string            `json:"createdBy,omitempty"`
	CreatedAt    int64             `json:"createdAt,omitempty"`
	UpdatedAt    int64             `json:"updatedAt,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type ResourceGrantQuery struct {
	App          string `json:"app,omitempty"`
	OrgID        string `json:"orgId,omitempty"`
	ProjectID    string `json:"projectId,omitempty"`
	ResourceType string `json:"resourceType,omitempty"`
	ResourceID   string `json:"resourceId,omitempty"`
	Object       string `json:"object,omitempty"`
	SubjectType  string `json:"subjectType,omitempty"`
	SubjectID    string `json:"subjectId,omitempty"`
	Limit        int    `json:"limit,omitempty"`
	Offset       int    `json:"offset,omitempty"`
}

type ResourceGrantListResponse struct {
	Items  []ResourceGrant `json:"items"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type ResourceGrantCheckRequest struct {
	Principal    *Principal `json:"principal,omitempty"`
	Subject      string     `json:"subject,omitempty"`
	App          string     `json:"app,omitempty"`
	OrgID        string     `json:"orgId,omitempty"`
	ProjectID    string     `json:"projectId,omitempty"`
	Object       string     `json:"object,omitempty"`
	ResourceType string     `json:"resourceType,omitempty"`
	ResourceID   string     `json:"resourceId,omitempty"`
	Action       string     `json:"action"`
	TraceID      string     `json:"traceId,omitempty"`
}

type ResourceGrantCheckDecision struct {
	Allow        bool           `json:"allow"`
	Reason       string         `json:"reason,omitempty"`
	App          string         `json:"app,omitempty"`
	Subject      string         `json:"subject,omitempty"`
	Object       string         `json:"object,omitempty"`
	ResourceType string         `json:"resourceType,omitempty"`
	ResourceID   string         `json:"resourceId,omitempty"`
	Action       string         `json:"action,omitempty"`
	MatchedGrant *ResourceGrant `json:"matchedGrant,omitempty"`
	TraceID      string         `json:"traceId,omitempty"`
	Debug        map[string]any `json:"debug,omitempty"`
}
