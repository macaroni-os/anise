#!/bin/bash

testsourcedir=$(dirname "${BASH_SOURCE[0]}")
source ${testsourcedir}/_common.sh

oneTimeSetUp() {
  export tmpdir="$(mktemp -d)"
}

oneTimeTearDown() {
  rm -rf "$tmpdir"
}

testBuild() {
  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/nofileconflicts_upgrade"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testbuild
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/nofileconflicts_upgrade" --destination $tmpdir/testbuild --compression gzip test1/noconflict
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create packages' "[ -e '$tmpdir/testbuild/noconflict-test1-1.0.package.tar.gz' ]"
  assertTrue 'create packages' "[ -e '$tmpdir/testbuild/noconflict-test1-1.1.package.tar.gz' ]"
}

testRepo() {
  assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
  $ANISE_BUILD create-repo --tree "$ROOT_DIR/tests/fixtures/nofileconflicts_upgrade" \
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
  debug: true
system:
  rootfs: $tmpdir/testrootfs
  database_path: "/"
  database_engine: "boltdb"
config_from_host: true
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
    $ANISE install --sync-repos -y --config $tmpdir/anise.yaml test1/noconflict@1.0
    installst=$?
    #assertTrue 'package installed' "[ -e '$tmpdir/testrootfs/c' ]"
}

testUpgrade() {
    out=$($ANISE upgrade --sync-repos -y --config $tmpdir/anise.yaml)
    installst=$?
    assertEquals 'install test succeeded' "$installst" "0"
    assertNotContains 'does find conflicts' "$out" \
      "Error: file conflict found: found file test1 conflict between package"

    installed=$($ANISE --config $tmpdir/anise.yaml search --installed)
    assertContains 'does upgrade' "$installed" "test1/noconflict-1.1"

}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

