#!/bin/bash

testsourcedir=$(dirname "${BASH_SOURCE[0]}")
source ${testsourcedir}/_common.sh

oneTimeSetUp() {
  export tmpdir="$(mktemp -d)"
  cat <<EOF > $tmpdir/anise-build.yaml
general:
  debug: false
logging:
  enable_emoji: false
  color: false
system:
  rootfs: $tmpdir/testrootfs
  database_path: "/"
  database_engine: "memory"
config_from_host: true
repos_confdir:
  - "$tmpdir/etc/anise/repos.conf.d"
EOF
}

oneTimeTearDown() {
  rm -rf "$tmpdir"
}

testBuild() {
  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/fileconflicts_upgrade"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testbuild
  $ANISE_BUILD build --config $tmpdir/anise-build.yaml \
    --tree "$ROOT_DIR/tests/fixtures/fileconflicts_upgrade" \
    --destination $tmpdir/testbuild --compression gzip test1/conflict test2/conflict
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create packages' "[ -e '$tmpdir/testbuild/conflict-test1-1.0.package.tar.gz' ]"
  assertTrue 'create packages' "[ -e '$tmpdir/testbuild/conflict-test2-1.0.package.tar.gz' ]"
}

testRepo() {
  assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
  $ANISE_BUILD create-repo --config $tmpdir/anise-build.yaml \
  --tree "$ROOT_DIR/tests/fixtures/fileconflicts_upgrade" \
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

testConfig() {
    mkdir $tmpdir/testrootfs
    cat <<EOF > $tmpdir/anise.yaml
general:
  debug: false
logging:
  enable_emoji: false
  color: false
system:
  rootfs: $tmpdir/testrootfs
  database_path: "/"
  database_engine: "boltdb"
config_from_host: true
repos_confdir:
  - "$tmpdir/etc/anise/repos.conf.d"
repositories:
   - name: "main"
     type: "disk"
     enable: true
     cached: true
     urls:
       - "$tmpdir/testbuild"
EOF
    $ANISE config --config $tmpdir/anise.yaml
    res=$?
    assertEquals 'config test successfully' "$res" "0"
}

testInstall() {
    $ANISE install --sync-repos -y --force --config $tmpdir/anise.yaml test1/conflict@1.0 test2/conflict@1.0
    installst=$?
    assertEquals 'install test succeded' "$installst" "0"
}

testUpgrade() {
    out=$($ANISE upgrade --sync-repos -y --config $tmpdir/anise.yaml)
    installst=$?
    assertEquals 'install test succeeded' "$installst" "1"
    assertContains 'does find conflicts' "$out" \
      "Error: file test1 conflict between package"

    $ANISE upgrade --sync-repos -y --config $tmpdir/anise.yaml --force
    installst=$?
    assertEquals 'install test succeeded' "$installst" "0"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

