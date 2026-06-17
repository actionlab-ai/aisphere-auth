#!/usr/bin/env bash
set -euo pipefail

HOST="127.0.0.1"
PORT="3306"
DATABASE="casdoor"
USER="root"
PASSWORD=""
SQL_FILE="deployments/casdoor/sql/aisphere-auth-casdoor.sql"
MYSQL_BIN="mysql"
MYSQLDUMP_BIN="mysqldump"
USE_DOCKER="false"
DOCKER_IMAGE="mysql:8.0"
BACKUP_BEFORE="false"
BACKUP_DIR="backups/casdoor"
CREATE_DATABASE="false"
DRY_RUN="false"
YES="false"
ALLOW_DESTRUCTIVE="false"
PREPARE_DUMP="false"
PREPARE_MODE="data-only"
PREPARED_SQL=""
PREPARE_KEYWORDS="aisphere,skillhub"
PREPARE_INCLUDE_USERS="false"
PREPARE_ONLY="false"
MYSQL_CNF=""
DOCKER_CNF_DIR=""
TEMP_SQL_FILE=""

usage() {
  cat <<'EOF'
Casdoor SQL 自动导入工具

用途：
  将 Casdoor SQL 初始化文件导入到现有 Casdoor MySQL 数据库，避免手工在 Casdoor UI 中点模型、权限、角色、策略。

推荐流程：
  1. 从已配置好的同版本 Casdoor 导出 SQL，例如 casdoor.sql。
  2. 使用 --prepare-dump --prepare-mode data-only 提取 aisphere/skillhub 相关数据。
  3. 导入前使用 --backup-before 备份目标库。

用法：
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

参数：
  --host <host>              Casdoor MySQL 地址，默认 127.0.0.1
  --port <port>              Casdoor MySQL 端口，默认 3306
  --database <db>            Casdoor 数据库名，默认 casdoor，只允许字母数字下划线
  --user <user>              MySQL 用户，默认 root，只允许字母数字下划线和横线
  --password <password>      MySQL 密码，也可用环境变量 CASDOOR_MYSQL_PASSWORD
  --sql <file>               要导入的 SQL 文件，默认 deployments/casdoor/sql/aisphere-auth-casdoor.sql
  --mysql-bin <path>         mysql 命令路径，默认 mysql
  --use-docker               不依赖本机 mysql 客户端，改用 docker run mysql:8.0 执行导入
  --docker-image <image>     --use-docker 时使用的镜像，默认 mysql:8.0
  --backup-before            导入前先 mysqldump 备份目标数据库
  --backup-dir <dir>         备份目录，默认 backups/casdoor
  --create-database          导入前执行 CREATE DATABASE IF NOT EXISTS
  --allow-destructive        允许导入包含 DROP/CREATE TABLE 的完整 dump。生产请谨慎使用。
  --dry-run                  只打印将执行的动作，不真正导入
  -y, --yes                  跳过确认

适配完整 Casdoor dump：
  --prepare-dump             导入前先把原始 dump 处理成可导入 SQL
  --prepare-mode <mode>      data-only 或 full，默认 data-only
                             data-only：只提取 aisphere/skillhub 相关数据，使用 REPLACE INTO，适合已有 Casdoor
                             full：保留完整 schema/data，但去掉 GTID、SQL_LOG_BIN、CREATE DATABASE、USE、LOCK TABLES
  --prepared-sql <file>      处理后的 SQL 输出路径。默认写到临时文件
  --prepare-keywords <list>  data-only 提取关键字，默认 aisphere,skillhub
  --prepare-include-users    data-only 模式也提取匹配用户。会复制密码 hash，需谨慎。
  --prepare-only             只生成处理后的 SQL，不执行导入

注意：
  1. 完整 mysqldump 常包含 SET @@GLOBAL.GTID_PURGED、CREATE DATABASE、USE、LOCK TABLES，直接导入到受限 MySQL 往往失败。
  2. data-only 模式不会导入 token/session/record/ticket 等运行态数据，适合把已配置好的 Casdoor 权限迁移到目标环境。
  3. full 模式可能 DROP/CREATE Casdoor 表，必须显式传 --allow-destructive。
  4. 生产导入建议开启 --backup-before。
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host) HOST="$2"; shift 2 ;;
    --port) PORT="$2"; shift 2 ;;
    --database) DATABASE="$2"; shift 2 ;;
    --user) USER="$2"; shift 2 ;;
    --password) PASSWORD="$2"; shift 2 ;;
    --sql) SQL_FILE="$2"; shift 2 ;;
    --mysql-bin) MYSQL_BIN="$2"; shift 2 ;;
    --use-docker) USE_DOCKER="true"; shift ;;
    --docker-image) DOCKER_IMAGE="$2"; shift 2 ;;
    --backup-before) BACKUP_BEFORE="true"; shift ;;
    --backup-dir) BACKUP_DIR="$2"; shift 2 ;;
    --create-database) CREATE_DATABASE="true"; shift ;;
    --allow-destructive) ALLOW_DESTRUCTIVE="true"; shift ;;
    --prepare-dump) PREPARE_DUMP="true"; shift ;;
    --prepare-mode) PREPARE_MODE="$2"; shift 2 ;;
    --prepared-sql) PREPARED_SQL="$2"; shift 2 ;;
    --prepare-keywords) PREPARE_KEYWORDS="$2"; shift 2 ;;
    --prepare-include-users) PREPARE_INCLUDE_USERS="true"; shift ;;
    --prepare-only) PREPARE_ONLY="true"; shift ;;
    --dry-run) DRY_RUN="true"; shift ;;
    -y|--yes) YES="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "[ERROR] unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

