#!/bin/bash
# Description: 

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
config_protect_confdir:
  - "$tmpdir/etc/anise/config.protect.d"
subsets_defdir:
  - "$tmpdir/etc/anise/subsets.conf.d"
EOF
}

oneTimeTearDown() {
    rm -rf "$tmpdir"
}

testBuild() {

  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/buildableseed"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testbuild
  $ANISE_BUILD build --config $tmpdir/anise-build.yaml \
    --tree "$ROOT_DIR/tests/fixtures/buildableseed" \
    --destination $tmpdir/testbuild \
    --compression gzip test/c > ${OUTPUT}
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create package dep B' "[ -e '$tmpdir/testbuild/b-test-1.0.package.tar.gz' ]"
  assertTrue 'create package' "[ -e '$tmpdir/testbuild/c-test-1.0.package.tar.gz' ]"
}

testRepo() {
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    $ANISE_BUILD create-repo --config $tmpdir/anise-build.yaml \
      --tree "$ROOT_DIR/tests/fixtures/buildableseed" \
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
  database_engine: "memory"
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
    $ANISE install -y --sync-repos --config $tmpdir/anise.yaml test/c
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed' "[ -e '$tmpdir/testrootfs/c' ]"
}

testCleanup() {
    $ANISE cleanup --config $tmpdir/anise.yaml
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package cleaned' "[ ! -e '$tmpdir/testrootfs/packages/c-test-1.0.package.tar.gz' ]"
}

testInstall2() {
    $ANISE install --sync-repos -y --config $tmpdir/anise.yaml --system-target $tmpdir/foo test/c
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'db not created' "[ ! -e '$tmpdir/foo/var/cache/anise/anise.db' ]"
    assertTrue 'package installed' "[ -e '$tmpdir/foo/c' ]"
}

testCleanup2() {
    $ANISE cleanup --config $tmpdir/anise.yaml --system-target $tmpdir/foo
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package cleaned' "[ ! -e '$tmpdir/foo/packages/c-test-1.0.package.tar.gz' ]"
}

testInstall3() {
        cat <<EOF > $tmpdir/anise2.yaml
general:
  debug: true
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
    $ANISE install --sync-repos -y --config $tmpdir/anise2.yaml --system-target $tmpdir/baz test/c
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed' "[ -e '$tmpdir/baz/c' ]"
}

testCleanup3() {
    $ANISE cleanup --config $tmpdir/anise2.yaml --system-target $tmpdir/baz
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package cleaned' "[ ! -e '$tmpdir/baz/packages/c-test-1.0.package.tar.gz' ]"
}

testInstall4() {
    $ANISE install --sync-repos -y --config $tmpdir/anise2.yaml --system-target $tmpdir/bad --system-engine boltdb test/c
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed' "[ -e '$tmpdir/bad/c' ]"
    assertTrue 'db created' "[ -d '$tmpdir/bad/var/cache/anise' ]"
}

testCleanup4() {
    $ANISE cleanup --config $tmpdir/anise2.yaml --system-target $tmpdir/bad --system-engine boltdb
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package cleaned' "[ ! -e '$tmpdir/bad/packages/c-test-1.0.package.tar.gz' ]"
}
# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

