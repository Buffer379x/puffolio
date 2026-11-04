"""The version gate names each forgotten pin instead of letting a release drift."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('check_version', Path(__file__).with_name('check-version.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def write_release(root, version='1.2.3'):
    files = {
        'VERSION': version + '\n',
        'deploy/compose.yaml': f'services:\n  puffolio:\n    image: ghcr.io/buffer379x/puffolio:{version}\n',
        'deploy/unraid/puffolio.xml': (f'<Repository>ghcr.io/buffer379x/puffolio:{version}</Repository>'
                                       f'<Icon>https://raw.githubusercontent.com/Buffer379x/puffolio/v{version}/i.png</Icon>'
                                       f'<TemplateURL>https://raw.githubusercontent.com/Buffer379x/puffolio/v{version}/t.xml</TemplateURL>'),
        f'docs/release-{version}.md': f'# Puffolio {version}\n',
        'CHANGELOG.md': f'## {version} – heute\n',
        'web/package.json': json.dumps({'version': version}),
        'docs/api/openapi.yaml': json.dumps({'info': {'version': version}}),
        'README.md': f'ghcr.io/buffer379x/puffolio:{version} /download/v{version}/compose.yaml',
    }
    for name, content in files.items():
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)


class VersionConsistency(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name)
        write_release(self.root)

    def test_consistent_release_passes(self):
        self.assertEqual(module.problems(self.root), [])
        self.assertEqual(module.problems(self.root, '1.2.3'), [])

    def test_requested_version_must_match(self):
        self.assertTrue(module.problems(self.root, '1.2.4'))

    def test_invalid_version_is_refused(self):
        (self.root / 'VERSION').write_text('latest\n')
        self.assertTrue(module.problems(self.root))

    def test_each_forgotten_pin_is_named(self):
        for name, old, new in [
            ('deploy/compose.yaml', 'puffolio:1.2.3', 'puffolio:1.2.2'),
            ('deploy/unraid/puffolio.xml', 'puffolio:1.2.3<', 'puffolio:1.2.2<'),
            ('deploy/unraid/puffolio.xml', '/v1.2.3/i.png', '/v1.2.2/i.png'),
            ('deploy/unraid/puffolio.xml', '/v1.2.3/t.xml', '/v1.2.2/t.xml'),
            ('CHANGELOG.md', '## 1.2.3', '## 1.2.2'),
            ('web/package.json', '1.2.3', '1.2.2'),
            ('docs/api/openapi.yaml', '1.2.3', '1.2.2'),
            ('README.md', '/download/v1.2.3/', '/download/v1.2.2/'),
            ('README.md', 'puffolio:1.2.3', 'puffolio:1.2.2'),
        ]:
            with self.subTest(file=name, old=old):
                write_release(self.root)
                path = self.root / name
                path.write_text(path.read_text().replace(old, new))
                found = module.problems(self.root)
                self.assertEqual(len(found), 1, found)
                self.assertIn(name, found[0])

    def test_missing_release_notes_are_named(self):
        (self.root / 'docs/release-1.2.3.md').unlink()
        self.assertIn('docs/release-1.2.3.md: missing', module.problems(self.root))

    def test_repository_itself_is_consistent(self):
        self.assertEqual(module.problems(), [])


if __name__ == '__main__':
    unittest.main()
