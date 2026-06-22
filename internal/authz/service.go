package authz

import (
	"context"

	"github.com/actionlab-ai/aisphere-auth/internal/principal"
)

type CheckRequest struct {
	Principal    *principal.Principal `json:"principal,omitempty"`
	Subject      string               `json:"subject,omitempty"`
	Object       string               `json:"object"`
	Action       string               `json:"action"`
	App          string               `json:"app,omitempty"`
	TraceID      string               `json:"traceId,omitempty"`
	OrgID        string               `json:"orgId,omitempty"`
	ProjectID    string               `json:"projectId,omitempty"`
	ResourceType string               `json:"resourceType,omitempty"`
	ResourceID   string               `json:"resourceId,omitempty"`
}

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

type Service interface {
	Check(ctx context.Context, req CheckRequest) (*Decision, error)
	BatchCheck(ctx context.Context, reqs []CheckRequest) ([]Decision, error)
}
