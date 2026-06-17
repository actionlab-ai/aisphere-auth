# AI Sphere Auth

AI Sphere 统一认证授权服务。

`aisphere-auth` 是 AI Sphere 平台的共享 AuthN/AuthZ 层，用于统一接入 SkillHub、AgentRuntime、SQLHub、ModelGateway、Portal 等组件。Casdoor 负责账号、组织、角色、权限策略；`aisphere-auth` 负责平台侧 Session、Principal 标准化、权限检查封装和业务 SDK 接入。

## 当前能力

- Gin HTTP Server
- Cobra 中文 CLI 帮助：`aisphere-auth -h`
- Viper 配置加载：命令行参数 > 环境变量 > 配置文件 > 默认值
- `/healthz`、轻量 `/livez` 和真实依赖检查的 `/readyz`
- Casdoor OAuth 登录 URL 生成和 callback 处理
- AI Sphere Session，支持 memory 或 Redis 存储
- Redis login state store
- `aisphere_session` HttpOnly Cookie，支持 SameSite / Secure / Domain
- `/auth/me`、`/auth/logout`
- service token 保护的 `/auth/sessions/introspect`、`/authz/check`、`/authz/batch-check`
- 内部 API 限流
- 短 TTL AuthZ decision cache
- 公共 `pkg/aisphereauth` HTTP Client 和 Gin Middleware
- Gin SDK 本地 introspect 缓存，降低每请求 RPC 的 N+1 问题
- SDK 自定义 401/403/error 响应钩子
- OpenAPI 契约：`api/openapi.yaml`，便于 Python / Java / JS 生成客户端
- GitHub Actions CI
- 离线 `.run` 构建和安装能力
- Casdoor 项目专用 Seed SQL 生成和导入能力

## 一、快速启动：已有 Casdoor 的场景

如果你已经有一个可用的 Casdoor 服务，推荐按下面顺序启动。

### 1. 先初始化 Casdoor 项目配置

如果 Casdoor 里还没有 AI Sphere / SkillHub 相关 Organization、Application、Model、Permission、Role，不要用手工 UI 点点点，直接用项目专用 seed 生成器。

生成 SQL：

```bash
python3 scripts/casdoor/render-casdoor-seed.py \
  --output deployments/casdoor/sql/aisphere-auth-casdoor.sql \
  --org aisphere \
  --app aisphere-auth \
  --client-id aisphere-auth \
  --client-secret '<替换为真实 OAuth Client Secret>' \
  --redirect-uri http://127.0.0.1:18080/auth/callback/casdoor \
  --admin-user admin
```

导入到 Casdoor MySQL：

```bash
bash scripts/casdoor/import-casdoor-sql.sh \
  --host 127.0.0.1 \
  --port 3306 \
  --database casdoor \
  --user root \
  --password '<Casdoor MySQL 密码>' \
  --sql deployments/casdoor/sql/aisphere-auth-casdoor.sql \
  --backup-before \
  -y
```

Windows PowerShell：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\casdoor\import-casdoor-sql.ps1 `
  -HostName 127.0.0.1 `
  -Port 3306 `
  -Database casdoor `
  -User root `
  -Password '<Casdoor MySQL 密码>' `
  -SqlFile .\deployments\casdoor\sql\aisphere-auth-casdoor.sql `
  -BackupBefore `
  -Yes
```

seed 会创建或更新：

```text
Organization: aisphere
Application:  aisphere-auth
Model:        aisphere-auth-model
Roles:        platform / skillhub / agentruntime / sqlhub / modelgateway / portal 常用角色
Permissions:  对应资源前缀和动作的常用策略
Binding:      默认把 aisphere/admin 绑定到 role_platform_admin
```

这个能力和历史 dump 提取不同：它是针对当前项目的标准开箱配置，不依赖某个环境的旧数据，也不会迁移 token/session/record 等运行态表。

### 2. 复制配置文件

```bash
cp configs/config.yaml.example configs/config.yaml
```

重点修改：

```yaml
server:
  addr: ":18080"
  mode: "release"
  publicBaseURL: "http://127.0.0.1:18080"

casdoor:
  endpoint: "http://127.0.0.1:8008"
  owner: "aisphere"
  application: "aisphere-auth"
  clientId: "aisphere-auth"
  clientSecret: "与 render-casdoor-seed.py 使用的 Client Secret 一致"
  redirectURL: "http://127.0.0.1:18080/auth/callback/casdoor"
  permissionId: "aisphere/perm_platform_admin"
  subjectFormat: "owner-name"

