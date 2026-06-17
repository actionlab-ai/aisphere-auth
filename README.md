# AI Sphere Auth

AI Sphere 统一认证授权服务。

`aisphere-auth` 是 AI Sphere 平台的共享 AuthN/AuthZ 层，用于统一接入 SkillHub、AgentRuntime、SQLHub、ModelGateway、Portal 等组件。Casdoor 仍然负责账号、组织、角色、权限策略；`aisphere-auth` 负责平台侧 Session、Principal 标准化、权限检查封装和业务 SDK 接入。

## 当前能力

当前仓库已经包含第一版可运行的认证授权服务：

- Gin HTTP Server
- Cobra 中文 CLI 帮助：`aisphere-auth -h`
- Viper 配置加载：命令行参数 > 环境变量 > 配置文件 > 默认值
- `/healthz` 和 `/readyz`
- Casdoor OAuth 登录 URL 生成
- Casdoor callback 处理
- AI Sphere Session，支持 memory 或 Redis 存储
- Redis login state store
- `aisphere_session` HttpOnly Cookie
- Cookie SameSite / Secure / Domain 配置
- `/auth/me`
- `/auth/logout`
- service token 保护的 `/auth/sessions/introspect`
- service token 保护的 `/authz/check`
- service token 保护的 `/authz/batch-check`
- 短 TTL AuthZ decision cache
- 公共 `pkg/aisphereauth` HTTP Client 和 Gin Middleware 骨架
- GitHub Actions CI：`gofmt`、`go vet`、`go test ./...`
- 离线 `.run` 构建和安装能力
- Casdoor SQL 自动导入脚本，避免手工 UI 点点点

## 一、快速启动：已有 Casdoor 的场景

如果你已经有一个可用的 Casdoor 服务，推荐按下面顺序启动。

### 1. 准备 Casdoor 配置

你需要拿到这些信息：

```text
Casdoor Endpoint，例如：http://36.138.61.152:8008
Casdoor Application Client ID
Casdoor Application Client Secret
Casdoor Redirect URL，例如：http://127.0.0.1:18080/auth/callback/casdoor
Casdoor Permission ID，例如：skillhub/platform_permission
```

如果 Casdoor 里的 Application、Model、Permission、Policy 已经配置好，可以直接进入下一步。

如果还没有配置，可以先用 SQL 导入方式初始化，见下面“二、Casdoor SQL 自动导入”。

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
  endpoint: "http://36.138.61.152:8008"
  owner: "skillhub"
  application: "aisphere"
  clientId: "你的 Casdoor Application Client ID"
  clientSecret: "你的 Casdoor Application Client Secret"
  redirectURL: "http://127.0.0.1:18080/auth/callback/casdoor"
  permissionId: "skillhub/platform_permission"
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
  serviceToken: "请替换成长随机字符串"
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
curl http://127.0.0.1:18080/readyz
```

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

## 二、Casdoor SQL 自动导入

如果你不想手工在 Casdoor UI 中点 Application、Model、Permission、Role、Policy，可以使用 SQL 自动导入。

推荐流程：

```text
1. 在一个参考 Casdoor 环境中完成一次正确配置。
2. 从参考环境导出与 AI Sphere Auth 相关的 SQL。
3. 脱敏检查后保存到 deployments/casdoor/sql/aisphere-auth-casdoor.sql。
4. 在新环境中执行 import-casdoor-sql 脚本导入。
5. 启动 aisphere-auth。
```

默认 SQL 文件路径：

```text
deployments/casdoor/sql/aisphere-auth-casdoor.sql
```

Linux / macOS：

```bash
bash scripts/casdoor/import-casdoor-sql.sh \
  --host 36.138.61.152 \
  --port 30306 \
  --database casdoor \
  --user root \
  --password 'your-casdoor-db-password' \
  --sql deployments/casdoor/sql/aisphere-auth-casdoor.sql \
  --backup-before \
  -y
```

Windows PowerShell：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\casdoor\import-casdoor-sql.ps1 `
  -HostName 36.138.61.152 `
  -Port 30306 `
  -Database casdoor `
  -User root `
  -Password 'your-casdoor-db-password' `
  -SqlFile .\deployments\casdoor\sql\aisphere-auth-casdoor.sql `
  -BackupBefore `
  -Yes
```

如果本机没有 `mysql` 客户端，可以走 Docker：

```bash
bash scripts/casdoor/import-casdoor-sql.sh \
  --host 36.138.61.152 \
  --port 30306 \
  --database casdoor \
  --user root \
  --password 'your-casdoor-db-password' \
  --sql deployments/casdoor/sql/aisphere-auth-casdoor.sql \
  --use-docker \
  --backup-before \
  -y
```

注意：不建议直接在脚本中拼接 Casdoor 表的 INSERT。Casdoor 表结构会随版本变化，最稳的方式是从同版本参考环境导出已验证 SQL，再导入到新环境。

