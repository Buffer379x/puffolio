"""Status-driven release preflight: distinguish absence from denied access safely."""
import base64
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import unittest
import tempfile
import urllib.request
import urllib.parse

spec = importlib.util.spec_from_file_location('release_preflight', Path(__file__).with_name('release-preflight.py'))
preflight = importlib.util.module_from_spec(spec)
spec.loader.exec_module(preflight)

CREDENTIAL = 'credential-DO-NOT-LOG'
BEARER = 'bearer-DO-NOT-LOG'
INDEX = json.dumps({'schemaVersion': 2, 'mediaType': 'application/vnd.oci.image.index.v1+json',
                    'manifests': [{'digest': 'sha256:' + 'a' * 64}]}).encode()
DIGEST = 'sha256:' + hashlib.sha256(INDEX).hexdigest()


def response(status, payload=None, headers=None):
    body = payload if isinstance(payload, bytes) else json.dumps(payload).encode()
    return status, headers or {}, body


def responses(manifest=None):
    return [response(200, {'full_name': 'Buffer379x/puffolio'}),
            response(404, {'message': 'Not Found'}),
            response(200, {'token': BEARER}),
            manifest or response(404, {'errors': [{'code': 'MANIFEST_UNKNOWN', 'message': 'missing'}]})]


class ScriptedHTTP:
    def __init__(self, replies):
        self.replies = list(replies)
        self.calls = []

    def __call__(self, stage, url, headers):
        self.calls.append((stage, url, headers))
        reply = self.replies.pop(0)
        if isinstance(reply, Exception):
            raise reply
        return reply


