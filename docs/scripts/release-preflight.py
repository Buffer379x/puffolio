#!/usr/bin/env python3
"""Read-only, status-based GitHub/GHCR preflight for an immutable Puffolio release."""
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import ssl
import sys
import urllib.error
import urllib.parse
import urllib.request

MAX_RESPONSE_BYTES = 2 * 1024 * 1024
INDEX_TYPES = {
    'application/vnd.oci.image.index.v1+json',
    'application/vnd.docker.distribution.manifest.list.v2+json',
}


class PreflightError(Exception):
    """Only fixed stage/status/class diagnostics; never remote text or credentials."""


def fail(stage, status, reason):
    raise PreflightError(f'release-preflight stage={stage} status={status} class={reason}')


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, new_url):
        return None


class HTTPSGet:
    """Verified TLS, no redirects, bounded bodies, and GET requests only."""
    def __init__(self):
        self.opener = urllib.request.build_opener(
            NoRedirect(), urllib.request.HTTPSHandler(context=ssl.create_default_context()))

    def __call__(self, stage, url, headers):
        parsed = urllib.parse.urlsplit(url)
        if (parsed.scheme != 'https' or parsed.netloc not in ('api.github.com', 'ghcr.io')
                or parsed.fragment or parsed.username or parsed.password):
            fail(stage, 'none', 'unsafe-endpoint')
        request = urllib.request.Request(url, headers=headers, method='GET')
        try:
            response = self.opener.open(request, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        try:
            if response.geturl() != url:
                fail(stage, 'none', 'redirect')
            body = response.read(MAX_RESPONSE_BYTES + 1)
            if len(body) > MAX_RESPONSE_BYTES:
                fail(stage, response.status, 'oversized-response')
            return response.status, dict(response.headers.items()), body
        finally:
            response.close()


def request(http, stage, url, headers):
    try:
        status, response_headers, body = http(stage, url, headers)
    except (OSError, urllib.error.URLError, TimeoutError):
        fail(stage, 'none', 'network')
    if not isinstance(status, int) or not isinstance(body, bytes):
        fail(stage, 'none', 'malformed-response')
    if len(body) > MAX_RESPONSE_BYTES:
        fail(stage, status, 'oversized-response')
    if status not in (200, 404):
        reason = {401: 'unauthorized', 403: 'denied', 429: 'rate-limited'}.get(status, 'unexpected-status')
        fail(stage, status, reason)
    return status, {key.lower(): value for key, value in response_headers.items()}, body


def json_object(stage, status, body):
    try:
        payload = json.loads(body)
    except (ValueError, UnicodeError):
        fail(stage, status, 'malformed-json')
    if not isinstance(payload, dict):
        fail(stage, status, 'malformed-json')
    return payload


def probe_release(http, actor, credential, log=print, repository='Buffer379x/puffolio', reference=''):
    if (not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]{0,99}/[A-Za-z0-9][A-Za-z0-9._-]{0,99}', repository)
            or not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', reference)
            or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_-]{0,99}(?:\[bot\])?', actor)
            or not credential or len(credential) > 8192
            or any(ord(char) < 33 or ord(char) > 126 for char in credential)):
        fail('configuration', 'none', 'invalid-input')
    github_headers = {
        'Authorization': 'Bearer ' + credential,
        'Accept': 'application/vnd.github+json',
        'X-GitHub-Api-Version': '2022-11-28',
        'User-Agent': 'Puffolio-release-preflight',
    }
    # Check repository access first: a denied/unavailable repository is never an absent tag.
    stage = 'github-repository'
    status, _, body = request(http, stage, 'https://api.github.com/repos/' + repository, github_headers)
    if status != 200:
        fail(stage, status, 'repository-unavailable')
    full_name = json_object(stage, status, body).get('full_name')
    if not isinstance(full_name, str) or full_name.lower() != repository.lower():
        fail(stage, status, 'repository-mismatch')
    log(f'release-preflight stage={stage} status=200 class=accessible')

    stage = 'github-tag'
    status, _, body = request(http, stage,
                              'https://api.github.com/repos/' + repository + '/git/ref/tags/v' + reference,
                              github_headers)
    payload = json_object(stage, status, body)
    if status == 200:
        fail(stage, status, 'existing-tag')
    if payload.get('message') != 'Not Found':
        fail(stage, status, 'unexpected-error')
    log(f'release-preflight stage={stage} status=404 class=absent')

    # Use the same actor/job credential and writer scope as publication. A pull-only
    # request for a not-yet-created package can be denied even when creation is allowed.
    # Requesting scope is not proof of a grant: token/manifest denials still fail closed.
    stage = 'ghcr-token'
    scope = 'repository:' + repository.lower() + ':pull,push'
    token_url = 'https://ghcr.io/token?' + urllib.parse.urlencode({'service': 'ghcr.io', 'scope': scope})
    basic = base64.b64encode((actor + ':' + credential).encode('ascii')).decode('ascii')
    status, _, body = request(http, stage, token_url, {
        'Authorization': 'Basic ' + basic, 'Accept': 'application/json',
        'User-Agent': 'Puffolio-release-preflight',
    })
    if status != 200:
        fail(stage, status, 'token-unavailable')
    payload = json_object(stage, status, body)
    bearer = payload.get('token', payload.get('access_token'))
    if (not isinstance(bearer, str) or not bearer or len(bearer) > 65536
            or any(ord(char) < 33 or ord(char) > 126 for char in bearer)
            or ('token' in payload and 'access_token' in payload and payload['token'] != payload['access_token'])):
        fail(stage, status, 'malformed-token')
    log(f'release-preflight stage={stage} status=200 class=token-issued')

    stage = 'ghcr-manifest'
    status, headers, body = request(http, stage,
                                   'https://ghcr.io/v2/' + repository.lower() + '/manifests/' + reference,
                                   {'Authorization': 'Bearer ' + bearer, 'Accept': ', '.join(sorted(INDEX_TYPES)),
                                    'User-Agent': 'Puffolio-release-preflight'})
    payload = json_object(stage, status, body)
    if status == 404:
        errors = payload.get('errors')
        if (not isinstance(errors, list) or not errors
                or any(not isinstance(error, dict) or error.get('code') not in ('MANIFEST_UNKNOWN', 'NAME_UNKNOWN')
                       for error in errors)):
            fail(stage, status, 'unexpected-error')
        log(f'release-preflight stage={stage} status=404 class=absent')
        return {'exists': False}
    digest = headers.get('docker-content-digest', '')
    if (not re.fullmatch(r'sha256:[0-9a-f]{64}', digest)
            or digest != 'sha256:' + hashlib.sha256(body).hexdigest()):
        fail(stage, status, 'digest-mismatch')
    if (payload.get('schemaVersion') != 2 or payload.get('mediaType') not in INDEX_TYPES
            or not isinstance(payload.get('manifests'), list) or not payload['manifests']):
        fail(stage, status, 'malformed-index')
    log(f'release-preflight stage={stage} status=200 class=existing')
    return {'exists': True, 'digest': digest}


def main(http=None, environ=None, stdout=None, stderr=None):
    environ = os.environ if environ is None else environ
    stdout = sys.stdout if stdout is None else stdout
    stderr = sys.stderr if stderr is None else stderr
    try:
        result = probe_release(http or HTTPSGet(), environ.get('GITHUB_ACTOR', ''), environ.get('GH_TOKEN', ''),
                               lambda message: print(message, file=stdout),
                               reference=environ.get('RELEASE_VERSION', ''))
        if environ.get('GITHUB_OUTPUT'):
            with Path(environ['GITHUB_OUTPUT']).open('a') as output:
                output.write('exists=' + str(result['exists']).lower() + '\n')
                if result['exists']:
                    output.write('digest=' + result['digest'] + '\n')
        print(json.dumps(result, sort_keys=True), file=stdout)
        return 0
    except PreflightError as error:
        print(str(error), file=stderr)
    except Exception:
        # Unexpected local failures must not render raw exception payloads/URLs either.
        print('release-preflight stage=internal status=none class=unexpected', file=stderr)
    return 1


if __name__ == '__main__':
    raise SystemExit(main())
