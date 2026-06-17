# Go SDK 使用说明

`pkg/aisphereauth` 是给 SkillHub、AgentRuntime、SQLHub、ModelGateway 等 Go 服务直接复用的公共 SDK。业务服务不应该直接 import `internal/*`。

## HTTP Client

```go
package main

import (
    "os"
    "time"

    authclient "github.com/actionlab-ai/aisphere-auth/pkg/aisphereauth/client"
)

func newAuthClient() *authclient.HTTPClient {
    return authclient.NewHTTPClient(
        "http://aisphere-auth:18080",
        authclient.WithServiceToken(os.Getenv("AISPHERE_SERVICE_TOKEN")),
        authclient.WithTimeout(3*time.Second),
        authclient.WithMaxErrorBodyBytes(4096),
        authclient.WithHook(func(ctx context.Context, e authclient.HookEvent) {
            // 在这里记录 SDK RPC 延迟、状态码和错误率。
        }),
    )
}
```

## 登录跳转 URL

```go
loginURL := authClient.LoginURL("skillhub", "/v3/admin/skills")
```

生成：

```text
http://aisphere-auth:18080/auth/login?app=skillhub&redirect=%2Fv3%2Fadmin%2Fskills
```

## Gin 中间件

```go
r := gin.New()

authClient := authclient.NewHTTPClient(
    "http://aisphere-auth:18080",
    authclient.WithServiceToken(os.Getenv("AISPHERE_SERVICE_TOKEN")),
)

// RequireLogin 默认会缓存 introspect 结果 5 秒，避免每个业务请求都 RPC 到 auth-service。
r.Use(authgin.RequireLogin(authClient, authgin.MiddlewareOptions{
    App: "skillhub",
    CookieName: "aisphere_session",
    CacheTTL: 5 * time.Second,
}))

r.GET("/v3/admin/skills",
    authgin.RequirePermission(authClient, "skillhub:skill:*", "admin:read"),
    handler.ListSkills,
)
```

## 自定义错误响应

业务服务可以使用自己的响应格式：

```go
authgin.RequireLogin(authClient, authgin.MiddlewareOptions{
    App: "skillhub",
    OnUnauthorized: func(c *gin.Context, err error) {
        c.JSON(401, gin.H{"code": 401, "msg": "请先登录"})
    },
})
```

权限不足：

```go
authgin.RequirePermission(authClient, "skillhub:skill:*", "admin:delete", authgin.MiddlewareOptions{
    OnForbidden: func(c *gin.Context, err error) {
        c.JSON(403, gin.H{"code": 403, "msg": "无操作权限"})
    },
})
```

## 中间件顺序

推荐顺序：

```go
r.Use(authgin.RequireLogin(authClient, opts))
r.GET("/path", authgin.RequirePermission(authClient, object, action), handler)
```

`RequirePermission` 在找不到 Principal 时会返回 401，不会 panic，但业务上仍应先挂 `RequireLogin`。

## 本地缓存说明

`RequireLogin` 默认对同一个 `sessionID + app` 的 introspect 结果缓存 5 秒，用于降低 N+1 RPC 问题。

可以关闭：

```go
authgin.RequireLogin(authClient, authgin.MiddlewareOptions{
    App: "skillhub",
    DisableCache: true,
})
```

或者调整 TTL：

```go
authgin.RequireLogin(authClient, authgin.MiddlewareOptions{
    App: "skillhub",
    CacheTTL: 10 * time.Second,
})
```

## Sentinel errors

SDK 暴露了可判断的错误：

```go
if errors.Is(err, aisphereauth.ErrInactiveSession) {
    // session 不存在、过期或 app 不匹配
}

if errors.Is(err, aisphereauth.ErrPermissionDenied) {
    // 权限拒绝
}
```

HTTP 非 2xx 会返回 `*client.APIError`，其 `Body` 已限制长度，避免 HTML 错误页导致日志爆炸。

## 非 Go 语言接入

OpenAPI 描述文件位于：

```text
api/openapi.yaml
```

Python / Java / JavaScript 服务可以基于该文件生成客户端。
