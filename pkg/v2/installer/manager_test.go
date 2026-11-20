/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package installer

import (
	"os"
	"path/filepath"
	"testing"

	cfg "github.com/macaroni-os/anise/pkg/config"
	repos "github.com/macaroni-os/anise/pkg/v2/repository"
)

func TestRemovePackageFilesDirectorySymlink(t *testing.T) {
	const linkPath = "usr/lib"
	const targetPath = "usr/a-lib64"

	for _, test := range []struct {
		name       string
		linkTarget string
		pruneOnly  bool
		keepFile   bool
	}{
		{
			name:       "listed link to non-empty directory",
			linkTarget: "a-lib64",
			keepFile:   true,
		},
		{
			name:       "link encountered when pruning parent directories",
			linkTarget: "a-lib64",
			pruneOnly:  true,
			keepFile:   true,
		},
		{
			name:       "absolute link resolved inside the rootfs",
			linkTarget: "/" + targetPath,
			keepFile:   true,
		},
		{
			name:       "target emptied through its real path",
			linkTarget: "a-lib64",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, targetPath)
			owned := filepath.Join(target, "owned.so")
			keep := filepath.Join(target, "keep.so")
			link := filepath.Join(root, linkPath)
			if err := os.MkdirAll(target, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(test.linkTarget, link); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(owned, []byte("package file"), 0644); err != nil {
				t.Fatal(err)
			}
			if test.keepFile {
				if err := os.WriteFile(keep, []byte("other file"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			config := cfg.NewAniseConfig(nil)
			config.GetSystem().Rootfs = root
			config.GetSystem().DatabasePath = "var/db/anise"
			manager := NewArtifactsManager(config)
			files := []string{linkPath, targetPath + "/owned.so"}
			if test.pruneOnly {
				files = []string{linkPath + "/owned.so"}
			}
			stone := &repos.Stone{Name: "example", Category: "app-test", Version: "1", Files: files}
			if err := manager.removePackageFiles(stone, root, false); err != nil {
				t.Fatal(err)
			}

			if test.keepFile {
				if got, err := os.Readlink(link); err != nil || got != test.linkTarget {
					t.Errorf("Readlink(%q) = %q, %v, want %q", link, got, err, test.linkTarget)
				}
			} else if _, err := os.Lstat(link); !os.IsNotExist(err) {
				t.Errorf("expected link to be removed, Lstat returned %v", err)
			}
			if _, err := os.Stat(owned); !os.IsNotExist(err) {
				t.Errorf("expected package file to be removed, Stat returned %v", err)
			}
			if test.keepFile {
				if got, err := os.ReadFile(keep); err != nil || string(got) != "other file" {
					t.Errorf("expected other file to survive, ReadFile returned %q, %v", got, err)
				}
			}
		})
	}
}
