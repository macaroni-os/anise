/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	config "github.com/macaroni-os/anise/pkg/config"
	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"
	installer "github.com/macaroni-os/anise/pkg/v2/installer"
	wagon "github.com/macaroni-os/anise/pkg/v2/repository"

	. "github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

func newMigrateLuetCommand(cfg *config.AniseConfig) *cobra.Command {
	var ans = &cobra.Command{
		Hidden: false,
		Use:    "migrate-luet",
		Short:  "Migrate old /var/cache/luet directory to /var/cache/anise/",
		Long:   `Migrate luet database to anise database.`,
		Run: func(cmd *cobra.Command, args []string) {

			oldCachedir := "/var/cache/luet"
			newCachedir := "/var/cache/anise"
			if !fileHelper.Exists(oldCachedir) {
				fmt.Println("Directory " + oldCachedir + " not present. Nothing to do.")
				return
			}

			dir2rm := true
			if !fileHelper.Exists(newCachedir) {
				err := os.Rename(oldCachedir, newCachedir)
				if err != nil {
					Fatal(fmt.Sprintf(
						"error on rename directory %s to %s: %s",
						oldCachedir, newCachedir, err.Error()))
				}
				dir2rm = false
			}

			oldDbpath := "/var/cache/anise/luet.db"
			newDbpath := "/var/cache/anise/anise.db"
			if fileHelper.Exists(newDbpath) {
				fmt.Println("Database " + newDbpath + " already present." +
					" Remove it if you want force the migration.")
				return
			}

			if dir2rm {
				oldDbpath = "/var/cache/luet/luet.db"
			}

			if fileHelper.Exists(oldDbpath) {
				err := os.Rename(oldDbpath, newDbpath)
				if err != nil {
					Fatal(fmt.Sprintf(
						"error on rename database %s to %s: %s",
						oldDbpath, newDbpath, err.Error()))
				}
			} else {
				fmt.Println("Database " + oldDbpath + " not present." +
					" Nothing to do.")
				return
			}

			// Search installed packages related to "repository"
			// category in order to rename the files list

			if dir2rm {
				err := os.RemoveAll(oldCachedir)
				if err != nil {
					Fatal(fmt.Sprintf(
						"error on remove directory %s: %s",
						oldCachedir, err.Error()))
				}
			}

			// Check if /etc/luet is a link to /etc/anise
			luetLink := fileHelper.ExistsLink("/etc/luet")
			if luetLink {
				err := os.RemoveAll("/etc/luet")
				if err != nil {
					Fatal(fmt.Sprintf(
						"error on remove link /etc/luet: %s",
						err.Error()))
				}

				// Try to remove also /etc/anise/luet.yaml link
				os.RemoveAll("/etc/anise/luet.yaml")
			}

			err := updateRepositoriesPkgs(cfg)
			if err != nil {
				Fatal(fmt.Sprintf(
					"error on update installed repositories packages: %s",
					err.Error()))
			}

			// Rewrite paths in the config file.
			if fileHelper.Exists("/etc/anise/anise.yaml") {
				err = rewriteConfigPaths("/etc/anise/anise.yaml")
				if err != nil {
					Fatal(fmt.Sprintf(
						"error on update file /etc/anise/anise.yaml: %s",
						err.Error()))
				}
			} else if fileHelper.Exists("/etc/anise/luet.yaml") {
				err = rewriteConfigPaths("/etc/anise/luet.yaml")
				if err != nil {
					Fatal(fmt.Sprintf(
						"error on update file /etc/anise/luet.yaml: %s",
						err.Error()))
				}
			}

			InfoC(fmt.Sprintf(":confetti_ball:%s",
				Bold(Blue("All done."))))

		},
	}

	return ans
}

func rewriteConfigPaths(file string) error {

	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	config := strings.ReplaceAll(string(data),
		"etc/luet", "etc/anise")

	err = os.WriteFile(file, []byte(config), 0644)
	if err != nil {
		return err
	}

	return nil
}

func updateRepositoriesPkgs(cfg *config.AniseConfig) error {
	searchOpts := &wagon.StonesSearchOpts{
		Matches:          []string{"."},
		Categories:       []string{"repository"},
		WithFiles:        true,
		AndCondition:     true,
		WithRootfsPrefix: false,
	}

	// Initialize anise artifact manager
	manager := installer.NewArtifactsManager(cfg)
	defer manager.Close()
	manager.Setup()

	manager.Lock()
	defer manager.Unlock()

	searcher := wagon.NewSearcherSimple(cfg)

	stonesref, err := searcher.SearchInstalled(searchOpts)
	searcher.Close()
	if err != nil {
		return err
	}

	for _, stone := range *stonesref {

		prepo := stone.ToPackage()

		InfoC(fmt.Sprintf(":factory: [%s] Update files list...",
			prepo.HumanReadableString()))

		// Delete existing package without remove filesystem files.
		err := manager.Database.RemovePackageFiles(prepo)
		if err != nil {
			return err
		}

		newFilesList := []string{}

		for idx := range stone.Files {

			// Drop rootfs path string
			file := strings.ReplaceAll(stone.Files[idx],
				"etc/luet", "etc/anise")

			newFilesList = append(newFilesList, file)
		}

		// Set package Files
		err = manager.Database.SetPackageFiles(
			&pkg.PackageFile{
				PackageFingerprint: prepo.GetFingerPrint(),
				Files:              newFilesList,
			},
		)
		if err != nil {
			return fmt.Errorf(
				"error on update files list of package %s",
				prepo.HumanReadableString())
		}

	}

	return nil
}
