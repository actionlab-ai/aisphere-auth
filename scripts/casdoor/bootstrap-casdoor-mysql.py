#!/usr/bin/env python3
"""Bootstrap Casdoor MySQL data for AI Sphere Auth with Python only.

This wrapper avoids PowerShell/Bash glue for Windows users:
1. Render project-specific idempotent Casdoor seed SQL.
2. Optionally back up the target Casdoor database.
3. Import the generated SQL into MySQL using one of these methods:
   - local mysql client
   - dockerized mysql client
   - optional PyMySQL package

It does not embed environment-specific SQL in the repository. Runtime values such as
client_secret, redirect_uri, database host and password are injected via arguments.
"""

from __future__ import annotations

import argparse
import os
import re
import shutil
import subprocess
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Render and import AI Sphere Casdoor seed SQL into MySQL.")

    # Seed rendering options. Kept aligned with render-casdoor-seed.py.
    parser.add_argument("--output", default="deployments/casdoor/sql/aisphere-auth-casdoor.sql", help="Generated seed SQL output path")
    parser.add_argument("--env-output", default=".env.casdoor.generated", help="Optional env output path")
    parser.add_argument("--org", default="aisphere", help="Casdoor organization name")
    parser.add_argument("--org-display-name", default="AI Sphere", help="Casdoor organization display name")
    parser.add_argument("--app-owner", default="admin", help="Casdoor application resource owner")
    parser.add_argument("--app", default="aisphere-auth", help="Casdoor application name")
    parser.add_argument("--app-display-name", default="AI Sphere Auth", help="Casdoor application display name")
    parser.add_argument("--client-id", default="aisphere-auth", help="OAuth client_id")
    parser.add_argument("--client-secret", default="", help="OAuth client_secret. If empty, render script generates one")
    parser.add_argument("--redirect-uri", action="append", default=[], help="Allowed redirect URI. Can be passed multiple times")
    parser.add_argument("--cert", default="cert-built-in", help="Casdoor cert name")
    parser.add_argument("--model", default="aisphere-auth-model", help="Casbin model name")
    parser.add_argument("--permission-id", default="perm_platform_admin", help="Primary permission name for aisphere-auth config")
    parser.add_argument("--admin-user", default="admin", help="Bootstrap admin username to create/bind to role_platform_admin")
    parser.add_argument("--admin-display-name", default="Admin", help="Bootstrap admin display name")
    parser.add_argument("--admin-email", default="admin@example.com", help="Bootstrap admin email")
    parser.add_argument("--admin-password", default="", help="Bootstrap admin password. Requires python bcrypt package to hash.")
    parser.add_argument("--admin-password-hash", default="", help="Precomputed bcrypt hash for bootstrap admin password")
    parser.add_argument("--skip-admin-user-create", action="store_true", help="Do not create/update bootstrap admin user row")
    parser.add_argument("--skip-admin-binding", action="store_true", help="Do not bind admin user to role_platform_admin")
    parser.add_argument("--admin-user-only", action="store_true", help="Only create/update bootstrap admin user and role binding; do not touch application/client secret")
    parser.add_argument("--created-time", default="", help="Fixed Casdoor created_time")

    # Import options.
    parser.add_argument("--seed-only", action="store_true", help="Only render SQL/env files; do not connect to MySQL")
    parser.add_argument("--host", default="127.0.0.1", help="MySQL host")
    parser.add_argument("--port", type=int, default=3306, help="MySQL port")
    parser.add_argument("--database", default="casdoor", help="MySQL database name")
    parser.add_argument("--user", default="root", help="MySQL username")
    parser.add_argument("--password", default="", help="MySQL password. Can also use MYSQL_PWD env")
    parser.add_argument("--mysql-bin", default="mysql", help="mysql client binary path")
    parser.add_argument("--mysqldump-bin", default="mysqldump", help="mysqldump client binary path")
    parser.add_argument("--backup-before", action="store_true", help="Back up target database before import")
    parser.add_argument("--backup-output", default="", help="Backup SQL path. Default: backups/casdoor-<timestamp>.sql")
    parser.add_argument("--yes", "-y", action="store_true", help="Skip confirmation prompt")

    # Docker mysql client fallback.
    parser.add_argument("--use-docker", action="store_true", help="Use dockerized mysql client instead of local mysql")
    parser.add_argument("--docker-image", default="mysql:8.0", help="Docker image that contains mysql/mysqldump clients")

    # Pure Python optional fallback.
    parser.add_argument("--use-pymysql", action="store_true", help="Import with the optional PyMySQL package instead of mysql/docker clients")

    return parser.parse_args()


