# Casdoor 开箱即用初始化

本项目不再推荐把某个环境的完整 `casdoor.sql` 直接裁剪成 `data-only` 后导入。更合理的方式是：针对 `aisphere-auth` 当前项目生成一套标准基线配置，再导入目标 Casdoor。

基线配置包含：

- Organization：默认 `aisphere`
- OAuth Application：默认 `aisphere-auth`
- Casbin Model：`sub / obj / act` 三元组模型
- 常用 Role：平台管理员、平台只读、审计、安全、SkillHub、AgentRuntime、SQLHub、ModelGateway、Portal 等角色
- 常用 Permission / Policy：按资源前缀和动作建立默认策略
- Permission Rule：写入 Casdoor `permission_rule`，让策略可被 Casdoor enforce 使用
- Role Binding：默认把 `aisphere/admin` 绑定到 `role_platform_admin`

## 推荐方式：Python 初始化

Windows 上优先使用 Python 初始化，避免 PowerShell 脚本兼容问题。

### 只生成 SQL，不导入

```powershell
python .\scripts\casdoor\bootstrap-casdoor-mysql.py `
  --seed-only `
  --output .\deployments\casdoor\sql\aisphere-auth-casdoor.sql `
  --env-output .\.env.casdoor.generated `
  --org aisphere `
  --org-display-name "AI Sphere" `
  --app aisphere-auth `
  --app-display-name "AI Sphere Auth" `
  --client-id aisphere-auth `
  --client-secret "<替换为真实 OAuth Client Secret>" `
  --redirect-uri "http://127.0.0.1:18080/auth/callback/casdoor" `
  --admin-user admin
```

### 生成并导入已有 Casdoor MySQL

```powershell
python .\scripts\casdoor\bootstrap-casdoor-mysql.py `
  --output .\deployments\casdoor\sql\aisphere-auth-casdoor.sql `
  --env-output .\.env.casdoor.generated `
  --org aisphere `
  --org-display-name "AI Sphere" `
  --app aisphere-auth `
  --app-display-name "AI Sphere Auth" `
  --client-id aisphere-auth `
  --client-secret "<替换为真实 OAuth Client Secret>" `
  --redirect-uri "http://127.0.0.1:18080/auth/callback/casdoor" `
  --admin-user admin `
  --host 127.0.0.1 `
  --port 3306 `
  --database casdoor `
  --user root `
  --password "<Casdoor MySQL 密码>" `
  --backup-before `
  --yes
```

如果本机没有 `mysql.exe`，但有 Docker，可以加：

```powershell
  --use-docker
```

示例：

```powershell
python .\scripts\casdoor\bootstrap-casdoor-mysql.py `
  --use-docker `
  --client-secret "<替换为真实 OAuth Client Secret>" `
  --redirect-uri "http://127.0.0.1:18080/auth/callback/casdoor" `
  --host 127.0.0.1 `
  --port 3306 `
  --database casdoor `
  --user root `
  --password "<Casdoor MySQL 密码>" `
  --backup-before `
  --yes
```

## Linux / macOS 兼容方式

```bash
bash scripts/casdoor/import-casdoor-sql.sh \
  --seed \
  --seed-org aisphere \
  --seed-app aisphere-auth \
  --seed-client-id aisphere-auth \
  --seed-client-secret '<替换为真实 OAuth Client Secret>' \
  --seed-redirect-uri http://127.0.0.1:18080/auth/callback/casdoor \
  --seed-env-output ./.env.casdoor.generated \
  --host 127.0.0.1 \
  --port 3306 \
  --database casdoor \
  --user root \
  --password '<Casdoor MySQL 密码>' \
  --backup-before \
  -y
```

## aisphere-auth 配置

导入后，`aisphere-auth` 配置应与 seed 参数保持一致。建议直接读取 `--env-output` 生成的 `.env.casdoor.generated`，或者写入配置文件：

