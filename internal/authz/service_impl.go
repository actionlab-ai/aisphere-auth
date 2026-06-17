package authz

import (
	"context"
	"errors"
	"strings"

	"github.com/actionlab-ai/aisphere-auth/internal/casdoor"
	"github.com/actionlab-ai/aisphere-auth/internal/config"
)

var ErrMissingPermission = errors.New("missing subject/object/action")

type DefaultService struct {
	cfg      config.Config
	enforcer casdoor.Enforcer
}

func NewDefaultService(cfg config.Config, enforcer casdoor.Enforcer) *DefaultService {
	return &DefaultService{cfg: cfg, enforcer: enforcer}
}

func (s *DefaultService) Check(ctx context.Context, req CheckRequest) (*Decision, error) {
	subject := strings.TrimSpace(req.Subject)
	if subject == "" && req.Principal != nil {
		subject = req.Principal.EffectiveSubject()
	}
	object := strings.TrimSpace(req.Object)
	action := strings.TrimSpace(req.Action)
	decision := &Decision{Source: "casdoor", Subject: subject, Object: object, Action: action}
	if subject == "" || object == "" || action == "" {
		decision.Reason = ErrMissingPermission.Error()
		return decision, ErrMissingPermission
	}
	resp, err := s.enforcer.Enforce(ctx, casdoor.EnforceRequest{PermissionID: s.cfg.Casdoor.PermissionID, Sub: subject, Obj: object, Act: action})
	if err != nil {
		decision.Reason = err.Error()
		return decision, err
	}
	decision.Allow = resp != nil && resp.Allow
	if !decision.Allow {
		decision.Reason = "casdoor_denied"
	}
	return decision, nil
}

func (s *DefaultService) BatchCheck(ctx context.Context, reqs []CheckRequest) ([]Decision, error) {
	out := make([]Decision, 0, len(reqs))
	for _, req := range reqs {
		decision, err := s.Check(ctx, req)
		if decision != nil {
			out = append(out, *decision)
		}
		if err != nil && s.cfg.Authz.FailClosed {
			return out, err
		}
	}
	return out, nil
}
