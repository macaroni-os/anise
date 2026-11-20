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
  [ "$ANISE_BACKEND" == "img" ] && startSkipping

  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/perms"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testbuild
  $ANISE_BUILD build -d --tree "$ROOT_DIR/tests/fixtures/perms" --same-owner=true --destination $tmpdir/testbuild --compression gzip test/perms test/perms2
  buildst=$?
  assertTrue 'create package perms 0.1' "[ -e '$tmpdir/testbuild/perms-test-0.1.package.tar.gz' ]"
  assertEquals 'builds successfully' "$buildst" "0"
}

testRepo() {
  [ "$ANISE_BACKEND" == "img" ] && startSkipping
  assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
  $ANISE_BUILD create-repo --tree "$ROOT_DIR/tests/fixtures/perms" \
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
    [ "$ANISE_BACKEND" == "img" ] && startSkipping
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
    [ "$ANISE_BACKEND" == "img" ] && startSkipping
    $ANISE install --sync-repos -y --config $tmpdir/anise.yaml test/perms@0.1 test/perms2@0.1
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
   
    assertTrue 'package installed perms baz' "[ -d '$tmpdir/testrootfs/foo/baz' ]"
    assertTrue 'package installed perms bar' "[ -d '$tmpdir/testrootfs/foo/bar' ]"

    assertContains 'perms1' "$(stat -c %u:%g $tmpdir/testrootfs/foo/baz)" "100:100"
    assertContains 'perms2' "$(stat -c %u:%g $tmpdir/testrootfs/foo/bar)" "100:100"
    assertContains 'perms11' "$(stat -c %u:%g $tmpdir/testrootfs/foo/baz/.keep)" "101:101"
    assertContains 'perms22' "$(stat -c %u:%g $tmpdir/testrootfs/foo/bar/.keep)" "101:101"
}


# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