详细说明见：[docs/casdoor-startup.md](docs/casdoor-startup.md) 和 [deployments/casdoor/sql/README.md](deployments/casdoor/sql/README.md)。

## 三、CLI 用法

显示中文帮助：

```bash
./aisphere-auth -h
```

指定配置文件启动：

```bash
./aisphere-auth --config configs/config.yaml
```

用命令行覆盖配置：

```bash
./aisphere-auth \
  --config configs/config.yaml \
  --addr :18080 \
  --mode release \
  --session-provider redis \
  --redis-addrs 127.0.0.1:6379
```

只检查配置，不启动服务：

```bash
./aisphere-auth check-config --config configs/config.yaml
```

打印脱敏后的最终配置：

```bash
./aisphere-auth --config configs/config.yaml --print-config
```

显示版本：

```bash
./aisphere-auth version
```

## 四、配置说明

配置优先级：

```text
命令行参数 > 环境变量 > 配置文件 > 默认值
```

复制中文注释配置样例：

```bash
cp configs/config.yaml.example configs/config.yaml
```

样例文件已经给每个字段添加中文说明，包括 Casdoor、Redis、Cookie、Service Token、JWT 等配置应该怎么填写。

更多说明见：[docs/configuration.md](docs/configuration.md)。

## 五、本地开发启动

### memory session 模式

```bash
go mod tidy
go run ./cmd/server
```

### Redis session 模式

```bash
export AISPHERE_SESSION_PROVIDER="redis"
export AISPHERE_REDIS_ADDRS="127.0.0.1:6379"
export AISPHERE_REDIS_PREFIX="aisphere"
go run ./cmd/server
```

或者使用 docker compose：

```bash
cd deployments/docker
docker compose up --build
```

## 六、离线 `.run` 包

本地构建离线包：

```bash
bash build.sh --arch amd64
bash build.sh --arch arm64
bash build.sh --arch all
```

安装到离线 Kubernetes 环境，并推送镜像到目标内网仓库：

```bash
./dist/aisphere-auth-0.1.0-amd64.run install -y \
  --registry sealos.hub:5000 \
  --namespace aisphere-system
```

使用其他 registry：

```bash
./dist/aisphere-auth-0.1.0-amd64.run install -y \
  --registry 10.10.10.10:5000 \
  --namespace aisphere-system
```

只渲染不安装：

```bash
./dist/aisphere-auth-0.1.0-amd64.run install -y \
  --registry sealos.hub:5000 \
  --skip-push \
  --skip-apply \
  --output-dir ./out
```

更多说明见：[docs/offline-run.md](docs/offline-run.md)。

## 七、GitHub Actions

- `.github/workflows/ci.yml`：运行 `gofmt`、`go vet`、`go test ./...`。
- `.github/workflows/offline-run.yml`：构建 `amd64` 和 `arm64` `.run` 包。
- 推送 `v*` tag 时会将 `.run` 和 `.sha256` 挂到 GitHub Release。

## 八、内部服务令牌

SkillHub、AgentRuntime、SQLHub 等可信组件会调用 `/auth/sessions/introspect` 和 `/authz/check`。生产环境必须启用 service token：

```bash
export AISPHERE_SERVICE_TOKEN_REQUIRED=true
export AISPHERE_SERVICE_TOKEN='replace-with-long-random-secret'
export AISPHERE_SERVICE_TOKEN_HEADER='X-Aisphere-Service-Token'
```

SDK 用法：

```go
client := client.NewHTTPClient(
    "http://aisphere-auth:18080",
    client.WithServiceToken(os.Getenv("AISPHERE_SERVICE_TOKEN")),
)
```

HTTP 用法：

```bash
curl -X POST http://127.0.0.1:18080/authz/check \
  -H 'Content-Type: application/json' \
  -H "X-Aisphere-Service-Token: $AISPHERE_SERVICE_TOKEN" \
  -d '{"subject":"skillhub/admin","object":"skillhub:skill:*","action":"admin:read"}'
```

## 九、设计边界

Casdoor 负责：

```text
账号
组织
角色
权限模型
权限策略
SSO 登录态
```

AI Sphere Auth 负责：

```text
平台 Session
Principal 标准化
AuthZ Check 封装
内部服务 SDK
离线交付
配置和启动工程化
```

业务系统不直接接 Casdoor，而是通过 `aisphere-auth` 统一接入。

## 十、后续计划

1. 增加 Casdoor API bootstrap 工具，逐步替代数据库 SQL 直导。
2. 增加 Redis readyz 深度检查和 session sliding TTL 验收。
3. 增加 `casdoor-go-sdk` adapter。
4. 增加 AgentRuntime / CLI / 服务间调用的 API Token / JWT 能力。
5. 开始接入 SkillHub，先保护 `/v3/admin/skills`。
