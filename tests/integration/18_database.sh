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
  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/buildableseed"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testbuild
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/buildableseed" --destination $tmpdir/testbuild --compression gzip test/c > ${OUTPUT}
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create package dep B' "[ -e '$tmpdir/testbuild/b-test-1.0.package.tar.gz' ]"
  assertTrue 'create package' "[ -e '$tmpdir/testbuild/c-test-1.0.package.tar.gz' ]"
}

testRepo() {
  assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
  $ANISE_BUILD create-repo --tree "$ROOT_DIR/tests/fixtures/buildableseed" \
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

testDatabase() {
    $ANISE database create --config $tmpdir/anise.yaml $tmpdir/testbuild/c-test-1.0.metadata.yaml
    #$ANISE install -y --config $tmpdir/anise.yaml test/c-1.0 > /dev/null
    createst=$?
    assertEquals 'created package successfully' "$createst" "0"
    assertTrue 'package not installed' "[ ! -e '$tmpdir/testrootfs/c' ]"

    installed=$($ANISE --config $tmpdir/anise.yaml search --installed .)
    searchst=$?
    assertEquals 'search exists successfully' "$searchst" "0"
    assertContains 'contains test/c-1.0' "$installed" 'test/c-1.0'
    touch $tmpdir/testrootfs/c
    
    $ANISE database remove --config $tmpdir/anise.yaml test/c@1.0
    removetest=$?
    assertEquals 'package removed successfully' "$removetest" "0"
    assertTrue 'file not touched' "[ -e '$tmpdir/testrootfs/c' ]"

    $ANISE database create --config $tmpdir/anise.yaml $tmpdir/testbuild/c-test-1.0.metadata.yaml
    #$ANISE install -y --config $tmpdir/anise.yaml test/c-1.0 > /dev/null
    createst=$?
    assertEquals 'created package successfully' "$createst" "0"
    assertTrue 'file still present' "[ -e '$tmpdir/testrootfs/c' ]"
    
    $ANISE uninstall -y --config $tmpdir/anise.yaml test/c
    installst=$?
    assertEquals 'uninstall test successfully' "$installst" "0"
    assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/c' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