def assert_identifier(name: str, value: str, pattern: str) -> None:
    if not re.fullmatch(pattern, value):
        raise SystemExit(f"[ERROR] invalid {name}: {value!r}")


def run(cmd: list[str], *, stdin_path: Path | None = None) -> None:
    shown = " ".join(cmd)
    print(f"[RUN] {shown}")
    stdin = None
    try:
        if stdin_path is not None:
            stdin = stdin_path.open("rb")
        proc = subprocess.run(cmd, stdin=stdin)
    finally:
        if stdin is not None:
            stdin.close()
    if proc.returncode != 0:
        raise SystemExit(f"[ERROR] command failed with exit code {proc.returncode}: {shown}")


def write_client_cnf(path: Path, args: argparse.Namespace) -> None:
    password = args.password or os.environ.get("MYSQL_PWD", "")
    path.write_text(
        "[client]\n"
        f"user={args.user}\n"
        f"password={password}\n"
        f"host={args.host}\n"
        f"port={args.port}\n"
        "default-character-set=utf8mb4\n",
        encoding="utf-8",
    )
    try:
        os.chmod(path, 0o600)
    except OSError:
        # Windows ACLs do not map cleanly to chmod; the temp file is removed after use.
        pass


def render_seed(args: argparse.Namespace) -> None:
    script = Path(__file__).with_name("render-casdoor-seed.py")
    cmd = [
        sys.executable,
        str(script),
        "--output", args.output,
        "--env-output", args.env_output,
        "--org", args.org,
        "--org-display-name", args.org_display_name,
        "--app-owner", args.app_owner,
        "--app", args.app,
        "--app-display-name", args.app_display_name,
        "--client-id", args.client_id,
        "--cert", args.cert,
        "--model", args.model,
        "--permission-id", args.permission_id,
        "--admin-user", args.admin_user,
        "--admin-display-name", args.admin_display_name,
        "--admin-email", args.admin_email,
    ]
    if args.client_secret:
        cmd += ["--client-secret", args.client_secret]
    for uri in args.redirect_uri or ["http://127.0.0.1:18080/auth/callback/casdoor"]:
        cmd += ["--redirect-uri", uri]
    if args.admin_password:
        cmd += ["--admin-password", args.admin_password]
    if args.admin_password_hash:
        cmd += ["--admin-password-hash", args.admin_password_hash]
    if args.skip_admin_user_create:
        cmd.append("--skip-admin-user-create")
    if args.skip_admin_binding:
        cmd.append("--skip-admin-binding")
    if args.admin_user_only:
        cmd.append("--admin-user-only")
    if args.created_time:
        cmd += ["--created-time", args.created_time]

    print(f"[INFO] rendering Casdoor seed SQL: {args.output}")
    run(cmd)


def validate_seed_sql(sql: str) -> None:
    required_fragments = [
        "INSERT INTO `organization`",
        "`nav_items`",
        "`user_nav_items`",
        "`widget_items`",
        "`account_menu`",
        "`account_items`",
        '"name":"Organization"',
        '"name":"Email"',
        '"name":"Groups"',
    ]
    missing = [item for item in required_fragments if item not in sql]
    if missing:
        raise SystemExit("[ERROR] generated Casdoor seed SQL is missing required organization defaults: " + ", ".join(missing))


