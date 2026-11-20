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
  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/simple_dep"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testbuild1
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/simple_dep" --destination $tmpdir/testbuild1 test/c
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create package A 1.2' "[ -e '$tmpdir/testbuild1/a-test-1.2.package.tar' ]"
  assertTrue 'create package C 1.0' "[ -e '$tmpdir/testbuild1/c-test-1.0.package.tar' ]"
}

testBuildOnlyTarget() {
  mkdir $tmpdir/testbuild2
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/simple_dep" --destination $tmpdir/testbuild2 --only-target-package test/c
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create package A 1.2' "[ ! -e '$tmpdir/testbuild2/a-test-1.2.package.tar' ]"
  assertTrue 'create package C 1.0' "[ -e '$tmpdir/testbuild2/c-test-1.0.package.tar' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2
