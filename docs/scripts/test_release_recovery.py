"""Fixed-image recovery contracts and real scanner-shell behavior with a fake daemon."""
import json
import copy
import importlib.util
import hashlib
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[2]
SCANNER = ROOT / 'docs/scripts/scan-image.sh'
PIN = 'aquasec/trivy@sha256:790fd66d1697395d8693d1acaa1ea702406a20f109b6fc7bac8db623eca6c06c'
ORIGINAL = '2c7d5e043aaf15e2f738b2fbb442fada003a635f'
INDEX = 'sha256:e4f9d8a2ab11e5e853670da5a0b7609474ef6428ffe9549e440e3d37f37f722d'
AMD = 'sha256:7cead596f8990604fcbda2afb59550d2cd80ebefc46320759cf48a888a9a47f4'
ARM = 'sha256:305c7f351d74c3947a8d31aaba8741f3207456b92210a0f495817ba93fa3119b'
SUBJECT = 'ghcr.io/buffer379x/puffolio@' + AMD


def report(subject=SUBJECT):
    return {'SchemaVersion': 2, 'Trivy': {'Version': '0.76.1'}, 'ArtifactName': subject,
            'ArtifactType': 'container_image', 'Results': [{'Target': 'debian', 'Class': 'os-pkgs',
            'Type': 'debian', 'Vulnerabilities': [{'VulnerabilityID':'GO2026-5932','Severity':'UNKNOWN'}]}]}


class ScannerShell(unittest.TestCase):
    def exercise(self, payload, code=0, subject=SUBJECT, platform='linux/amd64', stale=False):
        (ROOT/'work').mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='recovery-test-', dir=ROOT/'work') as directory:
            directory = Path(directory)
            fake = directory/'docker'
            fake.write_text(textwrap.dedent('''\
                #!/usr/bin/env python3
                import json,os,pathlib,sys
                args=sys.argv[1:]
                pathlib.Path(os.environ['FAKE_CAPTURE']).write_text(json.dumps(args))
                mount=next(args[i+1] for i,a in enumerate(args) if a=='--mount' and 'dst=/out' in args[i+1])
                dest=pathlib.Path(next(p[4:] for p in mount.split(',') if p.startswith('src=')))
                target=dest/pathlib.Path(args[args.index('--output')+1]).name
                target.write_text(os.environ['FAKE_REPORT'])
                sys.exit(int(os.environ['FAKE_EXIT']))
            '''))
            fake.chmod(0o755)
            output=directory/'scan-amd64.json';capture=directory/'argv.json'
            if stale:output.write_text('stale report')
            env=dict(os.environ,PATH=str(directory)+os.pathsep+os.environ['PATH'],
                     FAKE_CAPTURE=str(capture),FAKE_REPORT=json.dumps(payload) if not isinstance(payload,str) else payload,
                     FAKE_EXIT=str(code),PYTHONPYCACHEPREFIX=str(ROOT/'work/cache/python'))
            done=subprocess.run(['bash',str(SCANNER),subject,str(output),platform],cwd=ROOT,env=env,capture_output=True,text=True)
            return done, json.loads(capture.read_text()) if capture.exists() else None, \
                output.read_text() if output.exists() else None, \
                json.loads(Path(str(output)+'.scanner.json').read_text()) if Path(str(output)+'.scanner.json').exists() else None

    def test_pinned_remote_full_report_and_actual_version_metadata(self):
        done,args,raw,metadata=self.exercise(report())
        self.assertEqual(done.returncode,0,done.stderr)
        self.assertIn(PIN,args)
        self.assertIn('size=512m',args[args.index('--tmpfs')+1])
        self.assertEqual(args[args.index('--image-src')+1],'remote')
        self.assertEqual(args[args.index('--severity')+1],'UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL')
        self.assertIn('--ignore-unfixed=false',args)
        self.assertIn('--skip-db-update=false',args)
        self.assertNotIn('/var/run/docker.sock',' '.join(args))
        self.assertNotIn('/.docker',' '.join(args))
        self.assertNotIn('GITHUB_TOKEN',' '.join(args))
        self.assertEqual(json.loads(raw),report())
        self.assertEqual(metadata['subject'],SUBJECT)
        self.assertEqual(metadata['scannerImage'],PIN)
        self.assertEqual(metadata['scannerVersion'],'0.76.1')

    def test_scanner_error_propagates_and_keeps_partial_report(self):
        done,args,raw,metadata=self.exercise('{"partial":true}',42)
        self.assertEqual(done.returncode,42)
        self.assertIsNotNone(args)
        self.assertEqual(raw,'{"partial":true}')
        self.assertIsNone(metadata)

    def test_malformed_missing_subject_or_wrong_version_is_not_success(self):
        for payload in ('not json',{},dict(report(),ArtifactName='wrong'),dict(report(),Trivy={'Version':'wrong'}),dict(report(),Results=[])):
            with self.subTest(payload=payload):
                done,args,raw,metadata=self.exercise(payload)
                self.assertIsNotNone(args)
                self.assertNotEqual(done.returncode,0)
                self.assertIsNone(metadata)

    def test_stale_report_is_refused_before_scanner_execution(self):
        done,args,_,metadata=self.exercise(report(),stale=True)
        self.assertNotEqual(done.returncode,0)
        self.assertIsNone(args)
        self.assertIsNone(metadata)

    def test_mutable_or_wrong_platform_input_never_calls_docker(self):
        for subject,platform in [('ghcr.io/buffer379x/puffolio:0.1.0','linux/amd64'),(SUBJECT,'linux/386')]:
            with self.subTest(subject=subject,platform=platform):
                done,args,_,_=self.exercise(report(),subject=subject,platform=platform)
                self.assertNotEqual(done.returncode,0)
                self.assertIsNone(args)


