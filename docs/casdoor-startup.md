# 已有 Casdoor 环境下启动 AI Sphere Auth

本文说明在已经有 Casdoor 服务和 Casdoor MySQL 数据库的情况下，如何不通过 UI 点点点，直接导入 SQL 初始化配置，然后启动 `aisphere-auth`。

## 一、需要准备什么

你需要知道：

```text
Casdoor 访问地址，例如：http://36.138.61.152:8008
Casdoor MySQL 地址，例如：36.138.61.152:30306
Casdoor MySQL 数据库名，例如：casdoor
Casdoor MySQL 用户和密码
Casdoor 中给 aisphere-auth 使用的 Application clientId / clientSecret
Casdoor Permission ID，例如：skillhub/platform_permission
```

如果 Application、Model、Permission、Policy 已经在 Casdoor 中配置好，只需要把这些值写入 `configs/config.yaml`。

如果还没配置，可以先从参考环境导出 SQL，再在新环境中导入。

## 二、导入 Casdoor SQL

把准备好的 SQL 放到：

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

如果本机没有 `mysql` 客户端，可以使用 Docker 模式：

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

## 三、配置 aisphere-auth

复制配置样例：

```bash
cp configs/config.yaml.example configs/config.yaml
```

核心配置示例：

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

## 四、检查配置

```bash
./aisphere-auth check-config --config configs/config.yaml
```

打印脱敏后的最终配置：

```bash
./aisphere-auth --config configs/config.yaml --print-config
```

## 五、启动服务

开发启动：

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

## 六、验证登录

浏览器打开：

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

验证当前用户：

```bash
curl -i http://127.0.0.1:18080/auth/me
```

## 七、验证权限检查

如果启用了内部服务令牌：

```bash
export AISPHERE_SERVICE_TOKEN='你的 service token'
```

执行：

```bash
curl -X POST http://127.0.0.1:18080/authz/check \
  -H 'Content-Type: application/json' \
  -H "X-Aisphere-Service-Token: $AISPHERE_SERVICE_TOKEN" \
  -d '{"subject":"skillhub/admin","object":"skillhub:skill:*","action":"admin:read"}'
```

预期返回：

```json
{
  "allow": true,
  "source": "casdoor",
  "subject": "skillhub/admin",
  "object": "skillhub:skill:*",
  "action": "admin:read"
}
```

## 八、常见问题

### 1. 回调后报 invalid audience

检查 `casdoor.clientId` 和 Casdoor Application 的 Client ID 是否一致。

### 2. 权限一直 forbidden

检查：

```text
casdoor.permissionId 是否等于 owner/name
subject 是否和 Casdoor policy 里的 g 规则一致
object/action 是否和 policy 匹配
Casdoor Permission 的 model 是否为三元组 sub,obj,act
```

### 3. 导入 SQL 后 Casdoor UI 没变化

检查是否导入到了正确数据库，以及 Casdoor 是否连接同一个 MySQL。必要时重启 Casdoor。

### 4. 不建议直接手写 INSERT

Casdoor 表结构会随版本变化。推荐从同版本参考环境导出 SQL，再用脚本导入。
