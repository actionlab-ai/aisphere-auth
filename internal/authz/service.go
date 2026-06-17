package authz

import (
	"context"

	"github.com/actionlab-ai/aisphere-auth/internal/principal"
)

type CheckRequest struct {
	Principal *principal.Principal
	Subject   string
	Object    string
	Action    string
	App       string
	TraceID   string
}

type Decision struct {
	Allow   bool
	Source  string
	Reason  string
	Subject string
	Object  string
	Action  string
}

type Service interface {
	Check(ctx context.Context, req CheckRequest) (*Decision, error)
	BatchCheck(ctx context.Context, reqs []CheckRequest) ([]Decision, error)
}