if [[ -z "$PASSWORD" && -n "${CASDOOR_MYSQL_PASSWORD:-}" ]]; then
  PASSWORD="$CASDOOR_MYSQL_PASSWORD"
fi

validate_identifier() {
  local name="$1"
  local value="$2"
  local regex="$3"
  if [[ ! "$value" =~ $regex ]]; then
    echo "[ERROR] invalid ${name}: ${value}" >&2
    exit 1
  fi
}

validate_identifier "database" "$DATABASE" '^[A-Za-z0-9_]+$'
validate_identifier "user" "$USER" '^[A-Za-z0-9_-]+$'
validate_identifier "port" "$PORT" '^[0-9]+$'
case "$PREPARE_MODE" in
  data-only|full) ;;
  *) echo "[ERROR] invalid --prepare-mode: $PREPARE_MODE" >&2; exit 1 ;;
esac

cleanup() {
  if [[ -n "${MYSQL_CNF}" && -f "${MYSQL_CNF}" ]]; then
    rm -f "${MYSQL_CNF}"
  fi
  if [[ -n "${DOCKER_CNF_DIR}" && -d "${DOCKER_CNF_DIR}" ]]; then
    rm -rf "${DOCKER_CNF_DIR}"
  fi
  if [[ -n "${TEMP_SQL_FILE}" && -f "${TEMP_SQL_FILE}" ]]; then
    rm -f "${TEMP_SQL_FILE}"
  fi
}
trap cleanup EXIT

create_mysql_cnf() {
  MYSQL_CNF="$(mktemp)"
  chmod 600 "${MYSQL_CNF}"
  cat > "${MYSQL_CNF}" <<EOF
[client]
user=${USER}
password=${PASSWORD}
host=${HOST}
port=${PORT}
protocol=tcp
EOF
}

create_docker_cnf() {
  DOCKER_CNF_DIR="$(mktemp -d)"
  chmod 700 "${DOCKER_CNF_DIR}"
  cat > "${DOCKER_CNF_DIR}/client.cnf" <<EOF
[client]
user=${USER}
password=${PASSWORD}
host=${HOST}
port=${PORT}
protocol=tcp
EOF
  chmod 600 "${DOCKER_CNF_DIR}/client.cnf"
}

if [[ ! -f "$SQL_FILE" ]]; then
  echo "[ERROR] SQL file not found: $SQL_FILE" >&2
  echo "        Put your exported Casdoor SQL at this path or pass --sql <file>." >&2
  exit 1
fi

if [[ "$PREPARE_DUMP" == "true" ]]; then
  PREPARE_TOOL="scripts/casdoor/prepare-casdoor-sql.py"
  if [[ ! -f "$PREPARE_TOOL" ]]; then
    echo "[ERROR] prepare tool not found: $PREPARE_TOOL" >&2
    exit 1
  fi
  if ! command -v python3 >/dev/null 2>&1; then
    echo "[ERROR] python3 is required for --prepare-dump" >&2
    exit 1
  fi
  if [[ -z "$PREPARED_SQL" ]]; then
    TEMP_SQL_FILE="$(mktemp /tmp/aisphere-casdoor-prepared.XXXXXX.sql)"
    PREPARED_SQL="$TEMP_SQL_FILE"
  fi
  echo "[INFO] preparing Casdoor SQL: mode=$PREPARE_MODE output=$PREPARED_SQL"
  PREPARE_ARGS=(python3 "$PREPARE_TOOL" --input "$SQL_FILE" --output "$PREPARED_SQL" --mode "$PREPARE_MODE" --keywords "$PREPARE_KEYWORDS")
  if [[ "$PREPARE_INCLUDE_USERS" == "true" ]]; then
    PREPARE_ARGS+=(--include-users)
  fi
  "${PREPARE_ARGS[@]}"
  SQL_FILE="$PREPARED_SQL"
  if [[ "$PREPARE_ONLY" == "true" ]]; then
    echo "[OK] prepare-only completed: $SQL_FILE"
    exit 0
  fi