def validate_admin_user_sql(sql: str) -> None:
    required_fragments = [
        "INSERT INTO `user`",
        "`password_type`",
        "'bcrypt'",
    ]
    forbidden_fragments = [
        "INSERT INTO `application`",
        "INSERT INTO `organization`",
        "INSERT INTO `permission`",
    ]
    missing = [item for item in required_fragments if item not in sql]
    forbidden = [item for item in forbidden_fragments if item in sql]
    if missing:
        raise SystemExit("[ERROR] generated admin-user SQL is missing required fragments: " + ", ".join(missing))
    if forbidden:
        raise SystemExit("[ERROR] generated admin-user SQL unexpectedly touches full seed tables: " + ", ".join(forbidden))


def backup_path(args: argparse.Namespace) -> Path:
    if args.backup_output:
        return Path(args.backup_output)
    ts = datetime.now(timezone.utc).strftime("%Y%m%d%H%M%S")
    return Path("backups") / f"casdoor-{ts}.sql"


def docker_mount_path(path: Path) -> str:
    # Docker Desktop accepts absolute Windows paths in -v. Resolve for stable mounts.
    return str(path.resolve())


def import_with_local_mysql(args: argparse.Namespace, cnf: Path, sql: Path) -> None:
    if shutil.which(args.mysql_bin) is None and not Path(args.mysql_bin).exists():
        raise SystemExit(f"[ERROR] mysql client not found: {args.mysql_bin}. Install MySQL client, use --use-docker, use --use-pymysql, or run --seed-only and import on the server.")

    if args.backup_before:
        dump_bin = args.mysqldump_bin
        if shutil.which(dump_bin) is None and not Path(dump_bin).exists():
            raise SystemExit(f"[ERROR] mysqldump client not found: {dump_bin}. Install MySQL client tools or disable --backup-before.")
        out = backup_path(args)
        out.parent.mkdir(parents=True, exist_ok=True)
        run([dump_bin, f"--defaults-extra-file={cnf}", "--single-transaction", args.database, f"--result-file={out}"])
        print(f"[OK] backup written: {out}")

    run([args.mysql_bin, f"--defaults-extra-file={cnf}", args.database], stdin_path=sql)


def import_with_docker_mysql(args: argparse.Namespace, cnf: Path, sql: Path) -> None:
    if shutil.which("docker") is None:
        raise SystemExit("[ERROR] docker command not found")

    cnf_container = "/tmp/mysql-client.cnf"
    sql_container = "/tmp/seed.sql"

    if args.backup_before:
        out = backup_path(args)
        out.parent.mkdir(parents=True, exist_ok=True)
        cmd = [
            "docker", "run", "--rm",
            "-v", f"{docker_mount_path(cnf)}:{cnf_container}:ro",
            args.docker_image,
            "mysqldump", f"--defaults-extra-file={cnf_container}", "--single-transaction", args.database,
        ]
        print(f"[RUN] {' '.join(cmd)} > {out}")
        with out.open("wb") as f:
            proc = subprocess.run(cmd, stdout=f)
        if proc.returncode != 0:
            raise SystemExit(f"[ERROR] docker mysqldump failed with exit code {proc.returncode}")
        print(f"[OK] backup written: {out}")

    run([
        "docker", "run", "--rm", "-i",
        "-v", f"{docker_mount_path(cnf)}:{cnf_container}:ro",
        "-v", f"{docker_mount_path(sql)}:{sql_container}:ro",
        args.docker_image,
        "mysql", f"--defaults-extra-file={cnf_container}", args.database,
    ])


def iter_sql_statements(sql: Path) -> list[str]:
    """Split generated seed SQL into statements without requiring mysql CLI.

    The seed generator intentionally emits plain SET/INSERT/DELETE statements with SQL comments.
    This parser is conservative enough for that generated SQL and avoids splitting on
    semicolons inside quoted strings.
    """
    text = sql.read_text(encoding="utf-8")
    statements: list[str] = []
    buf: list[str] = []
    in_single = False
    in_double = False
    in_line_comment = False
    i = 0
    while i < len(text):
        ch = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ""

        if in_line_comment:
            if ch in "\r\n":
                in_line_comment = False
                buf.append(ch)
            i += 1
            continue

        if not in_single and not in_double and ch == "-" and nxt == "-":
            in_line_comment = True
            i += 2
            continue

        if ch == "'" and not in_double:
            buf.append(ch)
            if in_single and nxt == "'":
                buf.append(nxt)
                i += 2
                continue
            if i == 0 or text[i - 1] != "\\":
                in_single = not in_single
            i += 1
            continue

        if ch == '"' and not in_single:
            buf.append(ch)
            if i == 0 or text[i - 1] != "\\":
                in_double = not in_double
            i += 1
            continue

        if ch == ";" and not in_single and not in_double:
            statement = "".join(buf).strip()
            if statement:
                statements.append(statement)
            buf = []
            i += 1
            continue

        buf.append(ch)
        i += 1

    tail = "".join(buf).strip()
    if tail:
        statements.append(tail)
    return statements


