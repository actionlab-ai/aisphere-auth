# Go SDK 使用说明

`pkg/aisphereauth` 是给 SkillHub、AgentRuntime、SQLHub、ModelGateway 等 Go 服务直接复用的公共 SDK。业务服务不应该直接 import `internal/*`。

## HTTP Client

```go
package main

import (
    "context"
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

## Audit 审计写入

统一审计能力已经进入 SDK。SkillHub、AgentRuntime、SQLHub、ModelGateway 后续都可以通过同一个 Client 写审计事件。

业务 handler 中推荐使用 `authgin.NewAuditEvent` 从当前请求自动带出登录人、IP、User-Agent、请求路径和 traceId：

```go
func CreateSkill(c *gin.Context) {
    // ...业务创建逻辑...

    event := authgin.NewAuditEvent(
        c,
        "skill",
        skillID,
        "skill.create",
        aisphereauth.AuditResultSuccess,
    )
    event.Reason = "create skill success"
    event.Metadata = map[string]string{
        "skillName": req.Name,
    }

    if _, err := authClient.WriteAudit(c.Request.Context(), event); err != nil {
        // 审计失败一般不应该阻断主业务，但必须打日志。
        slog.Warn("write audit failed", "error", err)
    }
}
```

也可以手工构造：

```go
_, err := authClient.WriteAudit(ctx, aisphereauth.AuditEvent{
    TraceID:       traceID,
    ActorSubject:  "aisphere/admin",
    ActorName:     "admin",
    App:           "skillhub",
    ResourceType:  "skill",
    ResourceID:    skillID,
    Action:        "skill.publish",
    Result:        aisphereauth.AuditResultSuccess,
    IP:            clientIP,
    UserAgent:     userAgent,
    RequestPath:   "/api/skills/" + skillID + "/publish",
    RequestMethod: "POST",
})
```

审计事件字段约定：

```text
actorSubject   操作人主体，例如 aisphere/admin
actorName      操作人显示名
app            业务系统，例如 skillhub
resourceType   资源类型，例如 skill / group / proposal / release
resourceId     资源 ID
action         操作动作，例如 skill.create / proposal.approve
result         success / failure / allow / deny
reason         失败原因或补充说明
traceId        请求链路 ID
metadata       业务自定义扩展字段
```

## Audit 查询

```go
resp, err := authClient.ListAudit(ctx, aisphereauth.AuditListRequest{
    App:          "skillhub",
    ActorSubject: "aisphere/admin",
    ResourceType: "skill",
    Limit:        50,
})
if err != nil {
    return err
}
for _, event := range resp.Items {
    fmt.Println(event.Action, event.Result, event.CreatedAt)
}
```

当前服务端默认是内存审计存储，适合本地联调和第一阶段集成；生产落库版下一步可以把 `internal/audit.Service` 换成 MySQL/Redis/ES 实现，SDK 合约不需要再改。

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
