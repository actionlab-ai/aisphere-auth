# AI Sphere Auth

AI Sphere unified authentication and authorization service.

`aisphere-auth` is the shared AuthN/AuthZ layer for AI Sphere platforms such as SkillHub, AgentRuntime, SQLHub, ModelGateway and Portal.

## Current milestone

This repository now includes the first runnable auth service implementation:

- Gin HTTP server
- Chinese CLI help with Cobra: `aisphere-auth -h`
- Viper config loading: flags > env > config file > defaults
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
- offline `.run` package builder and installer

## CLI

Show Chinese help:

```bash
./aisphere-auth -h
```

Run with config file:

```bash
./aisphere-auth --config configs/config.yaml
```

Run with command-line overrides:

```bash
./aisphere-auth \
  --config configs/config.yaml \
  --addr :18080 \
  --mode release \
  --session-provider redis \
  --redis-addrs 127.0.0.1:6379
```

Check config without starting the server:

```bash
./aisphere-auth check-config --config configs/config.yaml
```

Print final merged config with secrets masked:

```bash
./aisphere-auth --config configs/config.yaml --print-config
```

Show version:

```bash
./aisphere-auth version
```

## Configuration

Config precedence:

```text
command-line flags > environment variables > config file > defaults
```

Copy the annotated sample config:

```bash
cp configs/config.yaml.example configs/config.yaml
```

The sample file contains Chinese comments for every field, including how to fill Casdoor, Redis, Cookie, Service Token and JWT-related settings.

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

## Offline `.run` package

Build offline packages locally:

```bash
bash build.sh --arch amd64
bash build.sh --arch arm64
bash build.sh --arch all
```

Install into an offline Kubernetes environment and push images to the target registry:

```bash
./dist/aisphere-auth-0.1.0-amd64.run install -y \
  --registry sealos.hub:5000 \
  --namespace aisphere-system
```

Use another registry:

```bash
./dist/aisphere-auth-0.1.0-amd64.run install -y \
  --registry 10.10.10.10:5000 \
  --namespace aisphere-system
```

Render only:

```bash
./dist/aisphere-auth-0.1.0-amd64.run install -y \
  --registry sealos.hub:5000 \
  --skip-push \
  --skip-apply \
  --output-dir ./out
```

More details: [docs/offline-run.md](docs/offline-run.md).

## GitHub Actions

- `.github/workflows/ci.yml` runs `gofmt`, `go vet` and `go test ./...`.
- `.github/workflows/offline-run.yml` builds `amd64` and `arm64` `.run` packages.
- Tag pushes matching `v*` attach `.run` and `.sha256` files to GitHub Release.

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
