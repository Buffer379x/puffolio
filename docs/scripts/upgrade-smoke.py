#!/usr/bin/env python3
"""Real upgrade check: data written by the previous release image, read by this one."""
import argparse
import importlib.util
import json
from pathlib import Path
import secrets
import struct
import urllib.parse
import zlib

from smoke import Client

API = '/api/v1'
spec = importlib.util.spec_from_file_location('image_memory_smoke', Path(__file__).with_name('image-memory-smoke.py'))
image_helpers = importlib.util.module_from_spec(spec)
spec.loader.exec_module(image_helpers)


def landscape_png(width=800, height=600):
    def chunk(kind, data):
        return struct.pack('>I', len(data)) + kind + data + struct.pack('>I', zlib.crc32(kind + data) & 0xffffffff)
    raw = (b'\0' + b'\x80\xc0\x40' * width) * height
    return (b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', width, height, 8, 2, 0, 0, 0))
            + chunk(b'IDAT', zlib.compress(raw)) + chunk(b'IEND', b''))


def seed(url, code, state):
    """Only uses API routes of the previous release; 4:3 crop and invitation exist there."""
    admin = Client(url)
    password = secrets.token_urlsafe(24)
    me = admin.request(API + '/setup', 'POST', {'code': code, 'username': 'upgrade-admin', 'displayName': 'Upgrade',
                                                'password': password, 'starterFields': True})
    admin.csrf = me['csrfToken']
    fields = {f['label']: f for f in admin.request(API + '/fields')['items']}
    upload = admin.upload(API + '/media', 'synthetic.png', landscape_png(), 'image/png')
    media = admin.request(f'{API}/media/{upload["id"]}/crop', 'POST', {'x': 0, 'y': 0, 'width': 1, 'height': 1})
    values = {fields['Bild']['id']: {'type': 'image', 'value': media['id']},
              fields['Menge']['id']: {'type': 'number', 'value': '60.0'},
              fields['Kaufdatum']['id']: {'type': 'date', 'value': '2025-01-01'}}
    liquid = admin.request(API + '/liquids', 'POST', {'name': 'Aus der Vorversion', 'favorite': True, 'values': values})
    invited = admin.request(API + '/admin/users', 'POST', {'username': 'upgrade-other', 'displayName': 'Other', 'role': 'user'})
    other_password = secrets.token_urlsafe(24)
    Client(url).request(API + '/auth/invite', 'POST', {'token': invited['invitationToken'], 'password': other_password})
    state.write_text(json.dumps({'password': password, 'otherPassword': other_password, 'liquid': liquid,
                                 'mediaId': media['id'], 'dashboard': admin.request(API + '/dashboard'),
                                 'purchaseField': fields['Kaufdatum']['id'], 'fieldCount': len(fields)}))
    state.chmod(0o600)
    print(json.dumps({'status': 'PASS', 'mode': 'seeded-previous-release'}))


def verify(url, state):
    saved = json.loads(state.read_text())
    media = saved['mediaId']
    visitor = Client(url)
    page = visitor.request(API + '/liquids')
    assert page['total'] == 1 and page['items'][0]['id'] == saved['liquid']['id'], 'Visitors read the existing collection'
    assert page['items'][0]['values'] == saved['liquid']['values'], 'Values survive the upgrade'
    assert 'shelfLife' not in page['items'][0], 'No shelf life until it is set up'
    legacy = visitor.request(f'{API}/media/{media}?variant=thumbnail')
    assert image_helpers.jpeg_dimensions(legacy) == (480, 360), 'Stored 4:3 images remain servable'
    visitor.request(f'{API}/media/{media}?variant=master', status=401)
    visitor.request(API + '/auth/login', 'POST', {'username': 'upgrade-other', 'password': saved['otherPassword']}, status=401)

    admin = Client(url)
    admin.csrf = admin.request(API + '/auth/login', 'POST', {'username': 'upgrade-admin', 'password': saved['password']})['csrfToken']
    assert len(admin.request(API + '/fields')['items']) == saved['fieldCount'], 'Fields survive the upgrade'
    assert admin.request(API + '/dashboard')['widgets'] == saved['dashboard']['widgets'], 'Dashboard survives the upgrade'

    preferences = admin.request(API + '/preferences')
    assert preferences['shelfLifeMonths'] == 0
    preferences['shelfLifeMonths'] = 600
    preferences['roles']['purchase'] = saved['purchaseField']
    admin.request(API + '/preferences', 'PUT', preferences)
    life = admin.request(API + '/liquids/' + saved['liquid']['id'])['shelfLife']
    assert life['expiresOn'] == '2075-01-01' and not life['expired'], 'Shelf life can be set up afterwards'

    admin.request(f'{API}/media/{media}/crop', 'POST', {'x': 0, 'y': 0, 'width': 1, 'height': 1}, status=422)
    square = admin.request(f'{API}/media/{media}/crop', 'POST', {'x': 0.125, 'y': 0, 'width': 0.75, 'height': 1})
    width, height = image_helpers.jpeg_dimensions(admin.request(urllib.parse.urlsplit(square['url']).path))
    assert width == height == 600, 'A new crop of an old image is square'

    archive = admin.request(API + '/exports', 'POST', {})
    assert admin.upload(API + '/imports/preview', 'puffolio.zip', archive, 'application/zip')['counts']['liquids'] == 1
    print(json.dumps({'status': 'PASS', 'mode': 'upgraded', 'checks': [
        'visitor-read', 'values-kept', 'legacy-thumbnail', 'second-account-locked', 'admin-login', 'fields-dashboard-kept',
        'shelf-life-configurable', 'square-recrop', 'export-importable']}))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['seed', 'verify'])
    parser.add_argument('--url', required=True)
    parser.add_argument('--state-file', required=True, type=Path)
    parser.add_argument('--setup-code-file', type=Path)
    args = parser.parse_args()
    if urllib.parse.urlsplit(args.url).hostname not in ('127.0.0.1', 'localhost', '::1'):
        parser.error('Upgrade smoke tests may only target a loopback test instance.')
    if args.mode == 'seed':
        seed(args.url, args.setup_code_file.read_text().strip(), args.state_file)
    else:
        verify(args.url, args.state_file)


if __name__ == '__main__':
    main()
