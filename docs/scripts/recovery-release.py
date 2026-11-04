#!/usr/bin/env python3
"""Fixed published-0.1.0 identity, scan evidence and non-overwriting release guards."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

SOURCE = '2c7d5e043aaf15e2f738b2fbb442fada003a635f'
IMAGE = 'ghcr.io/buffer379x/puffolio'
INDEX = 'sha256:e4f9d8a2ab11e5e853670da5a0b7609474ef6428ffe9549e440e3d37f37f722d'
PLATFORMS = {
    'amd64': 'sha256:7cead596f8990604fcbda2afb59550d2cd80ebefc46320759cf48a888a9a47f4',
    'arm64': 'sha256:305c7f351d74c3947a8d31aaba8741f3207456b92210a0f495817ba93fa3119b',
}
SCANNER = 'aquasec/trivy@sha256:790fd66d1697395d8693d1acaa1ea702406a20f109b6fc7bac8db623eca6c06c'
VERSION = '0.76.1'
_API = None


def require(condition, reason):
    if not condition:
        raise ValueError('Recovery check failed: ' + reason)


def read_json(path):
    return json.loads(Path(path).read_text())


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def check_identity(data):
    require(isinstance(data, dict) and data.get('sourceSha') == SOURCE and data.get('digest') == INDEX
            and data.get('version') == '0.1.0' and data.get('platforms') == PLATFORMS, 'original image identity')


def platform(data, arch):
    require(arch in PLATFORMS and isinstance(data, dict), 'platform input')
    found = {}
    for item in data.get('manifests', []):
        info = item.get('platform', {})
        if info.get('os') == 'linux':
            name = info.get('architecture')
            require(name in PLATFORMS and name not in found, 'unexpected platform')
            found[name] = item.get('digest')
    require(found == PLATFORMS, 'platform digest mismatch')
    return found[arch]


def scan_metadata(path, subject, target):
    path = Path(path)
    require(re.fullmatch(r'ghcr\.io/buffer379x/puffolio@sha256:[0-9a-f]{64}', subject) is not None
            and target in ('linux/amd64', 'linux/arm64'), 'immutable scan input')
    report = read_json(path)
    require(isinstance(report, dict) and report.get('SchemaVersion') == 2
            and report.get('ArtifactType') == 'container_image' and report.get('ArtifactName') == subject
            and isinstance(report.get('Trivy'), dict) and report['Trivy'].get('Version') == VERSION
            and isinstance(report.get('Results'), list) and bool(report['Results'])
            and all(isinstance(item, dict) for item in report['Results']), 'complete subject-bound scanner JSON')
    metadata = {'subject': subject, 'platform': target, 'scannerImage': SCANNER,
                'scannerVersion': report['Trivy']['Version'], 'reportSha256': sha(path)}
    destination = Path(str(path) + '.scanner.json')
    require(not destination.exists(), 'existing scanner receipt')
    destination.write_text(json.dumps(metadata, indent=2, sort_keys=True) + '\n')


def run_identity(environ):
    control = environ.get('GITHUB_SHA', '')
    run = environ.get('GITHUB_RUN_ID', '')
    attempt = environ.get('GITHUB_RUN_ATTEMPT', '')
    require(re.fullmatch(r'[0-9a-f]{40}', control) is not None and run.isdigit() and attempt.isdigit(), 'workflow run identity')
    return {'controlSha': control, 'runId': run, 'runAttempt': attempt,
            'workflow': 'Buffer379x/puffolio/.github/workflows/recover-0.1.0.yml',
            'runUrl': 'https://github.com/Buffer379x/puffolio/actions/runs/' + run}


def evidence_names(arch):
    return [f'scan-{arch}.json', f'scan-{arch}.json.scanner.json', f'puffolio-{arch}.spdx.json']


def validate_platform_evidence(directory, arch):
    require(arch in PLATFORMS, 'unknown evidence architecture')
    directory = Path(directory)
    names = evidence_names(arch)
    require(all((directory / name).is_file() and not (directory / name).is_symlink() for name in names), 'missing scan/SPDX evidence')
    metadata = read_json(directory / names[1])
    require(metadata == {'subject': IMAGE + '@' + PLATFORMS[arch], 'platform': 'linux/' + arch,
                         'scannerImage': SCANNER, 'scannerVersion': VERSION,
                         'reportSha256': sha(directory / names[0])}, 'scanner evidence mismatch')
    scan = read_json(directory / names[0])
    require(scan.get('ArtifactName') == metadata['subject'] and scan.get('Trivy', {}).get('Version') == VERSION
            and isinstance(scan.get('Results'), list) and bool(scan['Results']), 'scanner report mismatch')
    spdx = read_json(directory / names[2])
    require(isinstance(spdx, dict) and str(spdx.get('spdxVersion', '')).startswith('SPDX-')
            and isinstance(spdx.get('packages'), list) and bool(spdx['packages']), 'SPDX evidence')
    return {name: sha(directory / name) for name in names}


def receipt(directory, arch, environ):
    directory = Path(directory)
    files = validate_platform_evidence(directory, arch)
    runner = environ.get('RUNNER_ARCH', '')
    require(runner == {'amd64': 'X64', 'arm64': 'ARM64'}[arch], 'native runner architecture')
    data = dict(run_identity(environ), sourceSha=SOURCE, indexDigest=INDEX, platform='linux/' + arch,
                platformDigest=PLATFORMS[arch], runnerArch=runner, files=files)
    (directory / f'verification-{arch}.json').write_text(json.dumps(data, indent=2, sort_keys=True) + '\n')


def assemble(original, evidence, identity, output, environ):
    original, evidence, identity, output = map(Path, (original, evidence, identity, output))
    expected = {name for arch in PLATFORMS for name in evidence_names(arch) + [f'verification-{arch}.json']}
    require(evidence.is_dir() and {p.name for p in evidence.iterdir()} == expected, 'complete exact platform asset set')
    require(identity.is_dir() and {p.name for p in identity.iterdir()} == {'index.json', 'attestation.json'}, 'identity/provenance asset set')
    check_identity(read_json(identity / 'index.json'))
    attestation = read_json(identity / 'attestation.json')
    require(isinstance(attestation, list) and bool(attestation) and any(
        subject.get('digest', {}).get('sha256') == INDEX.split(':')[1]
        for item in attestation for subject in item.get('verificationResult', {}).get('statement', {}).get('subject', [])
    ), 'verified provenance subject')
    current = run_identity(environ)
    for arch in PLATFORMS:
        files = validate_platform_evidence(evidence, arch)
        proof = read_json(evidence / f'verification-{arch}.json')
        require(proof.get('sourceSha') == SOURCE and proof.get('indexDigest') == INDEX
                and proof.get('platform') == 'linux/' + arch and proof.get('platformDigest') == PLATFORMS[arch]
                and proof.get('runnerArch') == {'amd64': 'X64', 'arm64': 'ARM64'}[arch]
                and proof.get('controlSha') == current['controlSha'] and proof.get('runId') == current['runId']
                and proof.get('files') == files, 'platform verification receipt mismatch')
        require(subprocess.run([sys.executable, str(original / 'docs/scripts/scan-policy.py'),
                                str(evidence / f'scan-{arch}.json')]).returncode == 0, 'original vulnerability policy')
    require(not output.exists(), 'existing release asset directory')
    output.mkdir(parents=True)
    for name in sorted(expected):
        shutil.copyfile(evidence / name, output / name)
    shutil.copyfile(original / 'deploy/compose.yaml', output / 'compose.yaml')
    shutil.copyfile(original / 'deploy/unraid/puffolio.xml', output / 'puffolio.xml')
    shutil.copyfile(identity / 'index.json', output / 'image-identity.json')
    shutil.copyfile(identity / 'attestation.json', output / 'provenance.json')
    (output / 'image-digests.txt').write_text('index ' + INDEX + '\n' + ''.join(
        'linux/' + arch + ' ' + digest + '\n' for arch, digest in PLATFORMS.items()))
    summary = dict(current, sourceSha=SOURCE, image=IMAGE, indexDigest=INDEX, platforms=PLATFORMS,
                   scannerImage=SCANNER, scannerVersion=VERSION,
                   originalBuildRun='https://github.com/Buffer379x/puffolio/actions/runs/41783511454',
                   existingAttestation='https://github.com/Buffer379x/puffolio/attestations/63449559',
                   files={p.name: sha(p) for p in sorted(output.iterdir())})
    (output / 'verification-evidence.json').write_text(json.dumps(summary, indent=2, sort_keys=True) + '\n')
    (output / 'SHA256SUMS').write_text(''.join(sha(p) + '  ' + p.name + '\n' for p in sorted(output.iterdir())))
    print('Validated complete fixed-source recovery assets and checksums')


def api_helpers():
    global _API
    if _API is not None:
        return _API
    spec = importlib.util.spec_from_file_location('release_preflight', Path(__file__).with_name('release-preflight.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    _API = module
    return module


def api_get(http, token, path, stage):
    require(bool(token) and not any(ord(char) < 33 or ord(char) > 126 for char in token), 'GitHub credential configuration')
    helper = api_helpers()
    status, _, body = helper.request(http, stage, 'https://api.github.com/repos/Buffer379x/puffolio' + path,
        {'Authorization': 'Bearer ' + token, 'Accept': 'application/vnd.github+json',
         'X-GitHub-Api-Version': '2022-11-28', 'User-Agent': 'Puffolio-fixed-release-recovery'})
    return status, helper.json_object(stage, status, body)


def check_absent(http, token):
    status, data = api_get(http, token, '', 'recovery-repository')
    require(status == 200 and data.get('full_name') == 'Buffer379x/puffolio', 'repository access')
    for path, stage in [('/releases/tags/v0.1.0', 'recovery-release'), ('/git/ref/tags/v0.1.0', 'recovery-tag')]:
        status, data = api_get(http, token, path, stage)
        require(status == 404 and data.get('message') == 'Not Found', stage + ' must be absent')
        print(stage + ' status=404 class=absent')


def check_tag(http, token):
    status, data = api_get(http, token, '/git/ref/tags/v0.1.0', 'recovery-tag-readback')
    require(status == 200 and data.get('ref') == 'refs/tags/v0.1.0'
            and data.get('object', {}).get('type') == 'commit'
            and data['object'].get('sha') == SOURCE, 'exact original-source tag target')
    print('Verified tag targets original published source')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=['scan-metadata', 'identity', 'platform', 'receipt', 'assemble', 'check-absent', 'check-tag'])
    parser.add_argument('values', nargs='*')
    args = parser.parse_args()
    values = args.values
    try:
        if args.command == 'scan-metadata': scan_metadata(*values)
        elif args.command == 'identity': check_identity(read_json(*values))
        elif args.command == 'platform': print(platform(read_json(values[0]), values[1]))
        elif args.command == 'receipt': receipt(*values, os.environ)
        elif args.command == 'assemble': assemble(*values, os.environ)
        else:
            http = api_helpers().HTTPSGet()
            (check_absent if args.command == 'check-absent' else check_tag)(http, os.environ.get('GH_TOKEN', ''))
        return 0
    except Exception as error:
        # Only our fixed diagnostics can cross the logging boundary.
        if isinstance(error, api_helpers().PreflightError):
            print(str(error), file=sys.stderr)
        elif isinstance(error, ValueError) and str(error).startswith('Recovery check failed: '):
            print(str(error), file=sys.stderr)
        else:
            print('Recovery guard failed: ' + args.command, file=sys.stderr)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
