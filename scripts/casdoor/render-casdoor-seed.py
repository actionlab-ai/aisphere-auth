#!/usr/bin/env python3
"""Render project-specific Casdoor bootstrap SQL for AI Sphere Auth.

This tool creates an idempotent baseline for the current project: organization,
OAuth application, Casbin model, common roles, common permissions and optional
role-user binding.

The generated SQL is intentionally not committed as an environment-specific file.
Use command-line parameters or CI/CD secrets to inject endpoint-specific values
such as client_id, client_secret and redirect_uri at deployment time.
"""

from __future__ import annotations

import argparse
import json
import re
import secrets
from datetime import datetime, timezone
from pathlib import Path

MODEL_TEXT = """[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (g(r.sub, p.sub) || r.sub == p.sub) &&
    (p.obj == "*" || keyMatch(r.obj, p.obj)) &&
    (p.act == "*" || r.act == p.act)"""

COMMON_ROLES = [
    ("role_platform_admin", "平台管理员", "平台全局管理员，允许所有对象和动作"),
    ("role_platform_viewer", "平台只读", "平台全局只读角色"),
    ("role_skillhub_admin", "SkillHub 管理员", "SkillHub 全部管理权限"),
    ("role_skillhub_editor", "SkillHub 编辑者", "SkillHub 技能、知识库等写入权限"),
    ("role_skillhub_viewer", "SkillHub 只读", "SkillHub 只读权限"),
    ("role_agentruntime_admin", "AgentRuntime 管理员", "AgentRuntime 全部管理权限"),
    ("role_agentruntime_operator", "AgentRuntime 操作者", "AgentRuntime 运行、停止、查看权限"),
    ("role_agentruntime_viewer", "AgentRuntime 只读", "AgentRuntime 只读权限"),
    ("role_sqlhub_admin", "SQLHub 管理员", "SQLHub 全部管理权限"),
    ("role_sqlhub_viewer", "SQLHub 只读", "SQLHub 只读权限"),
    ("role_modelgateway_admin", "ModelGateway 管理员", "模型网关全部管理权限"),
    ("role_modelgateway_viewer", "ModelGateway 只读", "模型网关只读权限"),
    ("role_portal_admin", "Portal 管理员", "门户全部管理权限"),
    ("role_portal_viewer", "Portal 只读", "门户只读权限"),
]

