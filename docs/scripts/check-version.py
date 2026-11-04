#!/usr/bin/env python3
"""A release must name one version everywhere; a forgotten pin fails before any build."""
import json
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[2]
SEMVER = re.compile(r'[0-9]+\.[0-9]+\.[0-9]+\Z')
IMAGE = 'ghcr.io/buffer379x/puffolio'
RAW = 'https://raw.githubusercontent.com/Buffer379x/puffolio/v'
# Documents a reader follows when installing. History is allowed to name old versions.
PINNED_DOCUMENTS = ('README.md', 'SECURITY.md', 'docs/installation.md', 'docs/backup-restore.md',
                    'docs/releasing.md', 'web/public/help/backup-restore.md')
PIN = re.compile(r'puffolio:([0-9]+\.[0-9]+\.[0-9]+)|/(?:download|blob|raw)/v([0-9]+\.[0-9]+\.[0-9]+)/')


def problems(root=ROOT, expected=None):
    found = []
    version = (root / 'VERSION').read_text().strip()
    if not SEMVER.match(version):
        return [f'VERSION is not MAJOR.MINOR.PATCH: {version!r}']
    if expected is not None and expected != version:
        found.append(f'requested {expected}, but VERSION says {version}')

    def require(path, text):
        file = root / path
        if not file.is_file():
            found.append(f'{path}: missing')
        elif text not in file.read_text():
            found.append(f'{path}: expected {text!r}')

    require('deploy/compose.yaml', f'image: {IMAGE}:{version}\n')
    require('deploy/unraid/puffolio.xml', f'<Repository>{IMAGE}:{version}</Repository>')
    require('deploy/unraid/puffolio.xml', f'<Icon>{RAW}{version}/')
    require('deploy/unraid/puffolio.xml', f'<TemplateURL>{RAW}{version}/')
    require(f'docs/release-{version}.md', f'# Puffolio {version}\n')
    require('CHANGELOG.md', f'## {version} ')
    if json.loads((root / 'web/package.json').read_text()).get('version') != version:
        found.append('web/package.json: version differs')
    if json.loads((root / 'docs/api/openapi.yaml').read_text()).get('info', {}).get('version') != version:
        found.append('docs/api/openapi.yaml: info.version differs')
    for path in PINNED_DOCUMENTS:
        file = root / path
        if file.is_file():
            for match in PIN.finditer(file.read_text()):
                if (match.group(1) or match.group(2)) != version:
                    found.append(f'{path}: still pins {match.group(0)}')
    return found


def main(arguments):
    found = problems(expected=arguments[0] if arguments else None)
    for problem in found:
        print(problem, file=sys.stderr)
    if not found:
        print('Version consistent:', (ROOT / 'VERSION').read_text().strip())
    return 1 if found else 0


if __name__ == '__main__':
    raise SystemExit(main(sys.argv[1:]))
