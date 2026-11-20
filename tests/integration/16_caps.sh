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
  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/caps"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testbuild
  $ANISE_BUILD build -d --tree "$ROOT_DIR/tests/fixtures/caps" --same-owner=true --destination $tmpdir/testbuild --compression gzip test/caps test/caps2
  buildst=$?
  assertTrue 'create package caps 0.1' "[ -e '$tmpdir/testbuild/caps-test-0.1.package.tar.gz' ]"
  assertEquals 'builds successfully' "$buildst" "0"
}

testRepo() {
  assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
  $ANISE_BUILD create-repo --tree "$ROOT_DIR/tests/fixtures/caps" \
  --output $tmpdir/testbuild \
  --packages $tmpdir/testbuild \
  --name "test" \
  --descr "Test Repo" \
  --urls $tmpdir/testrootfs \
  --type http

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
    $ANISE install --sync-repos -y --config $tmpdir/anise.yaml test/caps@0.1 test/caps2@0.1
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
   
    assertTrue 'package installed file1' "[ -e '$tmpdir/testrootfs/file1' ]"
    assertTrue 'package installed file2' "[ -e '$tmpdir/testrootfs/file2' ]"

    assertContains 'caps' "$(getcap $tmpdir/testrootfs/file1)" "cap_net_raw=ep"
    assertContains 'caps' "$(getcap $tmpdir/testrootfs/file2)" "cap_net_raw=ep"
}


# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

