"""Identity checks required before resuming an existing immutable image."""
import copy
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('release_image', Path(__file__).with_name('release-image.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

SHA = 'a' * 40
DIGEST = 'sha256:' + 'b' * 64
VERSION = '1.2.3'

def fixtures():
    index = {'manifests': [
        {'digest': 'sha256:' + char * 64, 'platform': {'os': 'linux', 'architecture': arch}}
        for arch, char in [('amd64', 'c'), ('arm64', 'd')]
    ] + [{'digest': 'sha256:' + 'e' * 64, 'platform': {'os': 'unknown', 'architecture': 'unknown'},
          'annotations': {'vnd.docker.reference.type': 'attestation-manifest'}}]}
    configs = {arch: {'Os': 'linux', 'Architecture': arch, 'Config': {'Labels': {
        'org.opencontainers.image.source': 'https://github.com/Buffer379x/puffolio',
        'org.opencontainers.image.revision': SHA, 'org.opencontainers.image.version': VERSION,
    }}} for arch in ['amd64', 'arm64']}
    return index, configs

class ImmutableImageIdentity(unittest.TestCase):
    def test_accepts_exact_source_platforms_version_and_digest(self):
        index, configs = fixtures()
        module.validate_identity(index, configs, SHA, DIGEST, DIGEST, VERSION)

    def test_refuses_digest_revision_version_source_or_platform_mismatch(self):
        for mutation in ['digest', 'revision', 'version', 'source', 'missing', 'extra', 'duplicate', 'unknown']:
            with self.subTest(mutation=mutation):
                index, configs = fixtures()
                expected = DIGEST
                if mutation == 'digest': expected = 'sha256:' + 'f' * 64
                elif mutation in ['revision', 'version', 'source']:
                    configs['arm64']['Config']['Labels']['org.opencontainers.image.' + mutation] = 'wrong'
                elif mutation == 'missing': index['manifests'].pop(1)
                elif mutation == 'extra': index['manifests'].append({'digest': DIGEST, 'platform': {'os': 'linux', 'architecture': '386'}})
                elif mutation == 'duplicate': index['manifests'].append(copy.deepcopy(index['manifests'][0]))
                elif mutation == 'unknown': index['manifests'][-1].pop('annotations')
                with self.assertRaises(ValueError):
                    module.validate_identity(index, configs, SHA, DIGEST, expected, VERSION)

    def test_refuses_a_version_that_is_not_exact(self):
        index, configs = fixtures()
        for version in ['latest', '1.2', '1.2.3-rc1', '1.2.3\n', '']:
            with self.subTest(version=version), self.assertRaises(ValueError):
                module.validate_identity(index, configs, SHA, DIGEST, DIGEST, version)

if __name__ == '__main__':
    unittest.main()