fi

if [[ "$ALLOW_DESTRUCTIVE" != "true" ]]; then
  if grep -Eiq '(^|[[:space:]])(DROP[[:space:]]+TABLE|CREATE[[:space:]]+TABLE|DROP[[:space:]]+DATABASE)' "$SQL_FILE"; then
    echo "[ERROR] SQL contains destructive schema statements." >&2
    echo "        Use --prepare-dump --prepare-mode data-only for existing Casdoor, or pass --allow-destructive deliberately." >&2
    exit 1
  fi
fi

if [[ "$USE_DOCKER" != "true" && ! -x "$(command -v "$MYSQL_BIN" || true)" ]]; then
  if command -v docker >/dev/null 2>&1; then
    echo "[WARN] mysql client not found, fallback to docker image: $DOCKER_IMAGE"
    USE_DOCKER="true"
  else
    echo "[ERROR] mysql client not found and docker is unavailable. Install mysql client or pass --use-docker." >&2
    exit 1
  fi
fi

if [[ "$USE_DOCKER" == "true" ]]; then
  create_docker_cnf
else
  create_mysql_cnf
fi

run_mysql_file() {
  local target_db="$1"
  local sql_file="$2"
  if [[ "$USE_DOCKER" == "true" ]]; then
    docker run --rm -i -v "${DOCKER_CNF_DIR}/client.cnf:/tmp/client.cnf:ro" "$DOCKER_IMAGE" mysql --defaults-extra-file=/tmp/client.cnf "$target_db" < "$sql_file"
  else
    "$MYSQL_BIN" --defaults-extra-file="$MYSQL_CNF" "$target_db" < "$sql_file"
  fi
}

run_mysql_exec() {
  local sql="$1"
  if [[ "$USE_DOCKER" == "true" ]]; then
    printf '%s\n' "$sql" | docker run --rm -i -v "${DOCKER_CNF_DIR}/client.cnf:/tmp/client.cnf:ro" "$DOCKER_IMAGE" mysql --defaults-extra-file=/tmp/client.cnf
  else
    "$MYSQL_BIN" --defaults-extra-file="$MYSQL_CNF" -e "$sql"
  fi
}

backup_database() {
  mkdir -p "$BACKUP_DIR"
  local ts
  ts="$(date +%Y%m%d_%H%M%S)"
  local backup_file="$BACKUP_DIR/${DATABASE}_${ts}.sql"
  echo "[INFO] backup database '$DATABASE' to $backup_file"
  if [[ "$DRY_RUN" == "true" ]]; then
    return 0
  fi
  if [[ "$USE_DOCKER" == "true" ]]; then
    docker run --rm -i -v "${DOCKER_CNF_DIR}/client.cnf:/tmp/client.cnf:ro" "$DOCKER_IMAGE" mysqldump --defaults-extra-file=/tmp/client.cnf "$DATABASE" > "$backup_file"
  else
    "$MYSQLDUMP_BIN" --defaults-extra-file="$MYSQL_CNF" "$DATABASE" > "$backup_file"
  fi
}

cat <<EOF
[INFO] Casdoor SQL import plan
  host          : $HOST
  port          : $PORT
  database      : $DATABASE
  user          : $USER
  sql           : $SQL_FILE
  useDocker     : $USE_DOCKER
  backup        : $BACKUP_BEFORE
  createDb      : $CREATE_DATABASE
  destructive   : $ALLOW_DESTRUCTIVE
  prepareDump   : $PREPARE_DUMP
  prepareMode   : $PREPARE_MODE
  dryRun        : $DRY_RUN
EOF

if [[ "$YES" != "true" ]]; then
  read -r -p "Continue importing SQL into '$DATABASE'? [y/N] " ans
  case "$ans" in
    y|Y|yes|YES) ;;
    *) echo "[INFO] cancelled"; exit 0 ;;
  esac
fi

if [[ "$CREATE_DATABASE" == "true" ]]; then
  echo "[INFO] create database if not exists: $DATABASE"
  if [[ "$DRY_RUN" != "true" ]]; then
    run_mysql_exec "CREATE DATABASE IF NOT EXISTS \`$DATABASE\` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
  fi
fi

if [[ "$BACKUP_BEFORE" == "true" ]]; then
  backup_database
fi

echo "[INFO] importing SQL..."
if [[ "$DRY_RUN" != "true" ]]; then
  run_mysql_file "$DATABASE" "$SQL_FILE"
fi

echo "[OK] Casdoor SQL import completed."
