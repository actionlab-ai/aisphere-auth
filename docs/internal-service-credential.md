# Internal Service Credential

`aisphere-auth` exposes several internal APIs for trusted AI Sphere services:

- `POST /auth/sessions/introspect`
- `POST /authz/check`
- `POST /authz/batch-check`

These APIs are used by SkillHub, AgentRuntime, SQLHub, ModelGateway and Portal backends. They must not be exposed as unauthenticated public APIs in production.

## Environment variables

```bash
export AISPHERE_SERVICE_TOKEN_REQUIRED=true
export AISPHERE_SERVICE_TOKEN='replace-with-long-random-secret'
export AISPHERE_SERVICE_TOKEN_HEADER='X-Aisphere-Service-Token'
```

If `AISPHERE_SERVICE_TOKEN_REQUIRED=true` or `AISPHERE_SERVICE_TOKEN` is non-empty, the server requires the credential for protected internal APIs.

## Header format

Preferred:

```http
X-Aisphere-Service-Token: <token>
```

Also supported for simple integrations:

```http
Authorization: Bearer <token>
```

## SDK usage

```go
package main

import (
    "os"

    authclient "github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth/client"
)

func newAuthClient() *authclient.HTTPClient {
    return authclient.NewHTTPClient(
        "http://aisphere-auth:18080",
        authclient.WithServiceToken(os.Getenv("AISPHERE_SERVICE_TOKEN")),
    )
}
```

## Curl example

```bash
curl -X POST http://127.0.0.1:18080/authz/check \
  -H 'Content-Type: application/json' \
  -H "X-Aisphere-Service-Token: $AISPHERE_SERVICE_TOKEN" \
  -d '{"subject":"skillhub/admin","object":"skillhub:skill:*","action":"admin:read"}'
```

## Development mode

For local demo only, the service credential can be disabled:

```bash
export AISPHERE_SERVICE_TOKEN_REQUIRED=false
unset AISPHERE_SERVICE_TOKEN
```

Production should always enable it, and the token should be generated as a long random secret and distributed to trusted services through Kubernetes Secret, systemd EnvironmentFile, Vault or another secret manager.
