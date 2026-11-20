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
  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/annotations"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testbuild
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/annotations" --destination $tmpdir/testbuild --compression gzip test/pkgA > ${OUTPUT}
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create package A' "[ -e '$tmpdir/testbuild/pkgA-test-0.1.package.tar.gz' ]"
}

testRepo() {
  assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
  $ANISE_BUILD create-repo --tree "$ROOT_DIR/tests/fixtures/annotations" \
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
    $ANISE install --sync-repos -y --config $tmpdir/anise.yaml test/pkgA
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package A installed' "[ -e '$tmpdir/testrootfs/file1' ]"
}

testAnnotations() {
    local output
    output=$("$ANISE" search --config "$tmpdir/anise.yaml" --installed --annotation label1 --output json)
    assertEquals 'search annotations' 0 "$?"
    assertContains 'package found by annotation' "$output" 'pkgA'
    assertContains 'first annotation persisted' "$output" 'value1'
    assertContains 'second annotation persisted' "$output" 'value2'
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

