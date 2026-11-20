#!/bin/bash

source "$(dirname "${BASH_SOURCE[0]}")/_common.sh"
set -euo pipefail

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
export ANISE_SYSTEM__TMPDIR_BASE="$tmpdir/work"

tree="$tmpdir/tree"
mkdir -p "$tree/render-values" "$tmpdir/packages"

cat > "$tree/render-values/definition.yaml" <<'EOF'
category: test
name: render-values
version: '1.0'
EOF

cat > "$tree/render-values/build.yaml" <<'EOF'
image: alpine
steps:
  - >-
    printf '%s\n' '{{ .Values.default_only }}' '{{ .Values.values_only }}' '{{ .Values.shared }}'
    > /rendered-values
EOF

cat > "$tmpdir/default-values.yaml" <<'EOF'
default_only: from-defaults
shared: default
EOF

cat > "$tmpdir/values.yaml" <<'EOF'
values_only: from-values
shared: override
EOF

"$ANISE_BUILD" tree genidx --only-upper-level -t "$tree"
"$ANISE_BUILD" build --tree "$tree" --destination "$tmpdir/packages" --compression gzip \
  --default-values "$tmpdir/default-values.yaml" --values "$tmpdir/values.yaml" test/render-values

tar -xOf "$tmpdir/packages/render-values-test-1.0.package.tar.gz" rendered-values > "$tmpdir/rendered-values"
diff -u <(printf '%s\n' from-defaults from-values override) "$tmpdir/rendered-values"
