#!/bin/bash

testsourcedir=$(dirname "${BASH_SOURCE[0]}")
source ${testsourcedir}/_common.sh

oneTimeSetUp() {
  export tmpdir="${TEST_TMPDIR:-$(mktemp -d)}"
}

oneTimeTearDown() {
  if [ -z "${SKIP_CLEAN}" ] ; then
    rm -rf "$tmpdir"
  fi
}

testBuild() {
  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/config_protect"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testrootfs/testbuild -p
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/config_protect" \
    --destination $tmpdir/testrootfs/testbuild --compression gzip test/a
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create package' "[ -e '$tmpdir/testrootfs/testbuild/a-test-1.0.package.tar.gz' ]"
}

testRepo() {
  assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
  $ANISE_BUILD create-repo --tree "$ROOT_DIR/tests/fixtures/config_protect" \
  --output $tmpdir/testrootfs/testbuild \
  --packages $tmpdir/testrootfs/testbuild \
  --name "test" \
  --descr "Test Repo" \
  --urls $tmpdir/testrootfs \
  --type disk > ${OUTPUT}

  createst=$?
  assertEquals 'create repo successfully' "$createst" "0"
  assertTrue 'create repository' "[ -e '$tmpdir/testrootfs/testbuild/repository.yaml' ]"
}

testConfig() {

    mkdir $tmpdir/testrootfs/etc/anise/config.protect.d -p

    cat <<EOF > $tmpdir/testrootfs/etc/anise/config.protect.d/conf1.yml
name: "protect1"
dirs:
- /etc/
EOF

    cat <<EOF > $tmpdir/anise.yaml
general:
  debug: true
system:
  rootfs: $tmpdir/testrootfs
  database_path: "/var/cache/anise"
  database_engine: "boltdb"
config_protect_confdir:
    - /etc/anise/config.protect.d
config_from_host: false
repositories:
   - name: "main"
     type: "disk"
     enable: true
     cached: true
     urls:
       - "/testbuild"
EOF
    $ANISE config --config $tmpdir/anise.yaml
    res=$?
    assertEquals 'config test successfully' "$res" "0"
}



testInstall() {

    # Simulate previous installation
    mkdir $tmpdir/testrootfs/etc/a -p
    echo config > $tmpdir/testrootfs/etc/a/conf

    $ANISE install --sync-repos -y --config $tmpdir/anise.yaml test/a
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"


    # Simulate config protect
    assertTrue 'package A installed' "[ -e '$tmpdir/testrootfs/c' ]"
    assertTrue 'config protect not created, file is the same' "[ ! -e '$tmpdir/testrootfs/etc/a/._cfg0001_conf' ]"
    assertEquals 'config protect content' "$(cat $tmpdir/testrootfs/etc/a/conf)" "config"
}


testUnInstall() {
    $ANISE uninstall -y --keep-protected-files --config $tmpdir/anise.yaml test/a
    installst=$?
    assertEquals 'uninstall test successfully' "$installst" "0"
    assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/c' ]"
    assertTrue 'config protect maintains the protected files' "[ -e '$tmpdir/testrootfs/etc/a/conf' ]"
}


testCleanup() {
    $ANISE cleanup --config $tmpdir/anise.yaml --purge-repos
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed' "[ ! -e '$tmpdir/testrootfs/packages/a-test-1.0.package.tar.gz' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

