/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd_mask

import (
	"fmt"
	"os"
	"path/filepath"

	cfg "github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	"github.com/macaroni-os/anise/pkg/v2/repository/mask"

	. "github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

func NewMaskDisableCommand(config *cfg.AniseConfig) *cobra.Command {
	var ans = &cobra.Command{
		Use:     "disable [OPTIONS]",
		Aliases: []string{"d"},
		Short:   "Disable a mask rules file.",
		Long: `Disable a mask rule file.

	$> anise mask disable -f /etc/anise/mask.d/my-mask.yml

	$> anise mask disable -f nodejs '>net-libs/nodejs-24.0'

The filename if not with absolute path is used to write/update the file under the first
directory defined on packages_maskdir option.
`,
		Args: cobra.OnlyValidArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			filename, _ := cmd.Flags().GetString("file")
			if filename == "" {
				fmt.Println("filename mandatory argument not present")
				os.Exit(1)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			rootfs := ""
			mconfdir := ""
			conffile := ""

			filename, _ := cmd.Flags().GetString("file")

			maskManager := mask.NewPackagesMaskManager(config)
			err = maskManager.LoadAllFiles(false)
			if err != nil {
				Fatal(err)
			}

			// Respect the rootfs param on read repositories
			if !config.ConfigFromHost {
				rootfs, err = config.GetSystem().GetRootFsAbs()
				if err != nil {
					Fatal("Error on read rootfs config: ", err.Error())
				}
			}

			if filepath.IsAbs(filename) && filepath.Ext(filename) != "" {
				conffile = filepath.Join(rootfs, filename)
			} else {
				if len(config.PackagesMaskDir) > 0 {
					mconfdir = config.PackagesMaskDir[0]
					conffile = filepath.Join(rootfs, mconfdir, filename) + ".yml"
				} else {
					Fatal("No packages mask directories defined")
				}
			}

			pmf := maskManager.GetMaskFile(conffile)
			if pmf == nil {
				Fatal(fmt.Sprintf(
					"No mask file with path %s found.",
					conffile))
			}

			pmf.Enabled = false

			err = pmf.Write()
			if err != nil {
				Fatal(err)
			}

			InfoC(fmt.Sprintf(":confetti_ball:%s",
				Bold(Blue(fmt.Sprintf(
					"Mask file %s disabled.", pmf.File)))))

		},
	}

	ans.Flags().StringP("file", "f", "",
		"Define the filename without extension or as absolute path of the mask file to disable.")

	return ans
}
