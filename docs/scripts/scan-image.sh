#!/usr/bin/env bash
# Official pinned Trivy; remote public subject only, no Docker socket or credentials.
set -euo pipefail
image=${1:?Usage: scan-image.sh IMAGE@SHA256 OUTPUT.json linux/ARCH}
output=${2:?JSON output path required}
platform=${3:?Native platform required}
if ! [[ "$image" =~ ^ghcr\.io/buffer379x/puffolio@sha256:[0-9a-f]{64}$ ]]; then exit 2; fi
if ! [[ "$platform" = linux/amd64 || "$platform" = linux/arm64 ]]; then exit 2; fi
output=$(python3 - "$output" <<'PY'
from pathlib import Path
import sys
path=Path(sys.argv[1]).resolve();root=(Path.cwd()/'work').resolve()
assert path.is_relative_to(root) and path.suffix=='.json'
assert ',' not in str(path) and '\n' not in str(path) and not path.exists()
assert not Path(str(path)+'.scanner.json').exists()
path.parent.mkdir(parents=True,exist_ok=True)
print(path)
PY
)
mkdir -p work/cache
cache=$(mktemp -d "$PWD/work/cache/trivy-scan.XXXXXX")
cleanup() { rm -rf "$cache"; }
trap cleanup EXIT
# Fresh DB downloads temporarily keep two compressed copies (~264 MiB observed).
scanner='aquasec/trivy@sha256:790fd66d1697395d8693d1acaa1ea702406a20f109b6fc7bac8db623eca6c06c'
docker run --rm --platform "$platform" --user "$(id -u):$(id -g)" \
  --read-only --cap-drop ALL --security-opt no-new-privileges \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=512m,mode=1777 --workdir /tmp \
  --mount "type=bind,src=$(dirname "$output"),dst=/out" \
  --mount "type=bind,src=$cache,dst=/cache" \
  --env TRIVY_CACHE_DIR=/cache --env DOCKER_CONFIG=/tmp/empty-docker \
  "$scanner" image --image-src remote --platform "$platform" --scanners vuln \
  --format json --output "/out/$(basename "$output")" --exit-code 0 \
  --severity UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL --ignore-unfixed=false \
  --skip-db-update=false --skip-java-db-update=false --offline-scan=false \
  --ignorefile /dev/null "$image"
python3 "$(dirname "$0")/scan-metadata.py" "$output" "$image" "$platform"
