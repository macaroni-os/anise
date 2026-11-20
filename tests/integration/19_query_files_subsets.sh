#!/bin/bash

source "$(dirname "${BASH_SOURCE[0]}")/_common.sh"
set -euo pipefail

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

rootfs="$tmpdir/rootfs"
repo="$rootfs/var/db/anise/repos/test"
version="$repo/treefs/app-test/example/1.0"
definitions="$rootfs/etc/anise/subsets.def.d"
enabled="$rootfs/etc/anise/subsets.conf.d"
mkdir -p "$version" "$definitions" "$enabled"

cat > "$tmpdir/anise.yaml" <<EOF
logging:
  level: error
system:
  rootfs: "$rootfs"
  database_path: /var/db/anise
  tmpdir_base: "$tmpdir/work"
config_from_host: false
subsets_defdir: [/etc/anise/subsets.def.d]
subsets_confdir: [/etc/anise/subsets.conf.d]
repositories:
  - name: test
    type: disk
    cached: true
    enable: true
EOF

printf 'name: test\n' > "$repo/repository.yaml"
cat > "$version/definition.yaml" <<'EOF'
category: app-test
name: example
version: '1.0'
EOF
cat > "$version/metadata.yaml" <<'EOF'
runtime:
  category: app-test
  name: example
  version: '1.0'
  annotations:
    subsets:
      rules:
        devel: ['^/usr/include/']
files:
  - usr/bin/example
  - usr/include/example.h
  - opt/category/example
  - opt/package/example
EOF

cat > "$definitions/unrelated.yaml" <<'EOF'
subsets_def:
  devel:
    name: devel
    packages: [app-other/example]
    categories: [app-other]
    rules: ['^/usr/bin/']
EOF

check_files() {
  local expected=$1 actual
  shift
  actual=$("$ANISE" --config "$tmpdir/anise.yaml" --no-spinner \
    q files app-test/example "$@")
  diff -u <(printf '%s\n' "$expected") <(printf '%s\n' "$actual")
}

all_files=$'usr/bin/example\nusr/include/example.h\nopt/category/example\nopt/package/example'
check_files "$all_files" --with-rootfs-prefix=false
check_files $'usr/bin/example\nopt/category/example\nopt/package/example' \
  --subsets --with-rootfs-prefix=false

cat > "$definitions/category.yaml" <<'EOF'
subsets_def:
  devel:
    name: devel
    categories: [app-test]
    rules: ['^/opt/category/']
EOF
check_files $'usr/bin/example\nusr/include/example.h\nopt/package/example' \
  --subsets --with-rootfs-prefix=false

cat > "$definitions/package.yaml" <<'EOF'
subsets_def:
  devel:
    name: devel
    packages: [app-test/example]
    rules: ['^/opt/package/']
EOF
check_files "$(printf '%s\n' "$rootfs/usr/bin/example" "$rootfs/usr/include/example.h" "$rootfs/opt/category/example")" \
  --subsets

printf 'enabled: [devel]\n' > "$enabled/local.yaml"
check_files "$all_files" --subsets --with-rootfs-prefix=false
