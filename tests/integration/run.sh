#!/bin/bash
set -e
export ANISE_NO_SPINNER=true
export ANISE_YES=true
export ROOT_DIR="$(git rev-parse --show-toplevel)"

if [ -z "${SKIP_ANISE_BUILD}" ] ; then
pushd $ROOT_DIR
    CGO_ENABLED=0 go build -o "$ROOT_DIR/tests/integration/bin/anise"
	  CGO_ENABLED=0 go build -o "$ROOT_DIR/tests/integration/bin/anise-build" ./anise-build
popd
fi

[ ! -d "$ROOT_DIR/tests/integration/shunit2" ] && git clone https://github.com/kward/shunit2.git "$ROOT_DIR/tests/integration/shunit2" 

export PATH=$ROOT_DIR/tests/integration/bin/:$PATH

if [ -n "${SINGLE_TEST:-}" ]; then
  scripts=("$ROOT_DIR/tests/integration/$SINGLE_TEST")
else
  shopt -s nullglob
  scripts=("$ROOT_DIR"/tests/integration/[0-9]*_*.sh)
fi

for script in "${scripts[@]}"; do
  echo "Executing test '${script##*/}'."
  "$script"
done
