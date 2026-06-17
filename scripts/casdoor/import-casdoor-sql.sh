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
SEED="false"
SEED_ONLY="false"
SEED_OUTPUT=""
SEED_ENV_OUTPUT=""
SEED_ORG="aisphere"
SEED_ORG_DISPLAY_NAME="AI Sphere"
SEED_APP_OWNER="admin"
SEED_APP="aisphere-auth"
SEED_APP_DISPLAY_NAME="AI Sphere Auth"
SEED_CLIENT_ID="aisphere-auth"
SEED_CLIENT_SECRET=""
SEED_CERT="cert-built-in"
SEED_MODEL="aisphere-auth-model"
SEED_PERMISSION_ID="perm_platform_admin"
SEED_ADMIN_USER="admin"
SEED_SKIP_ADMIN_BINDING="false"
SEED_REDIRECT_URIS=()
MYSQL_CNF=""
DOCKER_CNF_DIR=""
TEMP_SQL_FILE=""

usage() {
  cat <<'EOF'
Casdoor SQL 自动导入工具

用途：
  1. 推荐：按 aisphere-auth 当前项目模型生成 Casdoor seed SQL，并导入现有 Casdoor MySQL。
  2. 兼容：导入已经准备好的 SQL 文件。
  3. 迁移：从完整 Casdoor dump 中预处理出 data-only SQL。

推荐开箱即用流程：
  bash scripts/casdoor/import-casdoor-sql.sh \
    --seed \
    --seed-org aisphere \
    --seed-app aisphere-auth \
    --seed-client-id aisphere-auth \
    --seed-client-secret 'replace-with-oauth-secret' \
    --seed-redirect-uri http://127.0.0.1:18080/auth/callback/casdoor \
    --host 127.0.0.1 \
    --port 3306 \
    --database casdoor \
    --user root \
    --password 'your-casdoor-mysql-password' \
    --backup-before \
    -y

通用导入参数：
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

项目 seed 参数（推荐）：
  --seed                         不依赖历史 dump，按项目内置模型生成组织、应用、模型、角色、权限、绑定 SQL
  --seed-output <file>           生成的 seed SQL 输出路径，默认写到临时文件
  --seed-env-output <file>       同时输出 aisphere-auth 需要使用的环境变量文件
  --seed-only                    只生成 seed SQL，不执行导入
  --seed-org <name>              Casdoor 组织名，默认 aisphere
  --seed-org-display-name <name> 组织显示名
  --seed-app-owner <owner>       Application owner，通常是 admin
  --seed-app <name>              Casdoor Application 名称，默认 aisphere-auth
  --seed-app-display-name <name> Application 显示名
  --seed-client-id <id>          OAuth client_id
  --seed-client-secret <secret>  OAuth client_secret。生产建议从 Secret 注入，不要写入仓库
  --seed-redirect-uri <uri>      OAuth 回调地址，可重复传多个
  --seed-cert <name>             Casdoor 证书名，默认 cert-built-in
  --seed-model <name>            Casbin model 名称
  --seed-permission-id <name>    aisphere-auth 默认 permission 名称
  --seed-admin-user <name>       绑定平台管理员角色的已有用户，默认 admin
  --seed-skip-admin-binding      不绑定默认 admin 用户

完整 dump 预处理参数（迁移用，不推荐作为默认初始化）：
  --prepare-dump             导入前先把原始 dump 处理成可导入 SQL
  --prepare-mode <mode>      data-only 或 full，默认 data-only
  --prepared-sql <file>      处理后的 SQL 输出路径。默认写到临时文件
  --prepare-keywords <list>  data-only 提取关键字，默认 aisphere,skillhub
  --prepare-include-users    data-only 模式也提取匹配用户。会复制密码 hash，需谨慎。
  --prepare-only             只生成处理后的 SQL，不执行导入

注意：
  1. 项目 seed 是开箱即用推荐方式，SQL 由参数生成，不应提交包含真实 secret 的 SQL。
  2. 完整 dump 迁移仅用于搬迁旧环境配置，不应作为新环境默认初始化。
  3. 生产导入建议开启 --backup-before。
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
    --seed) SEED="true"; shift ;;
    --seed-output) SEED_OUTPUT="$2"; shift 2 ;;
    --seed-env-output) SEED_ENV_OUTPUT="$2"; shift 2 ;;
    --seed-only) SEED_ONLY="true"; shift ;;
    --seed-org) SEED_ORG="$2"; shift 2 ;;
    --seed-org-display-name) SEED_ORG_DISPLAY_NAME="$2"; shift 2 ;;
    --seed-app-owner) SEED_APP_OWNER="$2"; shift 2 ;;
    --seed-app) SEED_APP="$2"; shift 2 ;;
    --seed-app-display-name) SEED_APP_DISPLAY_NAME="$2"; shift 2 ;;
    --seed-client-id) SEED_CLIENT_ID="$2"; shift 2 ;;
    --seed-client-secret) SEED_CLIENT_SECRET="$2"; shift 2 ;;
    --seed-redirect-uri) SEED_REDIRECT_URIS+=("$2"); shift 2 ;;
    --seed-cert) SEED_CERT="$2"; shift 2 ;;
    --seed-model) SEED_MODEL="$2"; shift 2 ;;
    --seed-permission-id) SEED_PERMISSION_ID="$2"; shift 2 ;;
    --seed-admin-user) SEED_ADMIN_USER="$2"; shift 2 ;;
    --seed-skip-admin-binding) SEED_SKIP_ADMIN_BINDING="true"; shift ;;
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