class WorkflowContract(unittest.TestCase):
    def test_normal_release_replaces_only_scanner_delivery(self):
        workflow=(ROOT/'.github/workflows/release.yml').read_text()
        self.assertIn('bash docs/scripts/scan-image.sh',workflow)
        self.assertNotIn('aquasecurity/trivy-action@',workflow)
        self.assertIn('python3 docs/scripts/scan-policy.py',workflow)
        self.assertIn('tags: ghcr.io/buffer379x/puffolio:0.1.0',workflow)
        self.assertIn('python3 docs/scripts/release-preflight.py',workflow)

    def test_fixed_recovery_uses_original_source_and_no_registry_writers(self):
        path=ROOT/'.github/workflows/recover-0.1.0.yml'
        self.assertTrue(path.exists(),'fixed recovery workflow missing')
        workflow=path.read_text()
        for value in [ORIGINAL,INDEX,AMD,ARM,'puffolio-release-0.1.0','ubuntu-24.04-arm','path: control','path: original',
                      '--source-digest "$ORIGINAL_SHA"','--source-ref refs/heads/main','--deny-self-hosted-runners',
                      'Buffer379x/puffolio/.github/workflows/release.yml','--verify-tag','repos/Buffer379x/puffolio/git/refs']:
            self.assertIn(value,workflow)
        for forbidden in ['packages: write','id-token: write','attestations: write','build-push-action',
                          'attest-build-provenance','docker push','docker build ','--force','get.trivy.dev']:
            self.assertNotIn(forbidden,workflow)
        self.assertIn('original/docs/scripts/scan-policy.py',workflow)
        self.assertIn('bash docs/scripts/container-smoke.sh',workflow)
        self.assertIn('bash control/docs/scripts/scan-image.sh',workflow)
        self.assertIn('needs: [validate-existing, native-verification]',workflow)
        self.assertLess(workflow.index('check-absent'),workflow.index('gh api --method POST'))
        self.assertLess(workflow.index('check-tag'),workflow.index('gh release create'))
        self.assertIn('-f sha="$ORIGINAL_SHA"',workflow)
        self.assertIn('--notes-file original/docs/release-0.1.0.md',workflow)


