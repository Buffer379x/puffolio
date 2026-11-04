#!/usr/bin/env python3
"""Prepare/check disposable synthetic startup databases; never opens user data."""
import argparse
import hashlib
import json
from pathlib import Path
import sqlite3


def snapshot(directory):
    path = directory / 'puffolio.db'
    return hashlib.sha256(path.read_bytes()).hexdigest()


def prepare(root):
    migration = Path('internal/store/migrations/001_initial.sql').read_bytes()
    for kind in ('initial', 'current', 'newer', 'denied'):
        directory = root / kind
        directory.mkdir(mode=0o700)
        with sqlite3.connect(directory / 'puffolio.db') as db:
            if kind == 'initial':
                db.execute('PRAGMA user_version=0')
            else:
                db.executescript(migration.decode())
                db.execute('INSERT INTO schema_migrations VALUES(1,?,?)', (hashlib.sha256(migration).hexdigest(), '2026-01-01T00:00:00Z'))
                if kind == 'newer':
                    db.execute('PRAGMA user_version=999')
                db.commit()
                db.execute('PRAGMA journal_mode=WAL')
        (directory / 'puffolio.db').chmod(0o600)
    (root / 'unchanged.json').write_text(json.dumps({kind: snapshot(root / kind) for kind in ('newer', 'denied')}))


def check(root):
    expected = json.loads((root / 'unchanged.json').read_text())
    for kind, digest in expected.items():
        assert snapshot(root / kind) == digest, kind + ' startup changed database bytes'
    for kind in ('initial', 'current'):
        with sqlite3.connect('file:' + str(root / kind / 'puffolio.db') + '?mode=ro&immutable=1', uri=True) as db:
            assert db.execute('PRAGMA user_version').fetchone()[0] == 1
            assert db.execute('PRAGMA integrity_check').fetchone()[0] == 'ok'
            assert db.execute('SELECT count(*) FROM schema_migrations WHERE version=1').fetchone()[0] == 1
    print('PASS synthetic schema0 initialization, schema1 reopen, denied permissions and newer-schema preservation')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=('prepare', 'check'))
    parser.add_argument('directory', type=Path)
    args = parser.parse_args()
    if args.mode == 'prepare':
        prepare(args.directory)
    else:
        check(args.directory)


if __name__ == '__main__':
    main()
