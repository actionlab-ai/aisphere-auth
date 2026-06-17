# Casdoor SQL 初始化目录

这个目录用于存放可导入到 Casdoor MySQL 数据库的 SQL 初始化文件。

## 当前推荐方式：项目专用 Seed SQL

`aisphere-auth` 现在推荐用项目专用 seed 生成器，而不是从某个历史 Casdoor dump 里按关键字抽数据。

生成命令：

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

导入命令：

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

生成的 SQL 是幂等的，使用 `INSERT ... ON DUPLICATE KEY UPDATE`，不会 `DROP TABLE`，也不会导入 token/session/record 等运行态数据。

## Seed 会初始化什么

默认会创建或更新：

```text
Organization:
  aisphere

Application:
  aisphere-auth

Model:
  aisphere-auth-model

Roles:
  role_platform_admin
  role_platform_viewer
  role_skillhub_admin
  role_skillhub_editor
  role_skillhub_viewer
  role_agentruntime_admin
  role_agentruntime_operator
  role_agentruntime_viewer
  role_sqlhub_admin
  role_sqlhub_viewer
  role_modelgateway_admin
  role_modelgateway_viewer
  role_portal_admin
  role_portal_viewer

Permissions:
  perm_platform_admin
  perm_platform_viewer
  perm_skillhub_admin
  perm_skillhub_editor
  perm_skillhub_viewer
  perm_agentruntime_admin
  perm_agentruntime_operator
  perm_agentruntime_viewer
  perm_sqlhub_admin
  perm_sqlhub_viewer
  perm_modelgateway_admin
  perm_modelgateway_viewer
  perm_portal_admin
  perm_portal_viewer
```

默认会把 `aisphere/admin` 绑定到 `role_platform_admin`。这个用户需要已经存在，或者后续在 Casdoor 里创建。

## aisphere-auth 对应配置

导入 seed 后，`aisphere-auth` 建议配置为：

```yaml
casdoor:
  endpoint: "http://127.0.0.1:8008"
  owner: "aisphere"
  application: "aisphere-auth"
  clientId: "aisphere-auth"
  clientSecret: "<与 seed 生成时一致>"
  redirectURL: "http://127.0.0.1:18080/auth/callback/casdoor"
  permissionId: "aisphere/perm_platform_admin"
```

## 历史 dump 预处理能力

`scripts/casdoor/prepare-casdoor-sql.py` 仍然保留，但它只适合迁移一个已验证环境中的配置，不适合作为当前项目的默认初始化方式。

处理完整 mysqldump：

```bash
python3 scripts/casdoor/prepare-casdoor-sql.py \
  --input ./casdoor.sql \
  --output deployments/casdoor/sql/aisphere-auth-casdoor.sql \
  --mode data-only \
  --keywords aisphere,skillhub
```

或者一边准备一边导入：

```bash
bash scripts/casdoor/import-casdoor-sql.sh \
  --host 127.0.0.1 \
  --port 3306 \
  --database casdoor \
  --user root \
  --password '<Casdoor MySQL 密码>' \
  --sql ./casdoor.sql \
  --prepare-dump \
  --prepare-mode data-only \
  --backup-before \
  -y
```

这个能力会按关键字抽取历史 dump 中的配置行，可能受原环境命名、历史数据和表结构影响。新环境开箱即用优先使用 `render-casdoor-seed.py`。
