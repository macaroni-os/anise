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
    "$ANISE_BUILD" tree genidx --only-upper-level -t "$ROOT_DIR/tests/fixtures/copy" || {
        fail 'generate copy fixture index'
        return
    }
    cat <<EOF > $tmpdir/default.yaml
extra: "bar"
foo: "baz"
EOF
    mkdir $tmpdir/testbuild
    $ANISE_BUILD build --tree "$ROOT_DIR/tests/fixtures/copy" \
               --destination $tmpdir/testbuild --concurrency 1 \
               --compression gzip --values $tmpdir/default.yaml \
               test/c
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    [ "$buildst" = 0 ] || return
    assertTrue 'create package c' "[ -e '$tmpdir/testbuild/c-test-1.2.package.tar.gz' ]"
    mkdir $tmpdir/extract
    tar -xvf $tmpdir/testbuild/c-test-1.2.package.tar.gz -C $tmpdir/extract
    assertTrue 'create result in package c' "[ -e '$tmpdir/extract/result' ]"
    assertTrue 'create busybox in package c' "[ -f '$tmpdir/extract/bina/busybox' ]"
}


# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

