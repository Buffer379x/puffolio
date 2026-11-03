#!/usr/bin/env python3
"""Fail release on HIGH/CRITICAL runtime findings; retain complete input report."""
import json,sys
report=json.load(open(sys.argv[1]))
if not isinstance(report.get('Results'),list):raise SystemExit('Invalid or incomplete scanner report')
blocking=[]
for result in report['Results']:
    for issue in result.get('Vulnerabilities') or []:
        if issue.get('Severity') in ('HIGH','CRITICAL'):
            blocking.append((issue['VulnerabilityID'],issue.get('PkgName'),issue.get('InstalledVersion'),issue.get('FixedVersion','unfixed')))
for item in blocking:print(*item)
if blocking:raise SystemExit('Runtime vulnerability policy failed')
print('No HIGH/CRITICAL runtime findings')