find_python() {
  if command -v python3 >/dev/null 2>&1; then
    echo python3
    return 0
  fi
  if command -v python >/dev/null 2>&1; then
    echo python
    return 0
  fi
  return 1
}

validate_identifier "database" "$DATABASE" '^[A-Za-z0-9_]+$'
validate_identifier "user" "$USER" '^[A-Za-z0-9_-]+$'
validate_identifier "port" "$PORT" '^[0-9]+$'
case "$PREPARE_MODE" in
  data-only|full) ;;
  *) echo "[ERROR] invalid --prepare-mode: $PREPARE_MODE" >&2; exit 1 ;;
esac
if [[ "$SEED" == "true" && "$PREPARE_DUMP" == "true" ]]; then
  echo "[ERROR] --seed and --prepare-dump are mutually exclusive" >&2
  exit 1
fi

cleanup() {
  if [[ -n "${MYSQL_CNF}" && -f "${MYSQL_CNF}" ]]; then rm -f "${MYSQL_CNF}"; fi
  if [[ -n "${DOCKER_CNF_DIR}" && -d "${DOCKER_CNF_DIR}" ]]; then rm -rf "${DOCKER_CNF_DIR}"; fi
  if [[ -n "${TEMP_SQL_FILE}" && -f "${TEMP_SQL_FILE}" ]]; then rm -f "${TEMP_SQL_FILE}"; fi
}
trap cleanup EXIT