session:
  provider: "redis"
  cookieName: "aisphere_session"
  ttlSeconds: 28800
  sliding: true
  redis:
    addrs: ["127.0.0.1:6379"]
    prefix: "aisphere"

internal:
  serviceTokenRequired: true
  serviceTokenHeader: "X-Aisphere-Service-Token"
  serviceToken: "请替换成长随机字符串，至少 32 位"
```

### 3. 检查配置

```bash
./aisphere-auth check-config --config configs/config.yaml
```

打印最终合并后的配置，敏感字段会脱敏：

```bash
./aisphere-auth --config configs/config.yaml --print-config
```

### 4. 启动

源码启动：

```bash
go run ./cmd/server --config configs/config.yaml
```

二进制启动：

```bash
./aisphere-auth --config configs/config.yaml
```

健康检查：

```bash
curl http://127.0.0.1:18080/healthz
curl http://127.0.0.1:18080/livez
curl http://127.0.0.1:18080/readyz
```

`/readyz` 会真实检查 Redis 和 Casdoor。Casdoor 或 Redis 不通时返回 `503` 是正常行为。

### 5. 登录验证

浏览器访问：

```text
http://127.0.0.1:18080/auth/login?app=skillhub&redirect=/
```

正常流程：

```text
1. 跳转到 Casdoor 登录页
2. 登录成功后回调 /auth/callback/casdoor
3. aisphere-auth 写入 aisphere_session Cookie
4. 跳回 redirect 地址
```

## 二、Casdoor 初始化方式说明

### 推荐：项目专用 Seed

适合新环境、已有 Casdoor 但没有配置 AI Sphere 的场景：

```bash
bash scripts/casdoor/import-casdoor-sql.sh \
  --seed \
  --seed-org aisphere \
  --seed-app aisphere-auth \
  --seed-client-id aisphere-auth \
  --seed-client-secret '<OAuth Client Secret>' \
  --seed-redirect-uri http://127.0.0.1:18080/auth/callback/casdoor \
  --host 127.0.0.1 \
  --port 3306 \
  --database casdoor \
  --user root \
  --password '<Casdoor MySQL 密码>' \
  --backup-before \
  -y
```

详细说明见：[docs/casdoor-bootstrap-seed.md](docs/casdoor-bootstrap-seed.md)。

### 保留：从历史 dump 提取

如果你已经有一个配置好的参考 Casdoor，并且确实要迁移其中的配置，可以继续用：

```bash
python3 scripts/casdoor/prepare-casdoor-sql.py \
  --input ./casdoor.sql \
  --output deployments/casdoor/sql/aisphere-auth-casdoor.sql \
  --mode data-only \
  --keywords aisphere,skillhub
```

这个方式依赖历史环境命名，不是开箱即用初始化的默认方案。

## 三、业务系统 Go SDK 接入

Go 服务优先使用：

```text
pkg/aisphereauth/client
pkg/aisphereauth/gin
```

完整说明见：[docs/go-sdk.md](docs/go-sdk.md)。

最小接入示例：

```go
authClient := client.NewHTTPClient(
    "http://aisphere-auth:18080",
    client.WithServiceToken(os.Getenv("AISPHERE_SERVICE_TOKEN")),
)

r.Use(authgin.RequireLogin(authClient, authgin.MiddlewareOptions{
    App: "skillhub",
    CacheTTL: 5 * time.Second,
}))

r.GET("/v3/admin/skills",
    authgin.RequirePermission(authClient, "skillhub:skill:*", "admin:read"),
    handler.ListSkills,
)
```

非 Go 语言接入使用 OpenAPI：

```text
api/openapi.yaml
```

## 四、CLI 用法

显示中文帮助：

```bash
./aisphere-auth -h
```

指定配置文件启动：

```bash
./aisphere-auth --config configs/config.yaml
```

校验配置：

```bash
./aisphere-auth check-config --config configs/config.yaml
```

查看版本：

```bash
./aisphere-auth version
```

## 五、离线 `.run` 交付

本地构建：

```bash
bash build.sh --arch amd64
bash build.sh --arch arm64
bash build.sh --arch all
```

离线安装到默认内网仓库：

```bash
./dist/aisphere-auth-0.1.0-amd64.run install -y \
  --registry sealos.hub:5000 \
  --namespace aisphere-system
```

切换其他内网仓库：

```bash
./dist/aisphere-auth-0.1.0-amd64.run install -y \
  --registry harbor.local:5000 \
  --namespace aisphere-system
```

## 六、设计边界

Casdoor 仍然是账号、组织、角色、Permission、Policy 的来源。AI Sphere Auth 只做平台侧 Session、Principal 归一化、权限检查封装、服务端 SDK、OpenAPI 契约和离线交付。
