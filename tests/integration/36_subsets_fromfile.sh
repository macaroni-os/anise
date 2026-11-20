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
  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/subsets"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testbuild
  $ANISE_BUILD build \
    --config $tmpdir/anise-build.yaml \
    --tree "$ROOT_DIR/tests/fixtures/subsets" \
    --destination $tmpdir/testbuild --compression zstd subset/a
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create package' "[ -e '$tmpdir/testbuild/a-subset-1.0.package.tar.zst' ]"
}

testRepo() {
  assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
  $ANISE_BUILD create-repo \
    --config $tmpdir/anise-build.yaml \
    --tree "$ROOT_DIR/tests/fixtures/subsets" \
    --output $tmpdir/testbuild \
    --packages $tmpdir/testbuild \
    --name "test" \
    --descr "Test Repo" \
    --urls $tmpdir/testrootfs \
    --type disk
  createst=$?
  assertEquals 'create repo successfully' "$createst" "0"
  assertTrue 'create repository' "[ -e '$tmpdir/testbuild/repository.yaml' ]"
}

testConfig() {
    mkdir $tmpdir/testrootfs

    mkdir $tmpdir/config.protect.d
    mkdir $tmpdir/etc/anise/subsets.conf.d -p
    mkdir $tmpdir/etc/anise/subsets.def.d -p

    cat <<EOF > $tmpdir/config.protect.d/conf1.yml
name: "protect1"
dirs:
- /etc/
EOF

    cat <<EOF > $tmpdir/etc/anise/subsets.def.d/00-testdata.yml
subsets_def:
  test-data:
    description: "Local subset"
    name: "test-data"
    rules:
    - ^/opt/data
    categories:
    - subset
EOF

    cat <<EOF > $tmpdir/anise.yaml
general:
  debug: true
system:
  rootfs: $tmpdir/testrootfs
  database_path: "/"
  database_engine: "boltdb"
config_protect_confdir:
    - $tmpdir/config.protect.d
config_from_host: true
subsets_confdir:
  - "$tmpdir/etc/anise/subsets.conf.d"
subsets_defdir:
  - "$tmpdir/etc/anise/subsets.def.d"
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
  $ANISE install --sync-repos -y --config $tmpdir/anise.yaml subset/a
  installst=$?
  assertEquals 'install test successfully' "$installst" "0"

   assertTrue 'package A /c installed' "[ -e '$tmpdir/testrootfs/c' ]"
   assertTrue 'package A /cd installed' "[ -e '$tmpdir/testrootfs/cd' ]"
   assertTrue 'package A /usr/include/file.h not installed' \
     "[ ! -e '$tmpdir/testrootfs/usr/include/file.h' ]"

   assertTrue 'package A /opt/data/file not installed' \
     "[ ! -e '$tmpdir/testrootfs/opt/data/file' ]"
}


testUnInstall() {
  $ANISE uninstall -y --config $tmpdir/anise.yaml subset/a
  installst=$?
  assertEquals 'uninstall test successfully' "$installst" "0"
  assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/c' ]"
  assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/cd' ]"
}

testInstall2() {
    cat <<EOF > $tmpdir/anise.yaml
general:
  debug: true
system:
  rootfs: $tmpdir/testrootfs
  database_path: "/"
  database_engine: "boltdb"
config_protect_confdir:
    - $tmpdir/config.protect.d
config_from_host: true
repos_confdir:
  - "$tmpdir/etc/anise/repos.conf.d"
subsets:
  enabled:
    - devel
    - test-data
repositories:
   - name: "main"
     type: "disk"
     enable: true
     cached: true
     urls:
       - "$tmpdir/testbuild"
EOF
  $ANISE config --config $tmpdir/anise.yaml

  ANISE_LOGGING__PARANOID="true" $ANISE install --sync-repos -y --config $tmpdir/anise.yaml subset/a
  installst=$?
  assertEquals 'install test successfully' "$installst" "0"

  assertTrue 'package A /c installed' "[ -e '$tmpdir/testrootfs/c' ]"
  assertTrue 'package A /cd installed' "[ -e '$tmpdir/testrootfs/cd' ]"
  assertTrue 'package A /usr/include/file.h installed' \
    "[ -e '$tmpdir/testrootfs/usr/include/file.h' ]"
   assertTrue 'package A /opt/data/file installed' \
     "[ -e '$tmpdir/testrootfs/opt/data/file' ]"

}

testCleanup() {
  $ANISE cleanup --config $tmpdir/anise.yaml
  installst=$?
  assertEquals 'install test successfully' "$installst" "0"
  assertTrue 'package installed' "[ ! -e '$tmpdir/testrootfs/packages/a-subset-1.0.package.tar.gz' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

