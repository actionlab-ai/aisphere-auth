package authz

import (
	"context"

	"github.com/actionlab-ai/aisphere-auth/internal/principal"
)

type CheckRequest struct {
	Principal *principal.Principal `json:"principal,omitempty"`
	Subject   string               `json:"subject,omitempty"`
	Object    string               `json:"object"`
	Action    string               `json:"action"`
	App       string               `json:"app,omitempty"`
	TraceID   string               `json:"traceId,omitempty"`
}

type Decision struct {
	Allow   bool   `json:"allow"`
	Source  string `json:"source"`
	Reason  string `json:"reason,omitempty"`
	Subject string `json:"subject"`
	Object  string `json:"object"`
	Action  string `json:"action"`
	TraceID string `json:"traceId,omitempty"`
}

type Service interface {
	Check(ctx context.Context, req CheckRequest) (*Decision, error)
	BatchCheck(ctx context.Context, reqs []CheckRequest) ([]Decision, error)
}
