# AI Sphere Auth

AI Sphere unified authentication and authorization service.

This repository will provide:

- `aisphere-auth-service`: centralized login, session, principal and authorization check service.
- `pkg/aisphereauth`: Go SDK and Gin middleware for AI Sphere services.
- Casdoor adapter layer for identity, roles and policy enforcement.

## First milestone

Milestone 0/1 bootstraps the service skeleton:

- Go module
- Gin HTTP server
- config loading
- health and readiness endpoints
- core domain interfaces for authn/authz/session/casdoor
- public SDK type placeholders
