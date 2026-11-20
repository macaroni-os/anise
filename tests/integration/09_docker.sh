#!/bin/bash

testsourcedir=$(dirname "${BASH_SOURCE[0]}")
source ${testsourcedir}/_common.sh

oneTimeSetUp() {
  tmpdir=$(mktemp -d)
  local suffix=${tmpdir##*.}
  image="anise-integration-scratch:$suffix"
  installed_image="$image-installed"
  container="anise-integration-scratch-$suffix"
}

oneTimeTearDown() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  docker rmi "$installed_image" "$image" >/dev/null 2>&1 || true
  rm -rf "$tmpdir"
}

testBuild() {
  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/docker"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testbuild
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/docker" --destination $tmpdir/testbuild --compression gzip seed/alpine > ${OUTPUT}
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create package' "[ -e '$tmpdir/testbuild/alpine-seed-1.0.package.tar.gz' ]"
}

testRepo() {
  assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
  $ANISE_BUILD create-repo --tree "$ROOT_DIR/tests/fixtures/docker" \
  --output $tmpdir/testbuild \
  --packages $tmpdir/testbuild \
  --name "test" \
  --descr "Test Repo" \
  --urls $tmpdir/testrootfs \
  --type disk > ${OUTPUT}

  createst=$?
  assertEquals 'create repo successfully' "$createst" "0"
  assertTrue 'create repository' "[ -e '$tmpdir/testbuild/repository.yaml' ]"
}

testInstallFromScratch() {
  local context="$tmpdir/context"
  mkdir "$context" || return 1
  cp "$(command -v "$ANISE")" "$context/anise" || return 1
  cp -r "$tmpdir/testbuild" "$context/packages" || return 1
  cat > "$context/anise.yaml" <<'YAML'
system:
  rootfs: /
  database_path: /var/cache/anise
config_from_host: true
repos_confdir: []
repositories:
  - name: main
    type: disk
    enable: true
    cached: true
    urls: [/repository]
YAML
  cat > "$context/Dockerfile" <<'DOCKERFILE'
FROM scratch
COPY anise /usr/bin/anise
COPY anise.yaml /etc/anise/anise.yaml
COPY packages /repository
ENTRYPOINT ["/usr/bin/anise"]
DOCKERFILE

  docker build --tag "$image" "$context" > "${OUTPUT}" || { fail 'build scratch image'; return; }
  docker run --name "$container" "$image" install --sync-repos -y seed/alpine > "$context/install.log" 2>&1 ||
    { cat "$context/install.log"; fail 'install Alpine in the scratch image'; return; }
  docker commit "$container" "$installed_image" > "${OUTPUT}" || { fail 'commit installed image'; return; }
  local output
  output=$(docker run --rm --entrypoint /bin/sh "$installed_image" -c 'echo ftw')
  assertEquals 'installed shell runs' 'ftw' "$output"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

