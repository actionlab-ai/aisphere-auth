# AI Sphere Auth

AI Sphere unified authentication and authorization service.

`aisphere-auth` is the shared AuthN/AuthZ layer for AI Sphere platforms such as SkillHub, AgentRuntime, SQLHub, ModelGateway and Portal.

## Current milestone

This repository now includes the first runnable auth service implementation:

- Gin HTTP server
- `/healthz` and `/readyz`
- Casdoor OAuth login URL generation
- Casdoor callback handling
- AI Sphere memory session
- `aisphere_session` HttpOnly cookie
- `/auth/me`
- `/auth/logout`
- `/auth/sessions/introspect`
- `/authz/check`
- `/authz/batch-check`
- public `pkg/aisphereauth` HTTP client and Gin middleware skeleton

## Run locally

```bash
go mod tidy
go run ./cmd/server
```

Health check:

```bash
curl http://127.0.0.1:18080/healthz
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

## Design boundary

Casdoor remains the source of users, roles and policies. AI Sphere Auth owns local platform sessions, principal normalization and the reusable service/SDK boundary for business services.
