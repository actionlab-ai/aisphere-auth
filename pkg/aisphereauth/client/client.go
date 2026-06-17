package client

import (
	"context"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
)

type CheckRequest struct {
	Subject string `json:"subject,omitempty"`
	Object  string `json:"object"`
	Action  string `json:"action"`
	App     string `json:"app,omitempty"`
}

type Decision struct {
	Allow   bool   `json:"allow"`
	Source  string `json:"source"`
	Reason  string `json:"reason,omitempty"`
	Subject string `json:"subject"`
	Object  string `json:"object"`
	Action  string `json:"action"`
}

type Client interface {
	Introspect(ctx context.Context, sessionID string, app string) (*aisphereauth.Principal, error)
	Check(ctx context.Context, req CheckRequest) (*Decision, error)
	BatchCheck(ctx context.Context, reqs []CheckRequest) ([]Decision, error)
}
