#!/usr/bin/env bash
# Run only against an owned disposable image/volume. Never loads user data.
set -euo pipefail
image=${1:?Usage: container-smoke.sh IMAGE [PLATFORM]}
platform=${2:-linux/amd64}
qa_dir=$(mktemp -d)
container="puffolio-smoke-${RANDOM}-${RANDOM}"
containers=()
cleanup() {
  for name in "${containers[@]}"; do docker rm -f "$name" >/dev/null 2>&1 || true; done
  sudo rm -rf "$qa_dir"
}
trap cleanup EXIT
mkdir -p "$qa_dir/data" "$qa_dir/restored" "$qa_dir/fixtures"
sudo python3 docs/scripts/startup-fixtures.py prepare "$qa_dir/fixtures"
sudo chown -R 99:100 "$qa_dir"
sudo chmod 700 "$qa_dir" "$qa_dir/data" "$qa_dir/restored"
sudo chmod 500 "$qa_dir/fixtures/denied"
runtime=(--platform "$platform" --user 99:100 --read-only --cap-drop ALL
  --security-opt no-new-privileges --cpus 2 --memory 512m --memory-swap 512m
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=32m,mode=1777)
wait_healthy() {
  for attempt in $(seq 1 60); do
    if docker exec "$container" /puffolio healthcheck >/dev/null 2>&1; then return; fi
    sleep 1
  done
  docker logs "$container"
  return 1
}
start() {
  containers+=("$container")
  docker run --detach --name "$container" "${runtime[@]}" \
    --publish 127.0.0.1:8788:8080 \
    --env PUFFOLIO_BASE_URL=http://127.0.0.1:8788 \
    --mount "type=bind,src=$1,dst=/data" "$image" >/dev/null
  wait_healthy
}
stop_cleanly() {
  docker stop --timeout 20 "$container" >/dev/null
  test "$(docker inspect --format '{{.State.ExitCode}}' "$container")" = 0
  docker rm "$container" >/dev/null
}
expect_startup_failure() {
  local directory=$1
  local reason=$2
  local negative="${container}-negative-${RANDOM}"
  containers+=("$negative")
  docker run --detach --name "$negative" "${runtime[@]}" \
    --mount "type=bind,src=$directory,dst=/data" "$image" >/dev/null
  for attempt in $(seq 1 20); do
    if test "$(docker inspect --format '{{.State.Running}}' "$negative")" = false; then
      test "$(docker inspect --format '{{.State.ExitCode}}' "$negative")" != 0
      # Require the intended rejection, not an unrelated startup crash.
      docker logs "$negative" 2>&1 | grep -F "$reason" >/dev/null
      docker rm "$negative" >/dev/null
      return
    fi
    sleep 1
  done
  echo "Expected startup failure did not occur: $reason" >&2
  return 1
}
start "$qa_dir/data"
sudo python3 docs/scripts/smoke.py --url http://127.0.0.1:8788 \
  --setup-code-file "$qa_dir/data/setup-code" --backup-file "$qa_dir/backup.zip" --state-file "$qa_dir/state.json"
sudo python3 docs/scripts/image-memory-smoke.py --url http://127.0.0.1:8788 \
  --state-file "$qa_dir/state.json"
docker restart --timeout 20 "$container" >/dev/null
wait_healthy
sudo python3 docs/scripts/smoke.py --url http://127.0.0.1:8788 --verify-restart "$qa_dir/state.json"
stop_cleanly
sudo chown 99:100 "$qa_dir/backup.zip"
restore_container="${container}-restore"
containers+=("$restore_container")
docker run --rm --name "$restore_container" "${runtime[@]}" \
  --mount "type=bind,src=$qa_dir,dst=/restore" "$image" restore \
  --from /restore/backup.zip --data-dir /restore/restored
start "$qa_dir/restored"
sudo python3 docs/scripts/smoke.py --url http://127.0.0.1:8788 --verify-state "$qa_dir/state.json"
stop_cleanly
# There is no previous public Puffolio release. These are synthetic migration/startup
# fixtures, not a claim of a real earlier-version upgrade or binary rollback.
start "$qa_dir/fixtures/initial"
stop_cleanly
start "$qa_dir/fixtures/current"
stop_cleanly
expect_startup_failure "$qa_dir/fixtures/denied" 'not writable'
expect_startup_failure "$qa_dir/fixtures/newer" 'newer than supported'
sudo python3 docs/scripts/startup-fixtures.py check "$qa_dir/fixtures"
