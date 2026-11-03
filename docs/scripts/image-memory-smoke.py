#!/usr/bin/env python3
"""Synthetic bounded-memory regression for an owned, disposable loopback instance.

Run after smoke.py with its private --state-file, inside the normal 512MiB
container lifecycle. Requires only Python's standard library; logs no secrets.
"""
import argparse
import concurrent.futures
import copy
import hashlib
import json
import struct
import threading
import urllib.parse
import zlib
from pathlib import Path

from smoke import Client

API = '/api/v1'
PASSES = ((0, 0, 8, 8), (4, 0, 8, 8), (0, 4, 4, 8),
          (2, 0, 4, 4), (0, 2, 2, 4), (1, 0, 2, 2), (0, 1, 1, 2))


def validate_origin(origin):
    parsed = urllib.parse.urlsplit(origin)
    if (parsed.scheme not in ('http', 'https')
            or parsed.hostname not in ('127.0.0.1', 'localhost', '::1')
            or parsed.username or parsed.password or parsed.query or parsed.fragment
            or parsed.path not in ('', '/')):
        raise ValueError('Only an explicit loopback test origin is supported')
    return origin.rstrip('/')


def rgba16_png(width, height, adam7=False):
    """Stream constant RGBA16 rows through zlib, without a full-size raster."""
    if width <= 0 or height <= 0:
        raise ValueError('Invalid PNG dimensions')

    def chunk(kind, data):
        return (struct.pack('>I', len(data)) + kind + data
                + struct.pack('>I', zlib.crc32(kind + data) & 0xffffffff))

    compressor = zlib.compressobj()
    compressed = bytearray()
    for x, y, dx, dy in PASSES if adam7 else ((0, 0, 1, 1),):
        pw = max(0, (width - x + dx - 1) // dx)
        ph = max(0, (height - y + dy - 1) // dy)
        if not pw or not ph:
            continue
        row = b'\0' + b'\x80\0\x20\0\xc0\0\x80\0' * pw
        for _ in range(ph):
            compressed.extend(compressor.compress(row))
    compressed.extend(compressor.flush())
    header = struct.pack('>IIBBBBB', width, height, 16, 6, 0, 0, int(adam7))
    return (b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', header)
            + chunk(b'IDAT', bytes(compressed)) + chunk(b'IEND', b''))


def jpeg_dimensions(data):
    """Read JPEG SOF dimensions; pixel/color assertions live in Go codec tests."""
    if not data.startswith(b'\xff\xd8') or not data.endswith(b'\xff\xd9'):
        raise AssertionError('Invalid JPEG envelope')
    offset = 2
    while offset < len(data):
        if data[offset] != 0xff:
            raise AssertionError('Invalid JPEG marker')
        while offset < len(data) and data[offset] == 0xff:
            offset += 1
        if offset >= len(data):
            break
        marker = data[offset]
        offset += 1
        if marker in (0xd8, 0xd9) or 0xd0 <= marker <= 0xd7:
            continue
        if offset + 2 > len(data):
            break
        length = struct.unpack_from('>H', data, offset)[0]
        if length < 2 or offset + length > len(data):
            raise AssertionError('Invalid JPEG segment')
        if marker in (0xc0, 0xc1, 0xc2):
            if length < 8:
                raise AssertionError('Invalid JPEG SOF')
            height, width = struct.unpack_from('>HH', data, offset + 3)
            if not width or not height:
                raise AssertionError('Invalid JPEG dimensions')
            return width, height
        if marker == 0xda:
            break
        offset += length
    raise AssertionError('Missing JPEG SOF')


def clone_session(client):
    cloned = Client(client.origin)
    for cookie in client.jar:
        cloned.jar.set_cookie(copy.copy(cookie))
    cloned.csrf = client.csrf
    return cloned


def login(client, saved):
    response = client.request(API + '/auth/login', 'POST',
                              {'username': 'smoke-admin', 'password': saved['password']})
    client.csrf = response['csrfToken']
    assert client.request(API + '/me')['user']['username'] == 'smoke-admin'


def check_media(client, media):
    assert media['width'] == 1600 and media['height'] == 1067
    assert client.request(API + '/media/' + media['id'] + '/info') == media
    anonymous = Client(client.origin)
    for variant, key in (('master', 'masterUrl'), ('image', 'url'), ('thumbnail', 'thumbnailUrl')):
        data = client.request(media[key])
        width, height = jpeg_dimensions(data)
        if variant == 'master':
            assert (width, height) == (1600, 1067)
        else:
            assert width * 3 == height * 4
            assert max(width, height) <= (480 if variant == 'thumbnail' else 1600)
        anonymous.request(media[key], status=401)


def check_saved_state(client, saved):
    liquid = client.request(API + '/liquids/' + saved['liquidId'])
    assert liquid['name'] == 'Smoke Updated' and liquid['favorite']
    assert liquid['values'] == saved['values']
    assert client.request(API + '/liquids')['total'] == saved['liquidCount']
    assert len(client.request(API + '/fields')['items']) == saved['fieldCount']
    assert client.request(API + '/preferences') == saved['preferences']
    assert client.request(API + '/dashboard') == saved['dashboard']
    assert client.request(API + '/dashboard/query', 'POST',
                          {'widgets': saved['dashboard']['widgets']}) == saved['dashboardResult']
    assert hashlib.sha256(client.request(API + '/media/' + saved['mediaId'])).hexdigest() == saved['imageSHA256']


def exercise(origin, saved):
    client = Client(origin)
    client.request('/readyz')
    login(client, saved)
    check_saved_state(client, saved)
    png = rgba16_png(6000, 4000)
    assert len(png) <= 8 << 20
    media_ids = set()

    def upload(worker, source):
        result = worker.upload(API + '/media', 'synthetic-rgba16.png', source, 'image/png')
        check_media(worker, result)
        worker.request('/readyz')
        return result['id']

    # Repeated sequential decodes catch retained old heap between operations.
    for _ in range(2):
        media_ids.add(upload(client, png))
    barrier = threading.Barrier(4)

    def upload_job():
        worker = clone_session(client)
        barrier.wait(timeout=30)
        return upload(worker, png)

    def login_job():
        worker = Client(origin)
        barrier.wait(timeout=30)
        login(worker, saved)  # Real unchanged Argon verification, never a mock.
        worker.request('/readyz')
        return None

    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        jobs = [pool.submit(upload_job) for _ in range(2)] + [pool.submit(login_job) for _ in range(2)]
        for job in jobs:
            result = job.result()
            if result is not None:
                media_ids.add(result)
    adam7 = rgba16_png(6000, 4000, adam7=True)
    assert len(adam7) <= 8 << 20
    media_ids.add(upload(client, adam7))
    assert len(media_ids) == 5, 'Every upload must create a fresh immutable revision'
    client.request('/readyz')
    check_saved_state(client, saved)
    return {'status': 'PASS', 'uploads': 5, 'realMixedLogins': 2,
            'sourceDimensions': [6000, 4000], 'sourceBitDepth': 16,
            'checks': ['repeated-24mp', 'two-uploads-two-logins', 'adam7-24mp',
                       'private-jpeg-variants', 'readiness', 'saved-state-preserved']}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--url', required=True)
    parser.add_argument('--state-file', required=True, type=Path)
    args = parser.parse_args()
    try:
        origin = validate_origin(args.url)
        saved = json.loads(args.state_file.read_text())
        result = exercise(origin, saved)
    except Exception as error:
        # No response, exception message, URL, credentials or tokens in CI logs.
        print(json.dumps({'status': 'FAIL', 'errorType': type(error).__name__}))
        return 1
    print(json.dumps(result))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