```yaml
casdoor:
  endpoint: "http://127.0.0.1:8008"
  owner: "aisphere"
  application: "aisphere-auth"
  clientId: "aisphere-auth"
  clientSecret: "<与 seed 使用的 Client Secret 一致>"
  redirectURL: "http://127.0.0.1:18080/auth/callback/casdoor"
  permissionId: "aisphere/perm_platform_admin"
```

## 默认角色和策略

| Role | 说明 | 默认资源 | 默认动作 |
|---|---|---|---|
| `role_platform_admin` | 平台管理员 | `*` | `*` |
| `role_platform_viewer` | 平台只读 | `portal:*`、`skillhub:*`、`agentruntime:*`、`sqlhub:*`、`modelgateway:*`、`audit:*` | `read`、`view`、`list`、`admin:read` |
| `role_audit_admin` | 审计管理员 | `audit:*`、`*:audit:*` | `*` |
| `role_audit_viewer` | 审计只读 | `audit:*`、`*:audit:*` | `read`、`view`、`list` |
| `role_security_admin` | 安全管理员 | `auth:*`、`authn:*`、`authz:*`、`audit:*`、`casdoor:*` | `*` |
| `role_skillhub_admin` | SkillHub 管理员 | `skillhub:*` | `*` |
| `role_skillhub_editor` | SkillHub 编辑者 | `skillhub:skill:*`、`skillhub:group:*`、`skillhub:proposal:*`、`skillhub:release:*`、`skillhub:knowledge:*`、`skillhub:workflow:*`、`skillhub:audit:*` | `read`、`view`、`list`、`write`、`create`、`update`、`delete`、`publish`、`rollback`、`approve`、`reject`、`admin:read`、`admin:write` |
| `role_skillhub_viewer` | SkillHub 只读 | `skillhub:*` | `read`、`view`、`list`、`admin:read` |
| `role_agentruntime_admin` | AgentRuntime 管理员 | `agentruntime:*` | `*` |
| `role_agentruntime_operator` | AgentRuntime 操作者 | `agentruntime:run:*`、`agentruntime:session:*`、`agentruntime:agent:*`、`agentruntime:tool:*`、`agentruntime:audit:*` | `read`、`view`、`list`、`run`、`stop`、`restart`、`approve`、`reject`、`admin:read` |
| `role_sqlhub_admin` | SQLHub 管理员 | `sqlhub:*` | `*` |
| `role_sqlhub_editor` | SQLHub 编辑者 | `sqlhub:datasource:*`、`sqlhub:query:*`、`sqlhub:template:*`、`sqlhub:report:*`、`sqlhub:audit:*` | `read`、`view`、`list`、`create`、`update`、`execute`、`export`、`admin:read`、`admin:write` |
| `role_modelgateway_admin` | ModelGateway 管理员 | `modelgateway:*` | `*` |
| `role_modelgateway_operator` | ModelGateway 操作者 | `modelgateway:model:*`、`modelgateway:route:*`、`modelgateway:key:*`、`modelgateway:quota:*`、`modelgateway:audit:*` | `read`、`view`、`list`、`create`、`update`、`publish`、`disable`、`enable`、`admin:read` |
| `role_portal_admin` | Portal 管理员 | `portal:*` | `*` |
| `role_portal_editor` | Portal 编辑者 | `portal:page:*`、`portal:menu:*`、`portal:notice:*`、`portal:asset:*` | `read`、`view`、`list`、`create`、`update`、`delete`、`publish`、`admin:read` |

## 和 dump 预处理工具的关系

`scripts/casdoor/prepare-casdoor-sql.py` 仍然保留，但它只适合从一个已经配置好的 Casdoor 环境迁移数据。项目默认初始化请优先使用：

```bash
python scripts/casdoor/bootstrap-casdoor-mysql.py --client-secret '<secret>' --password '<mysql-password>' --yes
```

这样不用依赖某个环境的历史 dump，也不会误迁移 token、session、record 等运行态数据。
