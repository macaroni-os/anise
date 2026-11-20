#!/bin/bash

testsourcedir=$(dirname "${BASH_SOURCE[0]}")
source ${testsourcedir}/_common.sh

oneTimeSetUp() {
export tmpdir="$(mktemp -d)"
  cat <<EOF > $tmpdir/anise-build.yaml
general:
  debug: true
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
  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/upgrade_integration"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testbuild
  $ANISE_BUILD build --config $tmpdir/anise-build.yaml --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" --destination $tmpdir/testbuild --compression gzip =test/b-1.0 > ${OUTPUT}
  buildst=$?
  assertTrue 'create package B 1.0' "[ -e '$tmpdir/testbuild/b-test-1.0.package.tar.gz' ]"
  assertEquals 'builds successfully' "$buildst" "0"

  $ANISE_BUILD build --config $tmpdir/anise-build.yaml --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" --destination $tmpdir/testbuild --compression gzip =test/b-1.1 > ${OUTPUT}
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create package B 1.1' "[ -e '$tmpdir/testbuild/b-test-1.1.package.tar.gz' ]"

  $ANISE_BUILD build --config $tmpdir/anise-build.yaml --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" --destination $tmpdir/testbuild --compression gzip =test/a-1.0 > ${OUTPUT}
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create package A 1.0' "[ -e '$tmpdir/testbuild/a-test-1.0.package.tar.gz' ]"

  $ANISE_BUILD build --config $tmpdir/anise-build.yaml --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" --destination $tmpdir/testbuild --compression gzip =test/a-1.1 > ${OUTPUT}
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"

  assertTrue 'create package A 1.1' "[ -e '$tmpdir/testbuild/a-test-1.1.package.tar.gz' ]"

  $ANISE_BUILD build --config $tmpdir/anise-build.yaml --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" --destination $tmpdir/testbuild --compression gzip =test/a-1.2 > ${OUTPUT}
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"

  assertTrue 'create package A 1.2' "[ -e '$tmpdir/testbuild/a-test-1.2.package.tar.gz' ]"


  $ANISE_BUILD build --config $tmpdir/anise-build.yaml --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" --destination $tmpdir/testbuild --compression gzip =test/c-1.0 > ${OUTPUT}
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create package C 1.0' "[ -e '$tmpdir/testbuild/c-test-1.0.package.tar.gz' ]"

}

testRepo() {
  assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
  $ANISE_BUILD create-repo --config $tmpdir/anise-build.yaml \
    --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" \
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

    $ANISE --config $tmpdir/anise.yaml repo update
}


testSearch() {
    installed=$($ANISE --config $tmpdir/anise.yaml q belongs testaa)
    searchst=$?
    assertEquals 'search exists successfully' "$searchst" "0"
    assertContains 'contains test/a-1.0' "$installed" 'test/a-1.0'
}

testGetSearchLocal() {
    $ANISE install --sync-repos -y --config $tmpdir/anise.yaml test/a@1.0
    assertTrue 'package installed A' "[ -e '$tmpdir/testrootfs/testaa' ]"
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"

    installed=$($ANISE --config $tmpdir/anise.yaml database get --files test/a@1.0)
    searchst=$?
    assertEquals 'search exists successfully' "$searchst" "0"
    assertContains 'contains file' "$installed" 'testaa'

    installed=$($ANISE --config $tmpdir/anise.yaml database get test/a@1.0)
    searchst=$?
    assertEquals 'search exists successfully' "$searchst" "0"
    assertNotContains 'contains file' "$installed" 'testaa'



    installed=$($ANISE --config $tmpdir/anise.yaml q belongs --installed testaa)
    searchst=$?
    assertEquals 'search exists successfully' "$searchst" "0"

    assertContains 'contains test/a-1.1' "$installed" 'test/a-1.0'

    installed=$($ANISE --config $tmpdir/anise.yaml q belongs --installed foo)
    searchst=$?
    assertEquals 'search exists successfully' "$searchst" "0"

    assertNotContains 'contains test/a-1.1' "$installed" 'test/a-1.0'
}


# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

