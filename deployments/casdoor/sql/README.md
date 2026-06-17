# Casdoor SQL 初始化目录

这个目录用于存放可导入到 Casdoor MySQL 数据库的 SQL 初始化文件。

## 推荐文件名

```text
deployments/casdoor/sql/aisphere-auth-casdoor.sql
```

这个文件默认不会在仓库中提供真实生产内容，因为 Casdoor 的数据库表结构会随版本变化，而且 SQL 中通常包含 client secret、证书、应用配置等敏感数据。

推荐做法：

1. 先在一个参考 Casdoor 环境中完成一次正确配置。
2. 从该参考环境导出与 AI Sphere Auth 相关的 SQL。
3. 脱敏检查后保存为 `aisphere-auth-casdoor.sql`。
4. 在新环境中用 `scripts/casdoor/import-casdoor-sql.sh` 或 PowerShell 脚本一键导入。

## 导入示例

Linux / macOS：

```bash
bash scripts/casdoor/import-casdoor-sql.sh \
  --host 127.0.0.1 \
  --port 3306 \
  --database casdoor \
  --user root \
  --password 'your-password' \
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
  -Password 'your-password' `
  -SqlFile .\deployments\casdoor\sql\aisphere-auth-casdoor.sql `
  -BackupBefore `
  -Yes
```

## 为什么不直接在脚本里拼 Casdoor INSERT？

Casdoor 的表结构不是稳定公共 API。不同版本、不同数据库方言、不同初始化方式下，`application`、`permission`、`model`、`role` 等表的字段可能不同。为了避免导入脚本在版本升级后破坏生产数据库，本项目采用“导入已验证 SQL 文件”的方式。

后续可以继续扩展一个 Casdoor API bootstrap 工具，用 Casdoor API 创建 Application、Model、Permission 和 Policy，避免直接依赖数据库表结构。
