# AI Sphere Auth

AI Sphere unified authentication and authorization service.

`aisphere-auth` is the shared AuthN/AuthZ layer for AI Sphere platforms such as SkillHub, AgentRuntime, SQLHub, ModelGateway and Portal.

## Current milestone

This repository now includes the first runnable auth service implementation:

- Gin HTTP server
- `/healthz` and `/readyz`
- Casdoor OAuth login URL generation
- Casdoor callback handling
- AI Sphere session with memory or Redis store
- Redis login state store when Redis session mode is enabled
- `aisphere_session` HttpOnly cookie
- `/auth/me`
- `/auth/logout`
- service-protected `/auth/sessions/introspect`
- service-protected `/authz/check`
- service-protected `/authz/batch-check`
- short-TTL in-memory authz decision cache
- public `pkg/aisphereauth` HTTP client and Gin middleware skeleton
- GitHub Actions CI for `gofmt`, `go vet` and `go test ./...`

## Run locally with memory session

```bash
go mod tidy
go run ./cmd/server
```

Health check:

```bash
curl http://127.0.0.1:18080/healthz
curl http://127.0.0.1:18080/readyz
```

## Run locally with Redis session

```bash
export AISPHERE_SESSION_PROVIDER="redis"
export AISPHERE_REDIS_ADDRS="127.0.0.1:6379"
export AISPHERE_REDIS_PREFIX="aisphere"
go run ./cmd/server
```

Or use docker compose:

```bash
cd deployments/docker
docker compose up --build
```

## Casdoor environment

```bash
export AISPHERE_CASDOOR_ENDPOINT="http://127.0.0.1:8000"
export AISPHERE_CASDOOR_OWNER="skillhub"
export AISPHERE_CASDOOR_APPLICATION="aisphere"
export AISPHERE_CASDOOR_CLIENT_ID="your-client-id"
export AISPHERE_CASDOOR_CLIENT_SECRET="your-client-secret"
export AISPHERE_CASDOOR_REDIRECT_URL="http://127.0.0.1:18080/auth/callback/casdoor"
export AISPHERE_CASDOOR_PERMISSION_ID="skillhub/platform_permission"
```

Login test:

```bash
open 'http://127.0.0.1:18080/auth/login?app=skillhub&redirect=/'
```

## Internal service credential

`SkillHub`, `AgentRuntime`, `SQLHub` and other trusted components call internal APIs such as `/auth/sessions/introspect` and `/authz/check`. Enable the service credential before exposing the service beyond local development:

```bash
export AISPHERE_SERVICE_TOKEN_REQUIRED=true
export AISPHERE_SERVICE_TOKEN='replace-with-long-random-secret'
export AISPHERE_SERVICE_TOKEN_HEADER='X-Aisphere-Service-Token'
```

SDK usage:

```go
client := client.NewHTTPClient(
    "http://aisphere-auth:18080",
    client.WithServiceToken(os.Getenv("AISPHERE_SERVICE_TOKEN")),
)
```

HTTP usage:

```bash
curl -X POST http://127.0.0.1:18080/authz/check \
  -H 'Content-Type: application/json' \
  -H "X-Aisphere-Service-Token: $AISPHERE_SERVICE_TOKEN" \
  -d '{"subject":"skillhub/admin","object":"skillhub:skill:*","action":"admin:read"}'
```

## Design boundary

Casdoor remains the source of users, roles and policies. AI Sphere Auth owns local platform sessions, principal normalization and the reusable service/SDK boundary for business services.

## Next milestones

1. Harden Redis readiness checks and session sliding TTL behavior.
2. Add `casdoor-go-sdk` adapter beside the current HTTP fallback.
3. Add API token / JWT issuance for AgentRuntime, CLI and service-to-service calls.
4. Add Casdoor config-as-code scripts for model, permission, role and policy initialization.
5. Start SkillHub integration using `pkg/aisphereauth/gin` middleware.
