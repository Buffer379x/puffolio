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


def migrations():
    return [(version, Path('internal/store/migrations', name).read_bytes())
            for version, name in ((1, '001_initial.sql'), (2, '002_palette.sql'))]


def seed_tag(db):
    """A tag in use, as the rebuild of the tags table has to keep it."""
    db.executescript("""
        INSERT INTO users(id,username,display_name,password_hash,role,created_at) VALUES('u','fixture','Fixture','x','admin','2026-01-01');
        INSERT INTO collections(id,user_id) VALUES('c','u');
        INSERT INTO field_definitions(id,collection_id,type,data) VALUES('f','c','tags','{}');
        INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id,version) VALUES('t','c','f','Tag','tag','pink',3);
        INSERT INTO liquids(id,collection_id,name,normalized_name,created_at,updated_at) VALUES('l','c','Liquid','liquid','2026-01-01','2026-01-01');
        INSERT INTO liquid_tags(collection_id,liquid_id,field_id,tag_id) VALUES('c','l','f','t');
    """)


def prepare(root):
    for kind in ('initial', 'previous', 'current', 'newer', 'denied'):
        directory = root / kind
        directory.mkdir(mode=0o700)
        with sqlite3.connect(directory / 'puffolio.db') as db:
            if kind == 'initial':
                db.execute('PRAGMA user_version=0')
            else:
                for version, script in migrations()[:1 if kind == 'previous' else None]:
                    db.executescript(script.decode())
                    db.execute('INSERT INTO schema_migrations VALUES(?,?,?)', (version, hashlib.sha256(script).hexdigest(), '2026-01-01T00:00:00Z'))
                if kind == 'previous':
                    seed_tag(db)
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
    recorded = [(version, hashlib.sha256(script).hexdigest()) for version, script in migrations()]
    for kind in ('initial', 'previous', 'current'):
        with sqlite3.connect('file:' + str(root / kind / 'puffolio.db') + '?mode=ro&immutable=1', uri=True) as db:
            assert db.execute('PRAGMA user_version').fetchone()[0] == 2, kind
            assert db.execute('PRAGMA integrity_check').fetchone()[0] == 'ok', kind
            assert db.execute('SELECT version,checksum FROM schema_migrations ORDER BY version').fetchall() == recorded, kind
            if kind == 'previous':
                assert db.execute('SELECT id,palette_id,version FROM tags').fetchall() == [('t', 'pink', 3)]
                assert db.execute('SELECT liquid_id,tag_id FROM liquid_tags').fetchall() == [('l', 't')]
    print('PASS synthetic schema0 initialization, schema1 migration, schema2 reopen, denied permissions and newer-schema preservation')


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
