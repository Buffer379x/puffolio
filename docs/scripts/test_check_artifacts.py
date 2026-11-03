"""Literal publication-boundary regression cases; no Git metadata needed."""
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('check_artifacts', Path(__file__).with_name('check-artifacts.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)

class PublicationPaths(unittest.TestCase):
    def test_runtime_cache_database_and_secret_paths_are_rejected(self):
        forbidden = [
            'data/media/private.jpg', 'backups/instance.zip', 'work/qa/state.json',
            'nested/data/private.jpg', 'nested/backups/instance.zip', 'nested/work/state.json',
            'web/node_modules/package/index.js', '.pnpm-store/pkg', 'web/.cache/state',
            'work/cache/python/a.pyc', '__pycache__/a.pyc', '.pytest_cache/state',
            'coverage/index.html', 'web/playwright-report/index.html', 'test-results/trace.zip',
            'puffolio.db', 'nested/settings.sqlite', 'synthetic.sqlite3',
            'puffolio.db-wal', 'puffolio.db-shm', 'nested/settings.sqlite-wal',
            'nested/settings.sqlite-shm', 'settings.sqlite3-wal', 'settings.sqlite3-shm',
            '.env', '.env.production', 'deploy/.env.local', 'web/.DS_Store', 'a.pyc',
        ]
        for name in forbidden:
            with self.subTest(name=name), self.assertRaises(ValueError):
                guard.validate_paths([name])

    def test_product_sources_examples_and_synthetic_fixtures_are_allowed(self):
        guard.validate_paths([
            'docs/backup-restore.md', '.env.example', 'deploy/.env.example',
            'internal/backup/testdata/synthetic.png', 'tests/fixtures/example.json',
            'docs/scripts/test_check_artifacts.py', 'web/src/features/tools/calculateNicotine.test.ts',
            'assets/brand/icon-256.png', 'data-model.md',
        ])

if __name__ == '__main__':
    unittest.main()
