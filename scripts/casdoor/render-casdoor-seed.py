#!/usr/bin/env python3
"""Render project-specific Casdoor bootstrap SQL for AI Sphere Auth.

This tool creates an idempotent baseline for the current project: organization,
OAuth application, optional bootstrap user, Casbin model, common roles, common
permissions/policies, permission_rule rows and role-user binding.

The generated SQL is intentionally not committed as an environment-specific file.
Use command-line parameters or CI/CD secrets to inject endpoint-specific values
such as client_id, client_secret, redirect_uri and bootstrap password.
"""

from __future__ import annotations

import argparse
import json
import re
import secrets
import uuid
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
m = (g(r.sub, p.sub) || r.sub == p.sub || keyMatch(r.sub, p.sub)) &&
    (p.obj == "*" || keyMatch(r.obj, p.obj)) &&
    (p.act == "*" || r.act == p.act)"""

SCOPE_ITEMS = [
    {
        "name": "openid",
        "displayName": "OpenID",
        "description": "OpenID Connect ID token scope",
        "tools": [],
    },
    {
        "name": "profile",
        "displayName": "Profile",
        "description": "Basic profile claims such as name and displayName",
        "tools": [],
    },
    {
        "name": "email",
        "displayName": "Email",
        "description": "Email claim scope",
        "tools": [],
    },
]

DEFAULT_ACCOUNT_ITEMS = [
    {"name": "Organization", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "ID", "visible": True, "viewRule": "Public", "modifyRule": "Immutable"},
    {"name": "Name", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Display name", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "First name", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Last name", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Avatar", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "User type", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Password", "visible": True, "viewRule": "Self", "modifyRule": "Self"},
    {"name": "Email", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Phone", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Country code", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Country/Region", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Location", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Address", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Addresses", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Affiliation", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Title", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "ID card type", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "ID card", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "ID card info", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Real name", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Is verified", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Homepage", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Bio", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Tag", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Language", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Gender", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Birthday", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Education", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Score", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Karma", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Ranking", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Currency", "visible": True, "viewRule": "Public", "modifyRule": "Self"},
    {"name": "Is default avatar", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Is online", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Is admin", "visible": True, "viewRule": "Admin", "modifyRule": "Admin"},
    {"name": "Is forbidden", "visible": True, "viewRule": "Admin", "modifyRule": "Admin"},
    {"name": "Is deleted", "visible": True, "viewRule": "Admin", "modifyRule": "Admin"},
    {"name": "Signup application", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Register type", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Register source", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Created IP", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Last signin time", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Last signin IP", "visible": True, "viewRule": "Public", "modifyRule": "Admin"},
    {"name": "Properties", "visible": True, "viewRule": "Admin", "modifyRule": "Admin"},
    {"name": "Roles", "visible": True, "viewRule": "Admin", "modifyRule": "Admin"},
    {"name": "Permissions", "visible": True, "viewRule": "Admin", "modifyRule": "Admin"},
    {"name": "Groups", "visible": True, "viewRule": "Admin", "modifyRule": "Admin"},
    {"name": "Consents", "visible": True, "viewRule": "Self", "modifyRule": "Self"},
    {"name": "IP whitelist", "visible": True, "viewRule": "Admin", "modifyRule": "Admin"},
    {"name": "Multi-factor authentication", "visible": True, "viewRule": "Self", "modifyRule": "Self"},
    {"name": "WebAuthn credentials", "visible": True, "viewRule": "Self", "modifyRule": "Self"},
    {"name": "Last change password time", "visible": True, "viewRule": "Self", "modifyRule": "Self"},
    {"name": "Managed accounts", "visible": True, "viewRule": "Admin", "modifyRule": "Admin"},
    {"name": "Face ID", "visible": True, "viewRule": "Self", "modifyRule": "Self"},
    {"name": "MFA accounts", "visible": True, "viewRule": "Self", "modifyRule": "Self"},
    {"name": "MFA items", "visible": True, "viewRule": "Admin", "modifyRule": "Admin"},
]

COMMON_ROLES = [
    ("role_platform_admin", "平台管理员", "平台全局管理员，允许所有对象和动作"),
    ("role_platform_viewer", "平台只读", "平台全局只读角色"),
    ("role_audit_admin", "审计管理员", "可查询所有平台审计记录"),
    ("role_audit_viewer", "审计只读", "可只读查询平台审计记录"),
    ("role_security_admin", "安全管理员", "可管理认证、授权、审计相关配置"),
    ("role_skillhub_admin", "SkillHub 管理员", "SkillHub 全部管理权限"),
    ("role_skillhub_editor", "SkillHub 编辑者", "SkillHub 技能、分组、发布、提案等写入权限"),
    ("role_skillhub_viewer", "SkillHub 只读", "SkillHub 只读权限"),
    ("role_agentruntime_admin", "AgentRuntime 管理员", "AgentRuntime 全部管理权限"),
    ("role_agentruntime_operator", "AgentRuntime 操作者", "AgentRuntime 运行、停止、查看权限"),
    ("role_agentruntime_viewer", "AgentRuntime 只读", "AgentRuntime 只读权限"),
    ("role_sqlhub_admin", "SQLHub 管理员", "SQLHub 全部管理权限"),
    ("role_sqlhub_editor", "SQLHub 编辑者", "SQLHub 查询、数据源、模板等写入权限"),
    ("role_sqlhub_viewer", "SQLHub 只读", "SQLHub 只读权限"),
    ("role_modelgateway_admin", "ModelGateway 管理员", "模型网关全部管理权限"),
    ("role_modelgateway_operator", "ModelGateway 操作者", "模型网关路由、发布、限流等操作权限"),
    ("role_modelgateway_viewer", "ModelGateway 只读", "模型网关只读权限"),
    ("role_portal_admin", "Portal 管理员", "门户全部管理权限"),
    ("role_portal_editor", "Portal 编辑者", "门户页面、菜单、公告等写入权限"),
    ("role_portal_viewer", "Portal 只读", "门户只读权限"),
]

COMMON_PERMISSIONS = [
    ("perm_platform_admin", "平台管理员策略", ["role_platform_admin"], ["*"], ["*"]),
    ("perm_platform_viewer", "平台只读策略", ["role_platform_viewer"], ["portal:*", "skillhub:*", "agentruntime:*", "sqlhub:*", "modelgateway:*", "audit:*"], ["read", "view", "list", "admin:read"]),
    ("perm_audit_admin", "审计管理策略", ["role_audit_admin", "role_security_admin"], ["audit:*", "*:audit:*"], ["*"]),
    ("perm_audit_viewer", "审计只读策略", ["role_audit_viewer"], ["audit:*", "*:audit:*"], ["read", "view", "list"]),
    ("perm_security_admin", "安全管理策略", ["role_security_admin"], ["auth:*", "authn:*", "authz:*", "audit:*", "casdoor:*"], ["*"]),

    ("perm_skillhub_admin", "SkillHub 管理策略", ["role_skillhub_admin"], ["skillhub:*"], ["*"]),
    ("perm_skillhub_editor", "SkillHub 编辑策略", ["role_skillhub_editor"], [
        "skillhub:skill:*",
        "skillhub:group:*",
        "skillhub:proposal:*",
        "skillhub:release:*",
        "skillhub:knowledge:*",
        "skillhub:workflow:*",
        "skillhub:audit:*",
    ], ["read", "view", "list", "write", "create", "update", "delete", "publish", "rollback", "approve", "reject", "admin:read", "admin:write"]),
    ("perm_skillhub_viewer", "SkillHub 只读策略", ["role_skillhub_viewer"], ["skillhub:*"], ["read", "view", "list", "admin:read"]),

    ("perm_agentruntime_admin", "AgentRuntime 管理策略", ["role_agentruntime_admin"], ["agentruntime:*"], ["*"]),
    ("perm_agentruntime_operator", "AgentRuntime 操作策略", ["role_agentruntime_operator"], ["agentruntime:run:*", "agentruntime:session:*", "agentruntime:agent:*", "agentruntime:tool:*", "agentruntime:audit:*"], ["read", "view", "list", "run", "stop", "restart", "approve", "reject", "admin:read"]),
    ("perm_agentruntime_viewer", "AgentRuntime 只读策略", ["role_agentruntime_viewer"], ["agentruntime:*"], ["read", "view", "list", "admin:read"]),

    ("perm_sqlhub_admin", "SQLHub 管理策略", ["role_sqlhub_admin"], ["sqlhub:*"], ["*"]),
    ("perm_sqlhub_editor", "SQLHub 编辑策略", ["role_sqlhub_editor"], ["sqlhub:datasource:*", "sqlhub:query:*", "sqlhub:template:*", "sqlhub:report:*", "sqlhub:audit:*"], ["read", "view", "list", "create", "update", "execute", "export", "admin:read", "admin:write"]),
    ("perm_sqlhub_viewer", "SQLHub 只读策略", ["role_sqlhub_viewer"], ["sqlhub:*"], ["read", "view", "list", "admin:read"]),

    ("perm_modelgateway_admin", "ModelGateway 管理策略", ["role_modelgateway_admin"], ["modelgateway:*"], ["*"]),
    ("perm_modelgateway_operator", "ModelGateway 操作策略", ["role_modelgateway_operator"], ["modelgateway:model:*", "modelgateway:route:*", "modelgateway:key:*", "modelgateway:quota:*", "modelgateway:audit:*"], ["read", "view", "list", "create", "update", "publish", "disable", "enable", "admin:read"]),
    ("perm_modelgateway_viewer", "ModelGateway 只读策略", ["role_modelgateway_viewer"], ["modelgateway:*"], ["read", "view", "list", "admin:read"]),

    ("perm_portal_admin", "Portal 管理策略", ["role_portal_admin"], ["portal:*"], ["*"]),
    ("perm_portal_editor", "Portal 编辑策略", ["role_portal_editor"], ["portal:page:*", "portal:menu:*", "portal:notice:*", "portal:asset:*"], ["read", "view", "list", "create", "update", "delete", "publish", "admin:read"]),
    ("perm_portal_viewer", "Portal 只读策略", ["role_portal_viewer"], ["portal:*"], ["read", "view", "list", "admin:read"]),
]

LOGIN_PERMISSION_NAME = "perm_aisphere_auth_login"


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
    parser.add_argument("--admin-user", default="admin", help="Bootstrap admin username to create/bind to role_platform_admin")
    parser.add_argument("--admin-display-name", default="Admin", help="Bootstrap admin display name")
    parser.add_argument("--admin-email", default="admin@example.com", help="Bootstrap admin email")
    parser.add_argument("--admin-password", default="", help="Bootstrap admin password. Requires python bcrypt package to hash.")
    parser.add_argument("--admin-password-hash", default="", help="Precomputed bcrypt hash for bootstrap admin password")
    parser.add_argument("--skip-admin-user-create", action="store_true", help="Do not create/update bootstrap admin user row")
    parser.add_argument("--skip-admin-binding", action="store_true", help="Do not bind admin user to role_platform_admin")
    parser.add_argument("--admin-user-only", action="store_true", help="Render only bootstrap admin user and role binding; do not touch application/client secret")
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


def permission_id(org: str, perm_name: str) -> str:
    return f"{org}/{perm_name}"


def model_id(org: str, model_name: str) -> str:
    return f"{org}/{model_name}"


def render_admin_user_insert(args: argparse.Namespace, created_time: str, password_hash: str, password_type: str) -> str:
    return insert_on_duplicate(
        "user",
        ["owner", "name", "created_time", "updated_time", "id", "type", "password", "password_salt", "password_type", "display_name", "avatar", "email", "email_verified", "phone", "country_code", "affiliation", "tag", "language", "score", "karma", "ranking", "currency", "is_default_avatar", "is_online", "is_admin", "is_forbidden", "is_deleted", "signup_application", "register_type", "register_source", "created_ip"],
        [sql_quote(args.org), sql_quote(args.admin_user), sql_quote(created_time), sql_quote(created_time), sql_quote(str(uuid.uuid4())), sql_quote("normal-user"), sql_quote(password_hash), sql_quote(""), sql_quote(password_type), sql_quote(args.admin_display_name), sql_quote("https://cdn.casbin.org/img/casbin.svg"), sql_quote(args.admin_email), "0", sql_quote(""), sql_quote("CN"), sql_quote("AI Sphere"), sql_quote("staff"), sql_quote("zh"), "0", "0", "0", sql_quote("USD"), "1", "0", "1", "0", "0", sql_quote(args.app), sql_quote("Add User"), sql_quote("bootstrap"), sql_quote("127.0.0.1")],
    )


def render_platform_admin_role_insert(args: argparse.Namespace, created_time: str, admin_subject: str) -> str:
    name, display_name, desc = COMMON_ROLES[0]
    return insert_on_duplicate(
        "role",
        ["owner", "name", "created_time", "display_name", "description", "users", "groups", "roles", "domains", "is_enabled"],
        [sql_quote(args.org), sql_quote(name), sql_quote(created_time), sql_quote(display_name), sql_quote(desc), sql_json([admin_subject]), sql_json([]), sql_json([]), sql_json([]), "1"],
    )


def iter_permission_specs(args: argparse.Namespace):
    yield (
        LOGIN_PERMISSION_NAME,
        "AI Sphere Auth 登录策略",
        [f"{args.org}/*"],
        [],
        [args.app],
        ["Read"],
    )
    for name, display_name, roles, resources, actions in COMMON_PERMISSIONS:
        yield name, display_name, [], [role_id(args.org, r) for r in roles], resources, actions


def build_password_hash(args: argparse.Namespace) -> tuple[str, str, str | None]:
    """Return (password_hash, password_type, generated_password_or_none)."""
    if args.admin_password_hash.strip():
        return args.admin_password_hash.strip(), "bcrypt", None
    if args.admin_password.strip():
        try:
            import bcrypt  # type: ignore[import-not-found]
        except ImportError:
            raise SystemExit("[ERROR] --admin-password requires bcrypt. Install with: python -m pip install bcrypt, or pass --admin-password-hash.")
        hashed = bcrypt.hashpw(args.admin_password.encode("utf-8"), bcrypt.gensalt(rounds=10)).decode("utf-8")
        return hashed, "bcrypt", None
    return "", "", None


def render(args: argparse.Namespace) -> tuple[str, str, list[str]]:
    for name in ["org", "app_owner", "app", "client_id", "model", "permission_id", "admin_user"]:
        validate_identifier(name, getattr(args, name))

    client_secret = args.client_secret.strip() or "aisphere_" + secrets.token_urlsafe(32)
    created_time = args.created_time.strip() or datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    redirect_uris = args.redirect_uri or ["http://127.0.0.1:18080/auth/callback/casdoor"]
    admin_subject = f"{args.org}/{args.admin_user}"
    password_hash, password_type, generated_admin_password = build_password_hash(args)
    admin_user_only = bool(getattr(args, "admin_user_only", False))

    if admin_user_only and args.skip_admin_user_create:
        raise SystemExit("[ERROR] --admin-user-only cannot be combined with --skip-admin-user-create.")
    if admin_user_only and not (args.admin_password.strip() or args.admin_password_hash.strip()):
        raise SystemExit("[ERROR] --admin-user-only requires --admin-password or --admin-password-hash so the user can sign in.")

    if admin_user_only:
        lines = [
            "-- AI Sphere Auth Casdoor bootstrap admin user SQL",
            "-- Generated by scripts/casdoor/render-casdoor-seed.py --admin-user-only",
            "-- Scope: bootstrap admin user and role-user binding only.",
            "-- This mode does not update application, client_secret, redirect URIs, model, permissions or policies.",
            "SET FOREIGN_KEY_CHECKS=0;",
            "",
            "-- 1. Bootstrap admin user",
            render_admin_user_insert(args, created_time, password_hash, password_type),
        ]
        if not args.skip_admin_binding:
            lines.extend(["", "-- 2. Platform admin role binding", render_platform_admin_role_insert(args, created_time, admin_subject)])
        lines.extend(["", "SET FOREIGN_KEY_CHECKS=1;"])
        config_lines = [
            f"AISPHERE_CASDOOR_OWNER={args.org}",
            f"AISPHERE_CASDOOR_APPLICATION={args.app}",
            f"AISPHERE_CASDOOR_ADMIN_USER={args.admin_user}",
        ]
        if args.admin_password.strip():
            config_lines.append(f"AISPHERE_CASDOOR_ADMIN_PASSWORD={args.admin_password}")
        return "\n".join(lines) + "\n", client_secret, config_lines

    lines: list[str] = [
        "-- AI Sphere Auth Casdoor bootstrap SQL",
        "-- Generated by scripts/casdoor/render-casdoor-seed.py",
        "-- Scope: organization, application, optional admin user, model, roles, permissions, permission rules and role-user binding.",
        "-- This file is idempotent and does not drop or create tables.",
        "-- Environment-specific values are injected by generator parameters at deployment time.",
        "SET FOREIGN_KEY_CHECKS=0;",
        "",
        "-- 1. Organization",
    ]

    lines.append(insert_on_duplicate(
        "organization",
        ["owner", "name", "created_time", "display_name", "website_url", "password_type", "country_codes", "default_application", "user_types", "tags", "languages", "default_avatar", "use_email_as_username", "is_profile_public", "nav_items", "user_nav_items", "widget_items", "account_menu", "account_items"],
        [sql_quote("admin"), sql_quote(args.org), sql_quote(created_time), sql_quote(args.org_display_name), sql_quote(""), sql_quote("bcrypt"), sql_json(["CN"]), sql_quote(args.app), sql_json([]), sql_json([]), sql_json(["zh", "en"]), sql_quote("https://cdn.casbin.org/img/casbin.svg"), "0", "1", sql_json(["all"]), sql_json([]), sql_json(["all"]), sql_quote("Horizontal"), sql_json(DEFAULT_ACCOUNT_ITEMS)],
    ))

    lines.extend(["", "-- 2. OAuth application"])
    lines.append(insert_on_duplicate(
        "application",
        ["owner", "name", "created_time", "display_name", "category", "type", "scopes", "logo", "title", "organization", "cert", "enable_password", "enable_sign_up", "enable_signin_session", "grant_types", "signin_methods", "signup_items", "signin_items", "tags", "client_id", "client_secret", "redirect_uris", "token_format", "token_signing_method", "expire_in_hours", "refresh_expire_in_hours", "cookie_expire_in_hours", "is_shared"],
        [sql_quote(args.app_owner), sql_quote(args.app), sql_quote(created_time), sql_quote(args.app_display_name), sql_quote("Default"), sql_quote("All"), sql_json(SCOPE_ITEMS), sql_quote("https://cdn.casbin.org/img/casdoor-logo_1185x256.png"), sql_quote(args.app_display_name), sql_quote(args.org), sql_quote(args.cert), "1", "1", "1", sql_json(["authorization_code", "refresh_token", "password", "client_credentials"]), sql_json([{"name": "Password", "displayName": "Password", "rule": "All"}, {"name": "Verification code", "displayName": "Verification code", "rule": "All"}]), sql_json([{"name": "Username", "visible": True, "required": True, "prompted": False, "type": "", "customCss": "", "label": "", "placeholder": "", "options": [], "regex": "", "rule": "None"}, {"name": "Password", "visible": True, "required": True, "prompted": False, "type": "", "customCss": "", "label": "", "placeholder": "", "options": [], "regex": "", "rule": "None"}, {"name": "Email", "visible": True, "required": False, "prompted": False, "type": "", "customCss": "", "label": "", "placeholder": "", "options": [], "regex": "", "rule": "Normal"}]), sql_json([{"name": "Username", "visible": True, "label": "", "customCss": "", "placeholder": "", "rule": "None", "isCustom": False}, {"name": "Password", "visible": True, "label": "", "customCss": "", "placeholder": "", "rule": "None", "isCustom": False}, {"name": "Login button", "visible": True, "label": "", "customCss": "", "placeholder": "", "rule": "None", "isCustom": False}]), sql_json([]), sql_quote(args.client_id), sql_quote(client_secret), sql_json(redirect_uris), sql_quote("JWT"), sql_quote("RS256"), "24", "168", "24", "0"],
    ))

    if not args.skip_admin_user_create:
        lines.extend(["", "-- 3. Bootstrap admin user"])
        lines.append(render_admin_user_insert(args, created_time, password_hash, password_type))

    lines.extend(["", "-- 4. Casbin model"])
    lines.append(insert_on_duplicate(
        "model",
        ["owner", "name", "created_time", "display_name", "description", "model_text"],
        [sql_quote(args.org), sql_quote(args.model), sql_quote(created_time), sql_quote("AI Sphere RBAC Model"), sql_quote("AI Sphere unified RBAC model for object/action checks"), sql_quote(MODEL_TEXT)],
    ))

    lines.extend(["", "-- 5. Roles and role-user bindings"])
    for name, display_name, desc in COMMON_ROLES:
        users = [admin_subject] if (name == "role_platform_admin" and not args.skip_admin_binding) else []
        lines.append(insert_on_duplicate(
            "role",
            ["owner", "name", "created_time", "display_name", "description", "users", "groups", "roles", "domains", "is_enabled"],
            [sql_quote(args.org), sql_quote(name), sql_quote(created_time), sql_quote(display_name), sql_quote(desc), sql_json(users), sql_json([]), sql_json([]), sql_json([]), "1"],
        ))

    lines.extend(["", "-- 6. Permissions / policies"])
    permission_specs = list(iter_permission_specs(args))
    generated_perm_ids = [permission_id(args.org, item[0]) for item in permission_specs]
    for name, display_name, users, roles, resources, actions in permission_specs:
        lines.append(insert_on_duplicate(
            "permission",
            ["owner", "name", "created_time", "display_name", "description", "users", "groups", "roles", "domains", "model", "adapter", "resource_type", "resources", "actions", "effect", "is_enabled", "submitter", "approver", "approve_time", "state"],
            [sql_quote(args.org), sql_quote(name), sql_quote(created_time), sql_quote(display_name), sql_quote(display_name), sql_json(users), sql_json([]), sql_json(roles), sql_json([]), sql_quote(model_id(args.org, args.model)), sql_quote(""), sql_quote("Application"), sql_json(resources), sql_json(actions), sql_quote("Allow"), "1", sql_quote(args.admin_user), sql_quote(args.admin_user), sql_quote(created_time), sql_quote("Approved")],
        ))

    lines.extend(["", "-- 7. Permission rules / Casbin policies"])
    lines.append("DELETE FROM `permission_rule` WHERE `v5` IN (" + ", ".join(sql_quote(x) for x in generated_perm_ids) + ");")
    for name, _display_name, users, roles, resources, actions in permission_specs:
        perm_id = permission_id(args.org, name)
        for subject in [*users, *roles]:
            for resource in resources:
                for action in actions:
                    lines.append(
                        "INSERT INTO `permission_rule` (`ptype`, `v0`, `v1`, `v2`, `v3`, `v4`, `v5`) VALUES "
                        f"('p', {sql_quote(subject)}, {sql_quote(resource)}, {sql_quote(action)}, 'allow', '', {sql_quote(perm_id)});"
                    )

    config_lines = [
        f"AISPHERE_CASDOOR_OWNER={args.org}",
        f"AISPHERE_CASDOOR_APPLICATION={args.app}",
        f"AISPHERE_CASDOOR_CLIENT_ID={args.client_id}",
        f"AISPHERE_CASDOOR_CLIENT_SECRET={client_secret}",
        f"AISPHERE_CASDOOR_PERMISSION_ID={args.org}/{args.permission_id}",
        f"AISPHERE_CASDOOR_ADMIN_USER={args.admin_user}",
    ]
    if args.admin_password.strip():
        config_lines.append(f"AISPHERE_CASDOOR_ADMIN_PASSWORD={args.admin_password}")
    elif generated_admin_password:
        config_lines.append(f"AISPHERE_CASDOOR_ADMIN_PASSWORD={generated_admin_password}")

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
        "-- Admin password is intentionally not printed in SQL comments.",
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
    print(f"[INFO] roles={len(COMMON_ROLES)} permissions={len(COMMON_PERMISSIONS) + 1}")
    if args.env_output.strip():
        print(f"[OK] wrote aisphere-auth env values: {args.env_output}")
    elif not args.client_secret.strip():
        print("[WARN] generated a random client_secret. Re-run with --env-output <file> or pass --client-secret explicitly so aisphere-auth can use the same secret.")
    else:
        print("[INFO] client_secret was injected by parameter and is not repeated in SQL comments.")
    if not args.skip_admin_user_create:
        if not args.admin_password and not args.admin_password_hash:
            print("[WARN] created bootstrap user without password hash. Set password in Casdoor UI or re-run with --admin-password / --admin-password-hash.")
        else:
            print(f"[INFO] created/updated bootstrap user {args.org}/{args.admin_user}")
    if not args.skip_admin_binding:
        print(f"[INFO] bound user {args.org}/{args.admin_user} to role_platform_admin")
    _ = client_secret
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
