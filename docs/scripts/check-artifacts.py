#!/usr/bin/env python3
"""Audit the tracked product boundary; never treat gitignore as publication evidence."""
from pathlib import Path, PurePosixPath
import gzip
import subprocess

FORBIDDEN_PARTS = {
    'data', 'backups', 'work', 'node_modules', '__pycache__', '.pnpm-store',
    '.cache', '.pytest_cache', '.mypy_cache', '.ruff_cache', '.npm',
    'coverage', 'playwright-report', 'test-results',
}
DATABASE_SUFFIXES = tuple(
    extension + sidecar
    for extension in ('.db', '.sqlite', '.sqlite3')
    for sidecar in ('', '-wal', '-shm')
)


def validate_paths(names):
    """Reject publication of runtime state, dependencies, caches and secrets."""
    for name in names:
        path = PurePosixPath(name)
        if (path.is_absolute() or '..' in path.parts
                or any(part in FORBIDDEN_PARTS for part in path.parts)
                or path.name.lower().endswith(DATABASE_SUFFIXES + ('.pyc',))
                or path.name == '.DS_Store'
                or (path.name.startswith('.env') and path.name != '.env.example')):
            raise ValueError('Forbidden tracked artifact: ' + name)


def main():
    files = subprocess.check_output(['git', 'ls-files', '-z']).decode().split('\0')
    try:
        validate_paths(filter(None, files))
    except ValueError as error:
        raise SystemExit(str(error)) from error
    assert '/work/' in Path('.gitignore').read_text()
    assert 'work' in Path('.dockerignore').read_text().splitlines()
    assets = list(Path('web/dist/assets').glob('*.js')) + list(Path('web/dist/assets').glob('*.css'))
    if assets:
        total = sum(len(gzip.compress(path.read_bytes(), mtime=0)) for path in assets)
        print('Frontend JavaScript+CSS gzip bytes:', total)
        if total > 250 * 1024:
            raise SystemExit('Frontend transfer budget exceeds 250 KiB')
    print('Tracked product boundary checked')


if __name__ == '__main__':
    main()