class ReleasePreflight(unittest.TestCase):
    def run_probe(self, replies, **kwargs):
        http = ScriptedHTTP(replies)
        messages = []
        result = preflight.probe_release(http, 'Buffer379x', CREDENTIAL, messages.append, **kwargs)
        return result, http, '\n'.join(messages)

    def test_authenticated_structured_absence_permits_first_build(self):
        result, http, messages = self.run_probe(responses())
        self.assertEqual(result, {'exists': False})
        self.assertEqual(len(http.calls), 4)
        self.assertIn('stage=github-tag status=404 class=absent', messages)
        self.assertIn('stage=ghcr-manifest status=404 class=absent', messages)
        token_url = urllib.parse.urlsplit(http.calls[2][1])
        self.assertEqual((token_url.scheme, token_url.netloc, token_url.path), ('https', 'ghcr.io', '/token'))
        self.assertEqual(urllib.parse.parse_qs(token_url.query), {
            'service': ['ghcr.io'], 'scope': ['repository:buffer379x/puffolio:pull,push']})
        encoded = base64.b64encode(('Buffer379x:' + CREDENTIAL).encode()).decode()
        self.assertEqual(http.calls[2][2]['Authorization'], 'Basic ' + encoded)
        self.assertEqual(http.calls[3][2]['Authorization'], 'Bearer ' + BEARER)
        for secret in (CREDENTIAL, BEARER, encoded):
            self.assertNotIn(secret, messages)

    def test_structured_name_unknown_is_also_absent(self):
        result, _, _ = self.run_probe(responses(response(404, {'errors': [{'code': 'NAME_UNKNOWN'}]})))
        self.assertEqual(result, {'exists': False})

    def test_existing_manifest_returns_hash_verified_digest_for_identity_validation(self):
        result, _, messages = self.run_probe(responses(response(200, INDEX, {'Docker-Content-Digest': DIGEST})))
        self.assertEqual(result, {'exists': True, 'digest': DIGEST})
        self.assertIn('stage=ghcr-manifest status=200 class=existing', messages)

    def test_existing_git_tag_is_refused_before_registry_authentication(self):
        http = ScriptedHTTP([response(200, {'full_name': 'Buffer379x/puffolio'}),
                             response(200, {'ref': 'refs/tags/v0.1.0', 'object': {'type': 'commit'}})])
        with self.assertRaisesRegex(preflight.PreflightError, 'stage=github-tag status=200 class=existing-tag'):
            preflight.probe_release(http, 'Buffer379x', CREDENTIAL, lambda _: None)
        self.assertEqual(len(http.calls), 2)

    def test_denied_rate_and_unexpected_statuses_fail_closed_at_every_stage(self):
        for index, stage in enumerate(['github-repository', 'github-tag', 'ghcr-token', 'ghcr-manifest']):
            for status in (301, 302, 401, 403, 429, 500, 503):
                with self.subTest(stage=stage, status=status):
                    replies = responses()
                    replies[index] = response(status, {'message': CREDENTIAL, 'token': BEARER})
                    with self.assertRaisesRegex(preflight.PreflightError, f'stage={stage} status={status} class=') as caught:
                        self.run_probe(replies)
                    self.assertNotIn(CREDENTIAL, str(caught.exception))
                    self.assertNotIn(BEARER, str(caught.exception))

    def test_network_errors_never_leak_the_exception_payload(self):
        for index, stage in enumerate(['github-repository', 'github-tag', 'ghcr-token', 'ghcr-manifest']):
            with self.subTest(stage=stage):
                replies = responses()
                replies[index] = OSError('https://untrusted.invalid/?sig=' + CREDENTIAL + BEARER)
                with self.assertRaisesRegex(preflight.PreflightError, f'stage={stage} status=none class=network') as caught:
                    self.run_probe(replies)
                self.assertNotIn('untrusted', str(caught.exception))
                self.assertNotIn(CREDENTIAL, str(caught.exception))

    def test_unstructured_404_or_denied_error_payload_cannot_authorize_first_build(self):
        for body in (b'not found', b'{}', b'{', b'null',
                     b'{"errors":[]}', b'{"errors":[{"code":"DENIED"}]}',
                     b'{"errors":[{"code":"MANIFEST_UNKNOWN"},{"code":"UNAUTHORIZED"}]}'):
            with self.subTest(body=body), self.assertRaises(preflight.PreflightError):
                self.run_probe(responses(response(404, body)))

    def test_malformed_success_repo_token_manifest_or_digest_fails_closed(self):
        cases = [(0, response(200, {'full_name': 'wrong/repository'})),
                 (0, response(200, {'full_name': None})),
                 (1, response(404, b'not found')),
                 (2, response(200, {'token': ''})),
                 (2, response(200, {'token': BEARER, 'access_token': 'different'})),
                 (2, response(200, {'token': BEARER + '\nInjected: yes'})),
                 (3, response(200, INDEX, {})),
                 (3, response(200, INDEX, {'Docker-Content-Digest': 'sha256:' + '0' * 64})),
                 (3, response(200, b'not found', {'Docker-Content-Digest': DIGEST})),
                 (3, response(200, {'schemaVersion': 1}, {'Docker-Content-Digest': DIGEST}))]
        for index, reply in cases:
            with self.subTest(index=index, reply=reply), self.assertRaises(preflight.PreflightError):
                replies = responses(); replies[index] = reply
                self.run_probe(replies)

    def test_invalid_repository_reference_actor_or_credential_never_reaches_http(self):
        for kwargs in ({'repository': 'evil.invalid/repo?token=secret'}, {'repository': '../puffolio'},
                       {'reference': '0.1.0/../../private'}, {'reference': 'latest'}):
            http = ScriptedHTTP([])
            with self.subTest(kwargs=kwargs), self.assertRaises(preflight.PreflightError):
                preflight.probe_release(http, 'Buffer379x', CREDENTIAL, lambda _: None, **kwargs)
            self.assertFalse(http.calls)
        for actor, credential in [('user\r\nInjected:yes', CREDENTIAL), ('Buffer379x', ''), ('Buffer379x', CREDENTIAL+'\n')]:
            http = ScriptedHTTP([])
            with self.subTest(actor=actor), self.assertRaises(preflight.PreflightError):
                preflight.probe_release(http, actor, credential, lambda _: None)
            self.assertFalse(http.calls)


