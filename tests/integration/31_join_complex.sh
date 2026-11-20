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
    mkdir $tmpdir/testbuild
    $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/join_complex" \
               --destination $tmpdir/testbuild --concurrency 1 \
               --compression gzip \
               test/z test/x
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package z' "[ -e '$tmpdir/testbuild/z-test-0.1.package.tar.gz' ]"
    assertTrue 'create package z' "[ -e '$tmpdir/testbuild/x-test-0.1.package.tar.gz' ]"

    mkdir $tmpdir/extract
    tar -xvf $tmpdir/testbuild/x-test-0.1.package.tar.gz -C $tmpdir/extract
    tar -xvf $tmpdir/testbuild/z-test-0.1.package.tar.gz -C $tmpdir/extract
    assertTrue 'create result from a package that requires a join' "[ -e '$tmpdir/extract/z' ]"
    assertTrue 'create result from join of a join' "[ -e '$tmpdir/extract/x' ]"
}


# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