class FixedEvidenceGuards(unittest.TestCase):
    def guard(self):
        path=ROOT/'docs/scripts/recovery-release.py'
        self.assertTrue(path.exists(),'fixed identity/evidence guard missing')
        spec=importlib.util.spec_from_file_location('recovery_release',path)
        module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
        return module

    def test_current_recovery_commit_cannot_replace_original_product_identity(self):
        guard=self.guard()
        identity={'digest':INDEX,'sourceSha':ORIGINAL,'version':'0.1.0','platforms':{'amd64':AMD,'arm64':ARM}}
        guard.check_identity(identity)
        for key,value in [('sourceSha','a'*40),('digest','sha256:'+'0'*64),('version','latest')]:
            bad=dict(identity);bad[key]=value
            with self.subTest(key=key),self.assertRaises(ValueError):guard.check_identity(bad)
        bad=copy.deepcopy(identity);bad['platforms']['arm64']=AMD
        with self.assertRaises(ValueError):guard.check_identity(bad)

    def test_tag_release_denial_or_existing_object_fails_closed(self):
        guard=self.guard()
        def fake(replies):
            def get(stage,url,headers):
                status,body=replies.pop(0)
                return status,{},json.dumps(body).encode()
            return get
        absent=[(200,{'full_name':'Buffer379x/puffolio'}),(404,{'message':'Not Found'}),(404,{'message':'Not Found'})]
        guard.check_absent(fake(list(absent)),'secret')
        for replies in [[(200,{'full_name':'Buffer379x/puffolio'}),(200,{'tag_name':'v0.1.0'})],
                        [(200,{'full_name':'Buffer379x/puffolio'}),(404,{'message':'Not Found'}),(200,{'ref':'refs/tags/v0.1.0'})],
                        [(403,{'message':'secret'})],[(429,{})],[(500,{})],
                        [(200,{'full_name':'Buffer379x/puffolio'}),(404,{'message':'denied'})]]:
            with self.subTest(replies=replies),self.assertRaises(Exception) as caught:
                guard.check_absent(fake(replies),'secret')
            self.assertNotIn('secret',str(caught.exception))
        good={'ref':'refs/tags/v0.1.0','object':{'type':'commit','sha':ORIGINAL}}
        guard.check_tag(fake([(200,good)]),'secret')
        bad=copy.deepcopy(good);bad['object']['sha']='a'*40
        with self.assertRaises(ValueError):guard.check_tag(fake([(200,bad)]),'secret')

    def test_missing_mismatched_or_tampered_platform_evidence_is_refused(self):
        guard=self.guard()
        with tempfile.TemporaryDirectory(prefix='recovery-assets-',dir=ROOT/'work') as directory:
            root=Path(directory);original=root/'original';evidence=root/'evidence';identity=root/'identity'
            (original/'deploy/unraid').mkdir(parents=True);evidence.mkdir();identity.mkdir()
            (original/'deploy/compose.yaml').write_text('original compose')
            (original/'deploy/unraid/puffolio.xml').write_text('<original/>')
            (original/'docs/scripts').mkdir(parents=True)
            (original/'docs/scripts/scan-policy.py').write_bytes((ROOT/'docs/scripts/scan-policy.py').read_bytes())
            env={'GITHUB_SHA':'b'*40,'GITHUB_RUN_ID':'123','GITHUB_RUN_ATTEMPT':'1'}
            for arch,digest in [('amd64',AMD),('arm64',ARM)]:
                scan=evidence/f'scan-{arch}.json';scan.write_text(json.dumps(report('ghcr.io/buffer379x/puffolio@'+digest)))
                guard.scan_metadata(scan,'ghcr.io/buffer379x/puffolio@'+digest,'linux/'+arch)
                (evidence/f'puffolio-{arch}.spdx.json').write_text(json.dumps({'spdxVersion':'SPDX-2.3','packages':[{'name':'puffolio'}]}))
                guard.receipt(evidence,arch,dict(env,RUNNER_ARCH={'amd64':'X64','arm64':'ARM64'}[arch]))
            (identity/'index.json').write_text(json.dumps({'digest':INDEX,'sourceSha':ORIGINAL,'version':'0.1.0','platforms':{'amd64':AMD,'arm64':ARM}}))
            (identity/'attestation.json').write_text(json.dumps([{'verificationResult':{'statement':{'subject':[{'digest':{'sha256':INDEX.split(':')[1]}}]}}}]))
            guard.assemble(original,evidence,identity,root/'output',env)
            self.assertTrue((root/'output/SHA256SUMS').exists())
            self.assertEqual(json.loads((root/'output/verification-evidence.json').read_text())['sourceSha'],ORIGINAL)
            saved_files={p.name:p.read_bytes() for p in evidence.iterdir()}
            scan=evidence/'scan-arm64.json';high=json.loads(scan.read_text())
            high['Results'][0]['Vulnerabilities']=[{'VulnerabilityID':'SYNTHETIC-HIGH','Severity':'HIGH'}]
            scan.write_text(json.dumps(high))
            meta=Path(str(scan)+'.scanner.json');meta.unlink()
            guard.scan_metadata(scan,'ghcr.io/buffer379x/puffolio@'+ARM,'linux/arm64')
            guard.receipt(evidence,'arm64',dict(env,RUNNER_ARCH='ARM64'))
            with self.assertRaisesRegex(ValueError,'original vulnerability policy'):
                guard.assemble(original,evidence,identity,root/'blocked',env)
            self.assertFalse((root/'blocked').exists())
            for name,data in saved_files.items():(evidence/name).write_bytes(data)
            path=evidence/'scan-arm64.json';saved=path.read_bytes();path.write_text('{}')
            with self.assertRaises(ValueError):guard.assemble(original,evidence,identity,root/'bad',env)
            path.write_bytes(saved)
            receipt=evidence/'verification-arm64.json';data=json.loads(receipt.read_text());data['platformDigest']=AMD;receipt.write_text(json.dumps(data))
            with self.assertRaises(ValueError):guard.assemble(original,evidence,identity,root/'wrong',env)
            receipt.unlink()
            with self.assertRaises(ValueError):guard.assemble(original,evidence,identity,root/'missing',env)


if __name__=='__main__':
    unittest.main()
