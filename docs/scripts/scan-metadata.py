#!/usr/bin/env python3
"""Bind a Trivy JSON report to the exact image digest and scanner it came from."""
import hashlib
import json
import re
import sys
from pathlib import Path

SCANNER = 'aquasec/trivy@sha256:790fd66d1697395d8693d1acaa1ea702406a20f109b6fc7bac8db623eca6c06c'
VERSION = '0.76.1'


def require(condition, reason):
    if not condition:
        raise SystemExit('Scan check failed: ' + reason)


def main(path, subject, target):
    path = Path(path)
    require(re.fullmatch(r'ghcr\.io/buffer379x/puffolio@sha256:[0-9a-f]{64}', subject) is not None
            and target in ('linux/amd64', 'linux/arm64'), 'immutable scan input')
    report = json.loads(path.read_text())
    require(isinstance(report, dict) and report.get('SchemaVersion') == 2
            and report.get('ArtifactType') == 'container_image' and report.get('ArtifactName') == subject
            and isinstance(report.get('Trivy'), dict) and report['Trivy'].get('Version') == VERSION
            and isinstance(report.get('Results'), list) and bool(report['Results'])
            and all(isinstance(item, dict) for item in report['Results']), 'complete subject-bound scanner JSON')
    destination = Path(str(path) + '.scanner.json')
    require(not destination.exists(), 'existing scanner receipt')
    destination.write_text(json.dumps({
        'subject': subject, 'platform': target, 'scannerImage': SCANNER,
        'scannerVersion': report['Trivy']['Version'],
        'reportSha256': hashlib.sha256(path.read_bytes()).hexdigest(),
    }, indent=2, sort_keys=True) + '\n')


if __name__ == '__main__':
    main(*sys.argv[1:])