COMMON_PERMISSIONS = [
    ("perm_platform_admin", "平台管理员策略", ["role_platform_admin"], ["*"], ["*"]),
    ("perm_platform_viewer", "平台只读策略", ["role_platform_viewer"], ["portal:*", "skillhub:*", "agentruntime:*", "sqlhub:*", "modelgateway:*"], ["read", "view", "list", "admin:read"]),
    ("perm_skillhub_admin", "SkillHub 管理策略", ["role_skillhub_admin"], ["skillhub:*"], ["*"]),
    ("perm_skillhub_editor", "SkillHub 编辑策略", ["role_skillhub_editor"], ["skillhub:skill:*", "skillhub:knowledge:*", "skillhub:workflow:*"], ["read", "view", "list", "write", "create", "update", "approve", "admin:read", "admin:write"]),
    ("perm_skillhub_viewer", "SkillHub 只读策略", ["role_skillhub_viewer"], ["skillhub:*"], ["read", "view", "list", "admin:read"]),
    ("perm_agentruntime_admin", "AgentRuntime 管理策略", ["role_agentruntime_admin"], ["agentruntime:*"], ["*"]),
    ("perm_agentruntime_operator", "AgentRuntime 操作策略", ["role_agentruntime_operator"], ["agentruntime:run:*", "agentruntime:session:*", "agentruntime:agent:*"], ["read", "view", "list", "run", "stop", "admin:read"]),
    ("perm_agentruntime_viewer", "AgentRuntime 只读策略", ["role_agentruntime_viewer"], ["agentruntime:*"], ["read", "view", "list", "admin:read"]),
    ("perm_sqlhub_admin", "SQLHub 管理策略", ["role_sqlhub_admin"], ["sqlhub:*"], ["*"]),
    ("perm_sqlhub_viewer", "SQLHub 只读策略", ["role_sqlhub_viewer"], ["sqlhub:*"], ["read", "view", "list", "admin:read"]),
    ("perm_modelgateway_admin", "ModelGateway 管理策略", ["role_modelgateway_admin"], ["modelgateway:*"], ["*"]),
    ("perm_modelgateway_viewer", "ModelGateway 只读策略", ["role_modelgateway_viewer"], ["modelgateway:*"], ["read", "view", "list", "admin:read"]),
    ("perm_portal_admin", "Portal 管理策略", ["role_portal_admin"], ["portal:*"], ["*"]),
    ("perm_portal_viewer", "Portal 只读策略", ["role_portal_viewer"], ["portal:*"], ["read", "view", "list", "admin:read"]),
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Render AI Sphere Casdoor bootstrap SQL.")
    parser.add_argument("--output", default="deployments/casdoor/sql/aisphere-auth-casdoor.sql", help="Output SQL file")
    parser.add_argument("--env-output", default="", help="Optional output env file with generated/selected config values")
    parser.add_argument("--org", default="aisphere", help="Casdoor organization name")
    parser.add_argument("--org-display-name", default="AI Sphere", help="Casdoor organization display name")
    parser.add_argument("--app-owner", default="admin", help="Casdoor application resource owner, usually admin")
    parser.add_argument("--app", default="aisphere-auth", help="Casdoor application name")
    parser.add_argument("--app-display-name", default="AI Sphere Auth", help="Casdoor application display name")
    parser.add_argument("--client-id", default="aisphere-auth", help="OAuth client_id for aisphere-auth")
    parser.add_argument("--client-secret", default="", help="OAuth client_secret. If empty, a random secret is generated.")
    parser.add_argument("--redirect-uri", action="append", default=[], help="Allowed redirect URI. Can be passed multiple times.")
    parser.add_argument("--cert", default="cert-built-in", help="Casdoor cert name used by the application")
    parser.add_argument("--model", default="aisphere-auth-model", help="Casbin model name")
    parser.add_argument("--permission-id", default="perm_platform_admin", help="Primary permission name used by AISPHERE_CASDOOR_PERMISSION_ID")
    parser.add_argument("--admin-user", default="admin", help="Existing bootstrap admin username to bind to role_platform_admin")
    parser.add_argument("--skip-admin-binding", action="store_true", help="Do not bind admin user to role_platform_admin")
    parser.add_argument("--created-time", default="", help="Fixed Casdoor created_time. Default: current UTC ISO time")
    return parser.parse_args()


def validate_identifier(name: str, value: str) -> None:
    if not re.fullmatch(r"[A-Za-z0-9_.-]+", value):
        raise SystemExit(f"[ERROR] invalid {name}: {value!r}. Only letters, digits, '_', '-' and '.' are allowed.")


def sql_quote(value: str | int | float | None) -> str:
    if value is None:
        return "NULL"
    if isinstance(value, (int, float)):
        return str(value)
    return "'" + value.replace("\\", "\\\\").replace("'", "''") + "'"


def sql_json(value: object) -> str:
    return sql_quote(json.dumps(value, ensure_ascii=False, separators=(",", ":")))


def insert_on_duplicate(table: str, columns: list[str], values: list[str], update_columns: list[str] | None = None) -> str:
    cols = ", ".join(f"`{c}`" for c in columns)
    vals = ", ".join(values)
    if update_columns is None:
        update_columns = [c for c in columns if c not in {"owner", "name"}]
    updates = ", ".join(f"`{c}`=VALUES(`{c}`)" for c in update_columns)
    return f"INSERT INTO `{table}` ({cols}) VALUES ({vals}) ON DUPLICATE KEY UPDATE {updates};"


def role_id(org: str, role_name: str) -> str:
    return f"{org}/{role_name}"


def render(args: argparse.Namespace) -> tuple[str, str, list[str]]:
    for name in ["org", "app_owner", "app", "client_id", "model", "permission_id", "admin_user"]:
        validate_identifier(name, getattr(args, name))

    client_secret = args.client_secret.strip() or "aisphere_" + secrets.token_urlsafe(32)
    created_time = args.created_time.strip() or datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    redirect_uris = args.redirect_uri or ["http://127.0.0.1:18080/auth/callback/casdoor"]
    admin_subject = f"{args.org}/{args.admin_user}"

    lines: list[str] = [
        "-- AI Sphere Auth Casdoor bootstrap SQL",
        "-- Generated by scripts/casdoor/render-casdoor-seed.py",
        "-- Scope: organization, application, model, roles, permissions and role-user binding.",
        "-- This file is idempotent and does not drop or create tables.",
        "-- Environment-specific values are injected by generator parameters at deployment time.",
        "SET FOREIGN_KEY_CHECKS=0;",
        "",
        "-- 1. Organization",
    ]

    lines.append(insert_on_duplicate(
        "organization",
        ["owner", "name", "created_time", "display_name", "website_url", "password_type", "country_codes", "default_application", "user_types", "tags", "languages", "default_avatar", "use_email_as_username", "is_profile_public", "account_menu"],
        [sql_quote("admin"), sql_quote(args.org), sql_quote(created_time), sql_quote(args.org_display_name), sql_quote(""), sql_quote("bcrypt"), sql_json(["CN"]), sql_quote(args.app), sql_quote("[]"), sql_quote("[]"), sql_quote('["zh","en"]'), sql_quote("https://cdn.casbin.org/img/casbin.svg"), "0", "1", sql_quote("Normal")],
    ))

    lines.extend(["", "-- 2. OAuth application"])
    lines.append(insert_on_duplicate(
        "application",
        ["owner", "name", "created_time", "display_name", "category", "type", "scopes", "logo", "title", "organization", "cert", "enable_password", "enable_sign_up", "enable_signin_session", "grant_types", "signin_methods", "signup_items", "signin_items", "tags", "client_id", "client_secret", "redirect_uris", "token_format", "token_signing_method", "expire_in_hours", "refresh_expire_in_hours", "cookie_expire_in_hours", "is_shared"],
        [sql_quote(args.app_owner), sql_quote(args.app), sql_quote(created_time), sql_quote(args.app_display_name), sql_quote("Default"), sql_quote("All"), sql_json(["openid", "profile", "email"]), sql_quote("https://cdn.casbin.org/img/casdoor-logo_1185x256.png"), sql_quote(args.app_display_name), sql_quote(args.org), sql_quote(args.cert), "1", "1", "1", sql_json(["authorization_code", "refresh_token", "password", "client_credentials"]), sql_json([{"name": "Password", "displayName": "Password", "rule": "All"}, {"name": "Verification code", "displayName": "Verification code", "rule": "All"}]), sql_json([{"name": "Username", "visible": True, "required": True, "rule": "None"}, {"name": "Password", "visible": True, "required": True, "rule": "None"}, {"name": "Email", "visible": True, "required": False, "rule": "Normal"}]), sql_json([{"name": "Username", "visible": True, "rule": "None"}, {"name": "Password", "visible": True, "rule": "None"}, {"name": "Login button", "visible": True, "rule": "None"}]), sql_quote("[]"), sql_quote(args.client_id), sql_quote(client_secret), sql_json(redirect_uris), sql_quote("JWT"), sql_quote("RS256"), "24", "168", "24", "0"],
    ))

    lines.extend(["", "-- 3. Casbin model"])
    lines.append(insert_on_duplicate(
        "model",
        ["owner", "name", "created_time", "display_name", "description", "model_text"],
        [sql_quote(args.org), sql_quote(args.model), sql_quote(created_time), sql_quote("AI Sphere RBAC Model"), sql_quote("AI Sphere unified RBAC model for object/action checks"), sql_quote(MODEL_TEXT)],
    ))

    lines.extend(["", "-- 4. Roles and role-user bindings"])
    for name, display_name, desc in COMMON_ROLES:
        users = [admin_subject] if (name == "role_platform_admin" and not args.skip_admin_binding) else []
        lines.append(insert_on_duplicate(
            "role",
            ["owner", "name", "created_time", "display_name", "description", "users", "groups", "roles", "domains", "is_enabled"],
            [sql_quote(args.org), sql_quote(name), sql_quote(created_time), sql_quote(display_name), sql_quote(desc), sql_json(users), sql_quote("[]"), sql_quote("[]"), sql_quote("[]"), "1"],
        ))

    lines.extend(["", "-- 5. Permissions / policies"])
    for name, display_name, roles, resources, actions in COMMON_PERMISSIONS:
        lines.append(insert_on_duplicate(
            "permission",
            ["owner", "name", "created_time", "display_name", "description", "users", "groups", "roles", "domains", "model", "adapter", "resource_type", "resources", "actions", "effect", "is_enabled", "submitter", "approver", "approve_time", "state"],
            [sql_quote(args.org), sql_quote(name), sql_quote(created_time), sql_quote(display_name), sql_quote(display_name), sql_quote("[]"), sql_quote("[]"), sql_json([role_id(args.org, r) for r in roles]), sql_quote("[]"), sql_quote(args.model), sql_quote(""), sql_quote("Application"), sql_json(resources), sql_json(actions), sql_quote("Allow"), "1", sql_quote(args.admin_user), sql_quote(args.admin_user), sql_quote(created_time), sql_quote("Approved")],
        ))

    config_lines = [
        f"AISPHERE_CASDOOR_OWNER={args.org}",
        f"AISPHERE_CASDOOR_APPLICATION={args.app}",
        f"AISPHERE_CASDOOR_CLIENT_ID={args.client_id}",
        f"AISPHERE_CASDOOR_CLIENT_SECRET={client_secret}",
        f"AISPHERE_CASDOOR_PERMISSION_ID={args.org}/{args.permission_id}",
    ]

    lines.extend([
        "",
        "SET FOREIGN_KEY_CHECKS=1;",
        "",
        "-- Generated non-secret values for aisphere-auth config:",
        f"-- AISPHERE_CASDOOR_OWNER={args.org}",
        f"-- AISPHERE_CASDOOR_APPLICATION={args.app}",
        f"-- AISPHERE_CASDOOR_CLIENT_ID={args.client_id}",
        f"-- AISPHERE_CASDOOR_PERMISSION_ID={args.org}/{args.permission_id}",
        "-- AISPHERE_CASDOOR_CLIENT_SECRET is intentionally not printed in SQL comments.",
    ])
    return "\n".join(lines) + "\n", client_secret, config_lines


def write_env_file(path: str, config_lines: list[str]) -> None:
    out = Path(path)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text("\n".join(config_lines) + "\n", encoding="utf-8")


def main() -> int:
    args = parse_args()
    out = Path(args.output)
    out.parent.mkdir(parents=True, exist_ok=True)
    sql, client_secret, config_lines = render(args)
    out.write_text(sql, encoding="utf-8")
    if args.env_output.strip():
        write_env_file(args.env_output, config_lines)
    print(f"[OK] wrote Casdoor seed SQL: {out}")
    print(f"[INFO] org={args.org} app={args.app} client_id={args.client_id}")
    print(f"[INFO] permission_id={args.org}/{args.permission_id}")
    if args.env_output.strip():
        print(f"[OK] wrote aisphere-auth env values: {args.env_output}")
    elif not args.client_secret.strip():
        print("[WARN] generated a random client_secret. Re-run with --env-output <file> or pass --client-secret explicitly so aisphere-auth can use the same secret.")
    else:
        print("[INFO] client_secret was injected by parameter and is not repeated in SQL comments.")
    if not args.skip_admin_binding:
        print(f"[INFO] bound existing user {args.org}/{args.admin_user} to role_platform_admin. Create/change the user password in Casdoor UI or via your own user-management flow.")
    _ = client_secret
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
