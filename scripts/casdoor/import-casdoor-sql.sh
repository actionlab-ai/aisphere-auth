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

usage() {
  cat <<'EOF'
Casdoor SQL 自动导入工具

用途：
  将已经准备好的 Casdoor SQL 初始化文件导入到现有 Casdoor MySQL 数据库，避免手工在 Casdoor UI 中点模型、权限、角色、策略。

用法：
  bash scripts/casdoor/import-casdoor-sql.sh \
    --host 127.0.0.1 \
    --port 3306 \
    --database casdoor \
    --user root \
    --password 'your-password' \
    --sql deployments/casdoor/sql/aisphere-auth-casdoor.sql \
    --backup-before \
    -y

参数：
  --host <host>              Casdoor MySQL 地址，默认 127.0.0.1
  --port <port>              Casdoor MySQL 端口，默认 3306
  --database <db>            Casdoor 数据库名，默认 casdoor
  --user <user>              MySQL 用户，默认 root
  --password <password>      MySQL 密码，也可用环境变量 CASDOOR_MYSQL_PASSWORD
  --sql <file>               要导入的 SQL 文件，默认 deployments/casdoor/sql/aisphere-auth-casdoor.sql
  --mysql-bin <path>         mysql 命令路径，默认 mysql
  --use-docker               不依赖本机 mysql 客户端，改用 docker run mysql:8.0 执行导入
  --docker-image <image>     --use-docker 时使用的镜像，默认 mysql:8.0
  --backup-before            导入前先 mysqldump 备份目标数据库
  --backup-dir <dir>         备份目录，默认 backups/casdoor
  --create-database          导入前执行 CREATE DATABASE IF NOT EXISTS
  --dry-run                  只打印将执行的动作，不真正导入
  -y, --yes                  跳过确认
  -h, --help                 显示帮助

注意：
  1. SQL 文件应来自同版本或兼容版本的 Casdoor 数据库导出。
  2. 直接写死 Casdoor 表结构容易受版本影响，本脚本只负责可靠导入，不在脚本中拼接业务 INSERT。
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
    --dry-run) DRY_RUN="true"; shift ;;
    -y|--yes) YES="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "[ERROR] unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

if [[ -z "$PASSWORD" && -n "${CASDOOR_MYSQL_PASSWORD:-}" ]]; then
  PASSWORD="$CASDOOR_MYSQL_PASSWORD"
fi

if [[ ! -f "$SQL_FILE" ]]; then
  echo "[ERROR] SQL file not found: $SQL_FILE" >&2
  echo "        Put your exported Casdoor SQL at this path or pass --sql <file>." >&2
  exit 1
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

run_mysql_file() {
  local target_db="$1"
  local sql_file="$2"
  if [[ "$USE_DOCKER" == "true" ]]; then
    docker run --rm -i "$DOCKER_IMAGE" mysql --protocol=tcp -h "$HOST" -P "$PORT" -u "$USER" ${PASSWORD:+-p"$PASSWORD"} "$target_db" < "$sql_file"
  else
    MYSQL_PWD="$PASSWORD" "$MYSQL_BIN" --protocol=tcp -h "$HOST" -P "$PORT" -u "$USER" "$target_db" < "$sql_file"
  fi
}

run_mysql_exec() {
  local sql="$1"
  if [[ "$USE_DOCKER" == "true" ]]; then
    docker run --rm -i "$DOCKER_IMAGE" mysql --protocol=tcp -h "$HOST" -P "$PORT" -u "$USER" ${PASSWORD:+-p"$PASSWORD"} -e "$sql"
  else
    MYSQL_PWD="$PASSWORD" "$MYSQL_BIN" --protocol=tcp -h "$HOST" -P "$PORT" -u "$USER" -e "$sql"
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
    docker run --rm -i "$DOCKER_IMAGE" mysqldump --protocol=tcp -h "$HOST" -P "$PORT" -u "$USER" ${PASSWORD:+-p"$PASSWORD"} "$DATABASE" > "$backup_file"
  else
    MYSQL_PWD="$PASSWORD" "$MYSQLDUMP_BIN" --protocol=tcp -h "$HOST" -P "$PORT" -u "$USER" "$DATABASE" > "$backup_file"
  fi
}

cat <<EOF
[INFO] Casdoor SQL import plan
  host      : $HOST
  port      : $PORT
  database  : $DATABASE
  user      : $USER
  sql       : $SQL_FILE
  useDocker : $USE_DOCKER
  backup    : $BACKUP_BEFORE
  dryRun    : $DRY_RUN
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
