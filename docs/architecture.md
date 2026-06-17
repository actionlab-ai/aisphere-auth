# AI Sphere Auth Architecture

`aisphere-auth` is the shared authentication and authorization service for AI Sphere.

## Responsibility split

- Casdoor: identity source, organizations, roles, permissions and policy.
- AI Sphere Auth Service: login callback, AI Sphere session, normalized principal, token issuing and authorization check facade.
- Gateway: routing, TLS, CORS, cookie boundary, request ID and coarse security controls.
- Business services: SkillHub, AgentRuntime, SQLHub and ModelGateway define their own resources and actions.

## Session model

The platform uses two layers:

1. Casdoor session, owned by Casdoor, provides SSO login state.
2. AI Sphere session, owned by `aisphere-auth`, provides platform business login state.

Web console traffic should use an HttpOnly `aisphere_session` cookie. CLI, Agent and service-to-service traffic should use short-lived bearer tokens in later milestones.

## Authorization model

Authorization uses three values:

```text
sub, obj, act
```

Example:

```text
sub = skillhub/admin
obj = skillhub:skill:*
act = admin:read
```

`aisphere-auth` does not own policy. It calls Casdoor enforce and returns a normalized decision.
