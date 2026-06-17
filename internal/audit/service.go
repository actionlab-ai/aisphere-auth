package audit

import (
	"context"

	"github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth"
)

type Service interface {
	Write(ctx context.Context, event aisphereauth.AuditEvent) (*aisphereauth.AuditEvent, error)
	List(ctx context.Context, req aisphereauth.AuditListRequest) (*aisphereauth.AuditListResponse, error)
	Ping(ctx context.Context) error
	Close() error
}