class TransportAndWorkflow(unittest.TestCase):
    def test_credentials_are_not_forwarded_to_redirect_destinations(self):
        handler = preflight.NoRedirect()
        original = urllib.request.Request('https://ghcr.io/token', headers={'Authorization': 'Basic '+CREDENTIAL})
        for location in ('https://untrusted.invalid/?sig='+BEARER, 'http://ghcr.io/token', 'https://ghcr.io/other'):
            with self.subTest(location=location):
                self.assertIsNone(handler.redirect_request(original, None, 302, 'Found', {}, location))

    def test_non_https_credentials_hosts_and_ports_are_rejected_before_io(self):
        client = preflight.HTTPSGet()
        for url in ('http://ghcr.io/token', 'https://untrusted.invalid/token',
                    'https://user@ghcr.io/token', 'https://ghcr.io:444/token', 'https://ghcr.io/token#fragment'):
            with self.subTest(url=url), self.assertRaisesRegex(preflight.PreflightError, 'unsafe-endpoint'):
                client('ghcr-token', url, {'Authorization': 'Basic '+CREDENTIAL})

    def test_real_cli_main_emits_only_safe_workflow_outputs(self):
        root = Path(__file__).resolve().parents[2] / 'work'
        root.mkdir(exist_ok=True)
        for reply, expected in [(None, 'exists=false\n'),
                                (response(200, INDEX, {'Docker-Content-Digest': DIGEST}),
                                 'exists=true\ndigest='+DIGEST+'\n')]:
            with self.subTest(expected=expected), tempfile.TemporaryDirectory(dir=root) as directory:
                target = Path(directory) / 'outputs'
                stdout, stderr = io.StringIO(), io.StringIO()
                code = preflight.main(ScriptedHTTP(responses(reply)),
                    {'GITHUB_ACTOR':'Buffer379x','GH_TOKEN':CREDENTIAL,'GITHUB_OUTPUT':str(target)},stdout,stderr)
                self.assertEqual(code, 0)
                self.assertEqual(target.read_text(), expected)
                for secret in (CREDENTIAL, BEARER):
                    self.assertNotIn(secret, stdout.getvalue()+stderr.getvalue()+target.read_text())

    def test_main_suppresses_secret_payloads_on_unexpected_local_errors(self):
        stdout, stderr = io.StringIO(), io.StringIO()
        fake = ScriptedHTTP([RuntimeError('https://untrusted.invalid/?sig='+CREDENTIAL+BEARER)])
        self.assertEqual(preflight.main(fake, {'GITHUB_ACTOR':'Buffer379x','GH_TOKEN':CREDENTIAL},stdout,stderr), 1)
        self.assertEqual(stderr.getvalue(), 'release-preflight stage=internal status=none class=unexpected\n')
        self.assertNotIn(CREDENTIAL, stdout.getvalue()+stderr.getvalue())

    def test_workflow_uses_status_probe_and_keeps_resume_before_immutable_push(self):
        workflow = (Path(__file__).resolve().parents[2] / '.github/workflows/release.yml').read_text()
        publish = workflow.split('  publish-image:',1)[1].split('  signed-provenance:',1)[0]
        self.assertIn('needs: checks', publish)
        self.assertIn('python3 docs/scripts/release-preflight.py', publish)
        self.assertNotIn('work/release/tag-error', publish)
        self.assertNotIn('work/release/image-error', publish)
        self.assertIn("if: steps.version.outputs.exists == 'true'", publish)
        self.assertIn("if: steps.version.outputs.exists != 'true'", publish)
        self.assertLess(publish.index('python3 docs/scripts/release-image.py'), publish.index('name: Publish exact version'))
        self.assertIn('tags: ghcr.io/buffer379x/puffolio:0.1.0', publish)
        self.assertIn('${{ steps.build.outputs.digest || steps.version.outputs.digest }}', publish)


if __name__ == '__main__':
    unittest.main()
