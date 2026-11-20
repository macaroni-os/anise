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
  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/shared_templates"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir "$tmpdir/testbuild"
  cat > "$tmpdir/anise-build.yaml" <<EOF
templates-dir:
  - "$ROOT_DIR/tests/fixtures/shared_templates/templates"
EOF
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/shared_templates" --config "$tmpdir/anise-build.yaml" --destination $tmpdir/testbuild --compression gzip test/bar test/foo
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create packages' "[ -e '$tmpdir/testbuild/foo-test-1.0.package.tar.gz' ]"
  assertTrue 'create packages' "[ -e '$tmpdir/testbuild/bar-test-1.0.package.tar.gz' ]"
}

testRepo() {
  assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
  $ANISE_BUILD create-repo --tree "$ROOT_DIR/tests/fixtures/shared_templates" \
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
    $ANISE install --sync-repos -y --config $tmpdir/anise.yaml test/foo test/bar
    installst=$?
    assertEquals 'install test failed' "$installst" "0"
    assertTrue 'package bar installed' "[ -e '$tmpdir/testrootfs/bar' ]"
    assertTrue 'package foo installed' "[ -e '$tmpdir/testrootfs/foo' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

