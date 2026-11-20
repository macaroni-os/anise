#!/bin/bash

testsourcedir=$(dirname "${BASH_SOURCE[0]}")
source ${testsourcedir}/_common.sh

oneTimeSetUp() {
  export tmpdir="$(mktemp -d)"
}

oneTimeTearDown() {
  rm -rf "$tmpdir"
}

testBuildA() {
  $ANISE_BUILD tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/virtuals"
  genidx=$?
  assertEquals 'genidx successfully' "$genidx" "0"

  mkdir $tmpdir/testbuild1
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/virtuals"  --debug --compression "gzip" --destination $tmpdir/testbuild1 test/a
  buildst=$?
  assertEquals 'builds successfully' "$buildst" "0"
  assertTrue 'create package A 1.99' "[ -e '$tmpdir/testbuild1/a-test-1.99.package.tar.gz' ]"
  assertTrue 'create package A 1.99' "[ -e '$tmpdir/testbuild1/a-test-1.99.metadata.yaml' ]"
}

testBuildWithVirtualDependency() {
  mkdir $tmpdir/testbuild2
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/virtuals"  --debug --compression "gzip" --destination $tmpdir/testbuild2 test/b
  buildst=$?
  assertEquals 'build with a virtual dependency succeeds' "$buildst" "0"
  assertTrue 'create virtual dependency A' "[ -e '$tmpdir/testbuild2/a-test-1.99.package.tar.gz' ]"
  assertTrue 'create package B' "[ -e '$tmpdir/testbuild2/b-test-1.0.package.tar.gz' ]"
  assertTrue 'create metadata for package B' "[ -e '$tmpdir/testbuild2/b-test-1.0.metadata.yaml' ]"
}

testBuildC() {
  mkdir $tmpdir/testbuild3
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/virtuals"  --debug --compression "gzip" --destination $tmpdir/testbuild3 test/c
  buildst=$?
  assertEquals 'builds of C expected to fail. Steps with no source image' "$buildst" "1"
}

testBuildImage() {
  mkdir $tmpdir/testbuild4
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/virtuals"  --debug --compression "gzip"  --destination $tmpdir/testbuild4 test/image
  buildst=$?
  assertEquals 'builds of test/image expected to succeed' "$buildst" "0"
  assertTrue 'create package test/image 1.0' "[ -e '$tmpdir/testbuild4/image-test-1.0.package.tar.gz' ]"
  assertTrue 'create package test/image 1.0' "[ -e '$tmpdir/testbuild4/image-test-1.0.metadata.yaml' ]"
}

testBuildVirtual() {
  mkdir $tmpdir/testbuild5
  $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/virtuals"  --debug --compression "gzip" --destination $tmpdir/testbuild5 test/virtual
  buildst=$?
  assertEquals 'builds of test/virtual expected to succeed' "$buildst" "0"
  assertTrue 'create package test/image 1.0' "[ -e '$tmpdir/testbuild5/image-test-1.0.package.tar.gz' ]"
  assertTrue 'create package test/image 1.0' "[ -e '$tmpdir/testbuild5/image-test-1.0.metadata.yaml' ]"
  assertTrue 'create package test/virtual 1.0' "[ -e '$tmpdir/testbuild5/virtual-test-1.0.package.tar.gz' ]"
  assertTrue 'create package test/virtual 1.0' "[ -e '$tmpdir/testbuild5/virtual-test-1.0.metadata.yaml' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

