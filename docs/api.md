# AI Sphere Auth API Draft

## Health

```http
GET /healthz
GET /readyz
```

## Login

```http
GET /auth/login?app=skillhub&redirect=/skillhub
GET /auth/callback/casdoor?code=xxx&state=xxx
```

## Current principal

```http
GET /auth/me
Cookie: aisphere_session=xxx
```

## Logout

```http
POST /auth/logout
POST /auth/logout?global=true
```

## Internal session introspection

```http
POST /auth/sessions/introspect
Content-Type: application/json

{
  "sessionId": "sess_xxx",
  "app": "skillhub"
}
```

## Authorization check

```http
POST /authz/check
Content-Type: application/json

{
  "subject": "skillhub/admin",
  "object": "skillhub:skill:*",
  "action": "admin:read"
}
```

## Batch authorization check

```http
POST /authz/batch-check
Content-Type: application/json

{
  "checks": [
    {"object": "skillhub:skill:*", "action": "admin:read"},
    {"object": "skillhub:proposal:*", "action": "approve"}
  ]
}
```
