import argparse
import importlib.util
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def load_script(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


render_seed = load_script("render_casdoor_seed", ROOT / "scripts" / "casdoor" / "render-casdoor-seed.py")
bootstrap = load_script("bootstrap_casdoor_mysql", ROOT / "scripts" / "casdoor" / "bootstrap-casdoor-mysql.py")


class CasdoorSeedTest(unittest.TestCase):
    def test_rendered_seed_contains_organization_ui_defaults(self):
        args = argparse.Namespace(
            org="aisphere",
            org_display_name="AI Sphere",
            app_owner="admin",
            app="aisphere-auth",
            app_display_name="AI Sphere Auth",
            client_id="aisphere-auth",
            client_secret="test-secret",
            redirect_uri=["http://127.0.0.1:18080/auth/callback/casdoor"],
            cert="cert-built-in",
            model="aisphere-auth-model",
            permission_id="perm_platform_admin",
            admin_user="admin",
            admin_display_name="Admin",
            admin_email="admin@example.com",
            admin_password="",
            admin_password_hash="",
            skip_admin_user_create=True,
            skip_admin_binding=True,
            created_time="2026-06-17T00:00:00Z",
        )

        sql, _, _ = render_seed.render(args)

        bootstrap.validate_seed_sql(sql)
        self.assertIn("`account_items`", sql)
        self.assertIn('"name":"Email"', sql)
        self.assertIn('"name":"Groups"', sql)
        self.assertIn("'Horizontal'", sql)

    def test_rendered_permissions_reference_model_by_owner_scoped_id(self):
        args = argparse.Namespace(
            org="aisphere",
            org_display_name="AI Sphere",
            app_owner="admin",
            app="aisphere-auth",
            app_display_name="AI Sphere Auth",
            client_id="aisphere-auth",
            client_secret="test-secret",
            redirect_uri=["http://127.0.0.1:18080/auth/callback/casdoor"],
            cert="cert-built-in",
            model="aisphere-auth-model",
            permission_id="perm_platform_admin",
            admin_user="admin",
            admin_display_name="Admin",
            admin_email="admin@example.com",
            admin_password="",
            admin_password_hash="",
            skip_admin_user_create=True,
            skip_admin_binding=True,
            created_time="2026-06-17T00:00:00Z",
        )

        sql, _, _ = render_seed.render(args)
        permission_lines = [line for line in sql.splitlines() if line.startswith("INSERT INTO `permission`")]

        self.assertTrue(permission_lines)
        self.assertTrue(all("'aisphere/aisphere-auth-model'" in line for line in permission_lines))
        self.assertFalse(any("'aisphere-auth-model'" in line and "'aisphere/aisphere-auth-model'" not in line for line in permission_lines))

    def test_seed_validation_rejects_missing_account_items(self):
        bad_sql = "INSERT INTO `organization` (`owner`, `name`) VALUES ('admin', 'aisphere');"

        with self.assertRaises(SystemExit):
            bootstrap.validate_seed_sql(bad_sql)


if __name__ == "__main__":
    unittest.main()
