# Casdoor SQL 初始化目录

这个目录用于存放可导入到 Casdoor MySQL 数据库的 SQL 初始化文件。

## 推荐文件名

```text
deployments/casdoor/sql/aisphere-auth-casdoor.sql
```

这个文件默认不会在仓库中提供真实生产内容，因为 Casdoor 的数据库表结构会随版本变化，而且 SQL 中通常包含 client secret、证书、应用配置、用户密码 hash 等敏感数据。

## 推荐做法

1. 先在一个参考 Casdoor 环境中完成一次正确配置。
2. 从该参考环境导出完整 `casdoor.sql`。
3. 使用 `scripts/casdoor/prepare-casdoor-sql.py` 或 `import-casdoor-sql.sh --prepare-dump` 生成 data-only SQL。
4. 脱敏检查后保存为 `aisphere-auth-casdoor.sql`。
5. 在新环境中用 `scripts/casdoor/import-casdoor-sql.sh` 或 PowerShell 脚本一键导入。

## 处理完整 mysqldump

完整 Casdoor dump 一般包含：

```text
CREATE DATABASE / USE
SET @@GLOBAL.GTID_PURGED
SET @@SESSION.SQL_LOG_BIN
DROP TABLE / CREATE TABLE
LOCK TABLES / UNLOCK TABLES
token / session / record 等运行态数据
```

已有 Casdoor 环境推荐生成 data-only SQL：

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
  --password 'your-password' \
  --sql ./casdoor.sql \
  --prepare-dump \
  --prepare-mode data-only \
  --backup-before \
  -y
```

如果确实是全新数据库，需要导入完整 schema/data：

```bash
bash scripts/casdoor/import-casdoor-sql.sh \
  --host 127.0.0.1 \
  --port 3306 \
  --database casdoor \
  --user root \
  --password 'your-password' \
  --sql ./casdoor.sql \
  --prepare-dump \
  --prepare-mode full \
  --create-database \
  --allow-destructive \
  -y
```

`full` 模式会移除容易失败或环境绑定的语句：

```text
GTID_PURGED
SQL_LOG_BIN
CREATE DATABASE
USE
LOCK TABLES
UNLOCK TABLES
```

但仍然会保留 `DROP TABLE` / `CREATE TABLE`，所以必须显式加 `--allow-destructive`。

## data-only 默认行为

`data-only` 会：

```text
提取包含 aisphere / skillhub 关键字的配置行
默认处理 organization / application / model / permission / role / group / adapter / enforcer 等配置表
默认跳过 token / session / record / ticket 等运行态表
输出 REPLACE INTO 语句
```

如果确实要连用户一起迁移，可以加：

```bash
--prepare-include-users
```

这会复制用户密码 hash，请导入前人工检查输出 SQL。

## 为什么不直接在脚本里拼 Casdoor INSERT？

Casdoor 的表结构不是稳定公共 API。不同版本、不同数据库方言、不同初始化方式下，`application`、`permission`、`model`、`role` 等表的字段可能不同。为了避免导入脚本在版本升级后破坏生产数据库，本项目采用“导入已验证 SQL 文件”或“从完整 dump 生成 data-only SQL”的方式。

后续可以继续扩展一个 Casdoor API bootstrap 工具，用 Casdoor API 创建 Application、Model、Permission 和 Policy，避免直接依赖数据库表结构。