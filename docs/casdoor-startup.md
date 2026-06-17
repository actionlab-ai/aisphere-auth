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

## 二、你这类完整 casdoor.sql 应该怎么导入

你提供的 `casdoor.sql` 是完整 MySQL dump，里面包含：

```text
CREATE DATABASE / USE casdoor
SET @@GLOBAL.GTID_PURGED
SET @@SESSION.SQL_LOG_BIN
DROP TABLE / CREATE TABLE
LOCK TABLES / UNLOCK TABLES
token / session / record 等运行态数据
```

这种 SQL **不建议直接导入到已有 Casdoor 生产库**，因为会 DROP/CREATE 表，也可能因为 GTID 或 LOCK TABLES 权限不足而失败。

推荐用 data-only 模式，只提取 `aisphere` / `skillhub` 相关配置数据：

```bash
bash scripts/casdoor/import-casdoor-sql.sh \
  --host 36.138.61.152 \
  --port 30306 \
  --database casdoor \
  --user root \
  --password 'your-casdoor-db-password' \
  --sql ./casdoor.sql \
  --prepare-dump \
  --prepare-mode data-only \
  --backup-before \
  -y
```

这会自动调用：

```text
scripts/casdoor/prepare-casdoor-sql.py
```

处理逻辑：

```text
1. 从完整 dump 中提取 aisphere / skillhub 相关行
2. 默认只处理 organization / application / model / permission / role / group / adapter / enforcer 等配置表
3. 默认跳过 token / session / record / ticket 等运行态表
4. 输出 REPLACE INTO 语句
5. 再导入目标 Casdoor 数据库
```

如果你只是想先生成处理后的 SQL，不导入：

```bash
bash scripts/casdoor/import-casdoor-sql.sh \
  --sql ./casdoor.sql \
  --prepare-dump \
  --prepare-mode data-only \
  --prepared-sql ./deployments/casdoor/sql/aisphere-auth-casdoor.sql \
  --prepare-only
```

如果确实是全新 Casdoor 数据库，要导入完整 dump：

```bash
bash scripts/casdoor/import-casdoor-sql.sh \
  --host 36.138.61.152 \
  --port 30306 \
  --database casdoor \
  --user root \
  --password 'your-casdoor-db-password' \
  --sql ./casdoor.sql \
  --prepare-dump \
  --prepare-mode full \
  --create-database \
  --allow-destructive \
  -y
```

full 模式会去掉容易失败的语句：

```text
GTID_PURGED
SQL_LOG_BIN
CREATE DATABASE
USE
LOCK TABLES
UNLOCK TABLES
```

但仍会保留 `DROP TABLE` / `CREATE TABLE` / 全量数据，所以必须显式传 `--allow-destructive`。

Windows PowerShell data-only 示例：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\casdoor\import-casdoor-sql.ps1 `
  -HostName 36.138.61.152 `
  -Port 30306 `
  -Database casdoor `
  -User root `
  -Password 'your-casdoor-db-password' `
  -SqlFile .\casdoor.sql `
  -PrepareDump `
  -PrepareMode data-only `
  -BackupBefore `
  -Yes
```

## 三、普通已准备 SQL 的导入方式

如果 SQL 已经是处理后的 data-only 文件，放到：

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

## 四、配置 aisphere-auth

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
  serviceToken: "请替换成长随机字符串，至少 32 位"
```

## 五、检查配置

```bash
./aisphere-auth check-config --config configs/config.yaml
```

打印脱敏后的最终配置：

```bash
./aisphere-auth --config configs/config.yaml --print-config
```

## 六、启动服务

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

注意：`readyz` 已经是真检查。Casdoor 或 Redis 不通时会返回 `503`。

## 七、验证登录

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

## 八、验证权限检查

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

## 九、常见问题

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

### 4. 导入完整 dump 报 GTID_PURGED 权限错误

使用：

```bash
--prepare-dump --prepare-mode full
```

脚本会去掉 `SET @@GLOBAL.GTID_PURGED`。

### 5. 不建议直接手写 INSERT

Casdoor 表结构会随版本变化。推荐从同版本参考环境导出 SQL，再用脚本生成 data-only bootstrap SQL。