/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd_query

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	cfg "github.com/macaroni-os/anise/pkg/config"
	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
)

func TestQueryFilesSubsets(t *testing.T) {
	const metadata = `runtime:
  category: app-test
  name: example
  version: "1.0"
  annotations:
    subsets:
      rules:
        devel: ['^/usr/include/']
files:
  - usr/bin/example
  - usr/include/example.h
  - opt/devel/example
`
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	const repoDir = "var/db/anise/repos/test/"
	const versionDir = repoDir + "treefs/app-test/example/1.0/"
	write(repoDir+"repository.yaml", "name: test\n")
	write(versionDir+"definition.yaml", "category: app-test\nname: example\nversion: '1.0'\n")
	write(versionDir+"metadata.yaml", metadata)
	write("etc/anise/subsets.def.d/override.yaml", `subsets_def:
  devel:
    name: devel
    packages: [app-test/example]
    rules: ['^/opt/devel/']
`)
	write("etc/anise/subsets.conf.d/enabled.yaml", "enabled: [devel]\n")

	pack, err := artifact.NewPackageArtifactFromYaml([]byte(metadata))
	if err != nil {
		t.Fatal(err)
	}
	db := pkg.NewBoltDatabase(filepath.Join(root, "var/db/anise/anise.db"))
	if _, err := db.CreatePackage(pack.Runtime); err != nil {
		t.Fatal(err)
	}
	if err := db.SetPackageFiles(&pkg.PackageFile{PackageFingerprint: pack.Runtime.GetFingerPrint(), Files: pack.Files}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	for _, tc := range []struct {
		name      string
		flags     []string
		local     bool
		enabled   bool
		wantPaths []string
	}{
		{"default lists all files", []string{"--with-rootfs-prefix=false"}, true, false, pack.Files},
		{"artifact rules", []string{"--subsets", "--with-rootfs-prefix=false"}, false, false,
			[]string{"usr/bin/example", "opt/devel/example"}},
		{"definition directory overrides artifact", []string{"--subsets"}, true, false,
			[]string{filepath.Join(root, "usr/bin/example"), filepath.Join(root, "usr/include/example.h")}},
		{"configuration directory enables subset", []string{"--subsets", "--with-rootfs-prefix=false"}, true, true, pack.Files},
		{"installed package with rootfs prefix", []string{"--subsets", "--installed"}, true, false,
			[]string{filepath.Join(root, "usr/bin/example"), filepath.Join(root, "usr/include/example.h")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := cfg.NewAniseConfig(nil)
			previous := cfg.AniseCfg
			cfg.AniseCfg = config
			t.Cleanup(func() { cfg.AniseCfg = previous })
			config.Viper.Set("system.rootfs", root)
			config.Viper.Set("system.database_path", "/var/db/anise")
			config.Viper.Set("system.database_engine", "boltdb")
			config.GetGeneral().Concurrency = 1
			config.AddSystemRepository(&cfg.AniseRepository{Name: "test", Enable: true, Cached: true})
			if tc.local {
				config.SubsetsDefDir = []string{"/etc/anise/subsets.def.d"}
			}
			if tc.enabled {
				config.SubsetsConfDir = []string{"/etc/anise/subsets.conf.d"}
			}

			var output bytes.Buffer
			cmd := NewQueryFilesCommand(config)
			cmd.SetOut(&output)
			cmd.SetArgs(append([]string{"app-test/example", "--output", "json"}, tc.flags...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var got []string
			if slices.Contains(tc.flags, "--subsets") {
				var files []queryFileWithSubsets
				if err := json.Unmarshal(output.Bytes(), &files); err != nil {
					t.Fatalf("invalid JSON output %q: %v", output.String(), err)
				}
				for _, file := range files {
					got = append(got, file.Path)
					wantSubsets := []string{}
					if tc.enabled && file.Path == "opt/devel/example" {
						wantSubsets = []string{"devel"}
					}
					if file.Subsets == nil || !slices.Equal(file.Subsets, wantSubsets) {
						t.Errorf("subsets for %q = %#v, want %#v", file.Path, file.Subsets, wantSubsets)
					}
				}
			} else if err := json.Unmarshal(output.Bytes(), &got); err != nil {
				t.Fatalf("invalid JSON output %q: %v", output.String(), err)
			}
			if !slices.Equal(got, tc.wantPaths) {
				t.Errorf("files = %q, want %q", got, tc.wantPaths)
			}
		})
	}
}
