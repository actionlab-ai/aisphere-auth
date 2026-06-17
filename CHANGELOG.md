# Changelog

All notable changes to `aisphere-auth` will be documented in this file.

The project follows semantic versioning after the first tagged release. Before `v1.0.0`, minor versions may include SDK API adjustments, but breaking changes should still be called out here.

## [0.1.0] - Unreleased

### Added

- Casdoor OAuth login and callback flow.
- Memory and Redis session stores.
- Redis OAuth state store.
- Service-token-protected internal APIs.
- Public Go SDK under `pkg/aisphereauth`.
- Gin middleware under `pkg/aisphereauth/gin`.
- Short TTL local introspection cache in Gin middleware.
- SDK sentinel errors such as `ErrInactiveSession` and `ErrPermissionDenied`.
- SDK HTTP options for timeout, service token header, error body limit and instrumentation hook.
- Audit event contract, audit HTTP endpoints and Go SDK audit write/query methods.
- Gin helper for building audit events from the current request and principal.
- OpenAPI contract at `api/openapi.yaml` for non-Go SDK generation.
- Offline `.run` package build and install flow.
- Casdoor seed SQL renderer and import helpers.

### Changed

- Gin middleware now supports custom unauthorized/forbidden/error handlers.
- SDK `Decision` and `CheckRequest` include `traceId` for cross-service tracing.
- SDK error bodies are capped to avoid huge error messages.

### Security

- Internal APIs can be protected with service tokens and rate limits.
- Session cookies support `SameSite` and secure cookie settings.
- Redirect validation rejects absolute URLs, backslashes and control characters.
