# Casdoor 开箱即用初始化

本项目不再推荐把某个环境的完整 `casdoor.sql` 直接裁剪成 `data-only` 后导入。更合理的方式是：针对 `aisphere-auth` 当前项目生成一套标准基线配置，再导入目标 Casdoor。

基线配置包含：

- Organization：默认 `aisphere`
- OAuth Application：默认 `aisphere-auth`
- Casbin Model：`sub / obj / act` 三元组模型
- 常用 Role：平台管理员、平台只读、SkillHub、AgentRuntime、SQLHub、ModelGateway、Portal 等角色
- 常用 Permission / Policy：按资源前缀和动作建立默认策略
- Role Binding：默认把 `aisphere/admin` 绑定到 `role_platform_admin`

## 1. 生成项目专用 SQL

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

生成的 SQL 是幂等的，使用 `INSERT ... ON DUPLICATE KEY UPDATE`，不会删除或创建 Casdoor 表。

## 2. 导入到已有 Casdoor MySQL

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

## 3. aisphere-auth 配置

导入后，`aisphere-auth` 配置应与生成 SQL 的参数保持一致：

```yaml
casdoor:
  endpoint: "http://127.0.0.1:8008"
  owner: "aisphere"
  application: "aisphere-auth"
  clientId: "aisphere-auth"
  clientSecret: "<与生成 SQL 使用的 Client Secret 一致>"
  redirectURL: "http://127.0.0.1:18080/auth/callback/casdoor"
  permissionId: "aisphere/perm_platform_admin"
```

## 4. 默认角色和策略

| Role | 说明 | 默认资源 | 默认动作 |
|---|---|---|---|
| `role_platform_admin` | 平台管理员 | `*` | `*` |
| `role_platform_viewer` | 平台只读 | `portal:*`、`skillhub:*`、`agentruntime:*`、`sqlhub:*`、`modelgateway:*` | `read`、`view`、`list`、`admin:read` |
| `role_skillhub_admin` | SkillHub 管理员 | `skillhub:*` | `*` |
| `role_skillhub_editor` | SkillHub 编辑者 | `skillhub:skill:*`、`skillhub:knowledge:*`、`skillhub:workflow:*` | `read`、`write`、`approve`、`admin:read`、`admin:write` |
| `role_skillhub_viewer` | SkillHub 只读 | `skillhub:*` | `read`、`view`、`list`、`admin:read` |
| `role_agentruntime_admin` | AgentRuntime 管理员 | `agentruntime:*` | `*` |
| `role_agentruntime_operator` | AgentRuntime 操作者 | `agentruntime:run:*`、`agentruntime:session:*`、`agentruntime:agent:*` | `read`、`run`、`stop`、`admin:read` |
| `role_sqlhub_admin` | SQLHub 管理员 | `sqlhub:*` | `*` |
| `role_modelgateway_admin` | ModelGateway 管理员 | `modelgateway:*` | `*` |
| `role_portal_admin` | Portal 管理员 | `portal:*` | `*` |

## 5. 和 dump 预处理工具的关系

`scripts/casdoor/prepare-casdoor-sql.py` 仍然保留，但它只适合从一个已经配置好的 Casdoor 环境迁移数据。项目默认初始化请优先使用：

```bash
python3 scripts/casdoor/render-casdoor-seed.py ...
```

这样不用依赖某个环境的历史 dump，也不会误迁移 token、session、record 等运行态数据。
