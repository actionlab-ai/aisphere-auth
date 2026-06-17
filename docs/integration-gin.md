# Gin Integration Draft

Business services should depend on `pkg/aisphereauth` instead of importing Casdoor directly.

Example:

```go
r.GET("/v3/admin/skills",
    authgin.RequireLogin(authClient, authgin.MiddlewareOptions{
        App: "skillhub",
        CookieName: "aisphere_session",
    }),
    authgin.RequirePermission(authClient, "skillhub:skill:*", "admin:read"),
    handler.AdminListSkills,
)
```

The middleware flow is:

1. Read `aisphere_session` from cookie.
2. Call `aisphere-auth` session introspection.
3. Store normalized Principal in `gin.Context`.
4. On protected routes, call `aisphere-auth` authorization check.