render_seed_sql() {
  local tool="scripts/casdoor/render-casdoor-seed.py"
  if [[ ! -f "$tool" ]]; then
    echo "[ERROR] seed renderer not found: $tool" >&2
    exit 1
  fi
  local py
  if ! py="$(find_python)"; then
    echo "[ERROR] python3 or python is required for --seed" >&2
    exit 1
  fi
  if [[ -z "$SEED_OUTPUT" ]]; then
    TEMP_SQL_FILE="$(mktemp /tmp/aisphere-casdoor-seed.XXXXXX.sql)"
    SEED_OUTPUT="$TEMP_SQL_FILE"
  fi
  local args=("$tool" --output "$SEED_OUTPUT" --org "$SEED_ORG" --org-display-name "$SEED_ORG_DISPLAY_NAME" --app-owner "$SEED_APP_OWNER" --app "$SEED_APP" --app-display-name "$SEED_APP_DISPLAY_NAME" --client-id "$SEED_CLIENT_ID" --cert "$SEED_CERT" --model "$SEED_MODEL" --permission-id "$SEED_PERMISSION_ID" --admin-user "$SEED_ADMIN_USER")
  if [[ -n "$SEED_CLIENT_SECRET" ]]; then args+=(--client-secret "$SEED_CLIENT_SECRET"); fi
  if [[ -n "$SEED_ENV_OUTPUT" ]]; then args+=(--env-output "$SEED_ENV_OUTPUT"); fi
  if [[ "$SEED_SKIP_ADMIN_BINDING" == "true" ]]; then args+=(--skip-admin-binding); fi
  if [[ ${#SEED_REDIRECT_URIS[@]} -eq 0 ]]; then
    args+=(--redirect-uri "http://127.0.0.1:18080/auth/callback/casdoor")
  else
    for uri in "${SEED_REDIRECT_URIS[@]}"; do args+=(--redirect-uri "$uri"); done
  fi
  echo "[INFO] rendering project Casdoor seed SQL: $SEED_OUTPUT"
  "$py" "${args[@]}"
  SQL_FILE="$SEED_OUTPUT"
  if [[ "$SEED_ONLY" == "true" ]]; then
    echo "[OK] seed-only completed: $SQL_FILE"
    exit 0
  fi
}

prepare_dump_sql() {
  local prepare_tool="scripts/casdoor/prepare-casdoor-sql.py"
  if [[ ! -f "$prepare_tool" ]]; then
    echo "[ERROR] prepare tool not found: $prepare_tool" >&2
    exit 1
  fi
  local py
  if ! py="$(find_python)"; then
    echo "[ERROR] python3 or python is required for --prepare-dump" >&2
    exit 1
  fi
  if [[ -z "$PREPARED_SQL" ]]; then
    TEMP_SQL_FILE="$(mktemp /tmp/aisphere-casdoor-prepared.XXXXXX.sql)"
    PREPARED_SQL="$TEMP_SQL_FILE"
  fi
  echo "[INFO] preparing Casdoor SQL: mode=$PREPARE_MODE output=$PREPARED_SQL"
  local args=("$prepare_tool" --input "$SQL_FILE" --output "$PREPARED_SQL" --mode "$PREPARE_MODE" --keywords "$PREPARE_KEYWORDS")
  if [[ "$PREPARE_INCLUDE_USERS" == "true" ]]; then args+=(--include-users); fi
  "$py" "${args[@]}"
  SQL_FILE="$PREPARED_SQL"
  if [[ "$PREPARE_ONLY" == "true" ]]; then
    echo "[OK] prepare-only completed: $SQL_FILE"
    exit 0
  fi
}

if [[ "$SEED" == "true" ]]; then
  render_seed_sql
elif [[ ! -f "$SQL_FILE" ]]; then
  echo "[ERROR] SQL file not found: $SQL_FILE" >&2
  echo "        Use --seed to render a project seed, or pass --sql <file>." >&2
  exit 1
fi

if [[ "$PREPARE_DUMP" == "true" ]]; then
  prepare_dump_sql
fi

if [[ "$ALLOW_DESTRUCTIVE" != "true" ]]; then
  if grep -Eiq '(^|[[:space:]])(DROP[[:space:]]+TABLE|CREATE[[:space:]]+TABLE|DROP[[:space:]]+DATABASE)' "$SQL_FILE"; then
    echo "[ERROR] SQL contains destructive schema statements." >&2
    echo "        Use --seed for project bootstrap, --prepare-dump --prepare-mode data-only for migration, or pass --allow-destructive deliberately." >&2
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
  if [[ "$DRY_RUN" == "true" ]]; then return 0; fi
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
  seed          : $SEED
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
