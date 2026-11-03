#!/usr/bin/env python3
"""Verify an immutable registry image before resuming publication; never writes tags."""
import argparse
import json
from pathlib import Path
import re
import subprocess

DIGEST = re.compile(r'sha256:[0-9a-f]{64}\Z')
PLATFORMS = {'amd64', 'arm64'}


def platform_manifests(index):
    found = {}
    for manifest in index.get('manifests', []):
        platform = manifest.get('platform', {})
        arch, os_name = platform.get('architecture'), platform.get('os')
        if (os_name, arch) == ('unknown', 'unknown') and manifest.get('annotations', {}).get('vnd.docker.reference.type') == 'attestation-manifest':
            continue
        if os_name != 'linux' or arch not in PLATFORMS or arch in found:
            raise ValueError('Unexpected or duplicate executable platform')
        if not DIGEST.fullmatch(manifest.get('digest', '')):
            raise ValueError('Invalid platform digest')
        found[arch] = manifest['digest']
    if set(found) != PLATFORMS:
        raise ValueError('Expected exactly linux/amd64 and linux/arm64')
    return found


def validate_identity(index, configs, source_sha, digest, expected_digest):
    if not re.fullmatch(r'[0-9a-f]{40}', source_sha):
        raise ValueError('Invalid reviewed source SHA')
    if not DIGEST.fullmatch(digest) or digest != expected_digest:
        raise ValueError('Immutable index digest changed')
    for arch in platform_manifests(index):
        config = configs[arch]
        if config.get('Os') != 'linux' or config.get('Architecture') != arch:
            raise ValueError('Platform config mismatch')
        labels = config.get('Config', {}).get('Labels', {})
        required = {
            'org.opencontainers.image.source': 'https://github.com/Buffer379x/puffolio',
            'org.opencontainers.image.revision': source_sha,
            'org.opencontainers.image.version': '0.1.0',
        }
        if any(labels.get(key) != value for key, value in required.items()):
            raise ValueError('Existing image does not match reviewed source/version')


def docker(*args):
    return subprocess.check_output(['docker', *args], text=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--image', required=True)
    parser.add_argument('--source-sha', required=True)
    parser.add_argument('--expected-digest', required=True)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    image = args.image
    digest = docker('buildx', 'imagetools', 'inspect', image + ':0.1.0', '--format', '{{.Manifest.Digest}}').strip()
    if not DIGEST.fullmatch(digest) or digest != args.expected_digest:
        raise SystemExit('Immutable index digest changed')
    index = json.loads(docker('buildx', 'imagetools', 'inspect', image + '@' + digest, '--raw'))
    configs = {}
    for arch, platform_digest in platform_manifests(index).items():
        ref = image + '@' + platform_digest
        subprocess.run(['docker', 'pull', '--platform', 'linux/' + arch, ref], check=True, stdout=subprocess.DEVNULL)
        configs[arch] = json.loads(docker('image', 'inspect', ref))[0]
    validate_identity(index, configs, args.source_sha, digest, args.expected_digest)
    # Recheck the version alias after reading all platform configurations.
    if docker('buildx', 'imagetools', 'inspect', image + ':0.1.0', '--format', '{{.Manifest.Digest}}').strip() != digest:
        raise SystemExit('Immutable version alias moved during verification')
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps({'digest': digest, 'sourceSha': args.source_sha, 'version': '0.1.0', 'platforms': platform_manifests(index)}, sort_keys=True) + '\n')
    print(digest)


if __name__ == '__main__':
    main()