def import_with_pymysql(args: argparse.Namespace, sql: Path) -> None:
    if args.backup_before:
        raise SystemExit("[ERROR] --backup-before requires mysqldump. Use local mysql tools, disable --backup-before, or run --seed-only and back up on the server.")

    try:
        import pymysql  # type: ignore[import-not-found]
    except ImportError:
        raise SystemExit("[ERROR] PyMySQL is not installed. Install it with: python -m pip install PyMySQL")

    password = args.password or os.environ.get("MYSQL_PWD", "")
    statements = iter_sql_statements(sql)
    if not statements:
        raise SystemExit(f"[ERROR] no SQL statements found in {sql}")

    print(f"[INFO] importing {len(statements)} SQL statements with PyMySQL into {args.user}@{args.host}:{args.port}/{args.database}")
    conn = pymysql.connect(
        host=args.host,
        port=args.port,
        user=args.user,
        password=password,
        database=args.database,
        charset="utf8mb4",
        autocommit=False,
    )
    try:
        with conn.cursor() as cur:
            for idx, statement in enumerate(statements, 1):
                try:
                    cur.execute(statement)
                except Exception as exc:  # pragma: no cover - depends on live MySQL
                    preview = statement.replace("\n", " ")[:240]
                    raise RuntimeError(f"statement {idx} failed: {preview}") from exc
        conn.commit()
    except Exception:
        conn.rollback()
        raise
    finally:
        conn.close()


def confirm(args: argparse.Namespace) -> None:
    if args.yes:
        return
    print("[WARN] This will import idempotent Casdoor seed data into MySQL.")
    print(f"       target: {args.user}@{args.host}:{args.port}/{args.database}")
    if args.use_pymysql:
        print("       method: PyMySQL")
    elif args.use_docker:
        print("       method: docker mysql client")
    else:
        print("       method: local mysql client")
    value = input("Type 'yes' to continue: ").strip().lower()
    if value != "yes":
        raise SystemExit("[INFO] cancelled")


def main() -> int:
    args = parse_args()
    if args.use_docker and args.use_pymysql:
        raise SystemExit("[ERROR] --use-docker and --use-pymysql cannot be used together")
    assert_identifier("database", args.database, r"[A-Za-z0-9_]+")
    assert_identifier("user", args.user, r"[A-Za-z0-9_.-]+")
    if args.port <= 0 or args.port > 65535:
        raise SystemExit(f"[ERROR] invalid port: {args.port}")

    render_seed(args)
    sql = Path(args.output)
    generated_sql = sql.read_text(encoding="utf-8")
    if args.admin_user_only:
        validate_admin_user_sql(generated_sql)
    else:
        validate_seed_sql(generated_sql)
    if args.seed_only:
        print(f"[OK] seed-only completed: {sql}")
        if args.env_output:
            print(f"[OK] env values written: {args.env_output}")
        return 0

    confirm(args)

    if args.use_pymysql:
        import_with_pymysql(args, sql)
    else:
        with tempfile.TemporaryDirectory(prefix="aisphere-casdoor-") as d:
            cnf = Path(d) / "mysql-client.cnf"
            write_client_cnf(cnf, args)
            if args.use_docker:
                import_with_docker_mysql(args, cnf, sql)
            else:
                import_with_local_mysql(args, cnf, sql)

    print("[OK] Casdoor seed imported")
    if args.env_output:
        print(f"[INFO] copy values from {args.env_output} into configs/config.yaml or runtime env")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
