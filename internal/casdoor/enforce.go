package casdoor

import "context"

type EnforceRequest struct {
	PermissionID string
	Sub          string
	Obj          string
	Act          string
}

type EnforceResponse struct {
	Allow bool
}

type Enforcer interface {
	Enforce(ctx context.Context, req EnforceRequest) (*EnforceResponse, error)
}
