# AI Sphere Auth Implementation Checklist

This checklist tracks the platform-stability-first path for `aisphere-auth`.

## Done

- Gin HTTP server
- `healthz` and `readyz`
- Casdoor OAuth login URL generation
- Casdoor callback handling through HTTP adapter
- Principal normalization
- Memory session store
- Redis session store
- Memory OAuth state store
- Redis OAuth state store
- `aisphere_session` HttpOnly cookie
- `/auth/me`
- `/auth/logout`
- `/auth/sessions/introspect`
- `/authz/check`
- `/authz/batch-check`
- AuthZ short TTL cache
- Service credential middleware for internal APIs
- SDK HTTP client with service token support
- Gin middleware skeleton
- Docker Compose with Redis
- GitHub Actions CI

## P0: must-have before SkillHub production integration

- Confirm CI passes on `main`
- Add integration tests for:
  - session create/get/delete
  - service token middleware
  - SDK client header injection
  - `/auth/sessions/introspect`
  - `/authz/check`
- Harden Redis readiness checks in `/readyz`
- Add consistent error response format
- Improve Gin SDK:
  - cookie extraction
  - optional bearer extraction
  - `RequirePermissionFunc`
  - consistent 401/403 JSON responses
- Add deployment examples for Kubernetes and systemd

## P1: platform integration

- Add Casdoor Go SDK adapter beside HTTP fallback
- Add Casdoor config-as-code scripts:
  - export
  - apply
  - verify
- Integrate SkillHub first:
  - protect `/v3/admin/skills`
  - validate 401 / 403 / 200
  - validate Casdoor policy changes take effect
- Add AgentRuntime permission matrix draft

## P2: token and resource-server mode

- JWT issuer
- JWT verifier
- API token issue / revoke
- JWK Set endpoint
- token introspection
- token revocation
- optional local JWT verification in `pkg/aisphereauth`

## Later

- Gateway header signature mode
- Owner / maintainer relation permissions
- Approval
- Capability token
- MCP Tool Guard
- Multi-language SDKs
