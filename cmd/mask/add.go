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

func NewMaskAddCommand(config *cfg.AniseConfig) *cobra.Command {
	var ans = &cobra.Command{
		Use:     "add [OPTIONS] <pkgstr1> ... <pkgstrN>",
		Aliases: []string{"a"},
		Short:   "Add one or more mask rules.",
		Long: `Add one or more mask rules:

	$> anise mask add '>net-libs/nodejs-24.0'

	$> anise mask add -f /etc/anise/mask.d/my-mask.yml '>net-libs/nodejs-24.0'

	$> anise mask add -f nodejs '>net-libs/nodejs-24.0'

The filename if not with absolute path is used to write/update the file under the first
directory defined on packages_maskdir option (for example /etc/anise/mask.d/nodejs.yml else main.yml is used).
`,
		Args: cobra.OnlyValidArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				fmt.Println("No package string available")
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

			fname := "main.yml"
			if filename != "" {
				fname = filename + ".yml"
			}

			if filepath.IsAbs(filename) && filepath.Ext(filename) != "" {
				conffile = filepath.Join(rootfs, filename)
			} else {
				if len(config.PackagesMaskDir) > 0 {
					mconfdir = config.PackagesMaskDir[0]
					conffile = filepath.Join(rootfs, mconfdir, fname)
				} else {
					mconfdir = filepath.Join(rootfs, "/etc/anise/mask.d/")
					err = os.MkdirAll(mconfdir, 0755)
					if err != nil {
						Fatal(fmt.Sprintf("error on create directory %s: %s",
							mconfdir, err.Error()))
					}
					conffile = filepath.Join(mconfdir, fname)
				}
			}

			pmf := maskManager.GetMaskFile(conffile)
			if pmf == nil {
				pmf = mask.NewPackageMaskFile(conffile)
				pmf.Description = "Mask file generated with anise"
			}

			for _, pkgstr := range args {
				err = pmf.AddRule(pkgstr)
				if err != nil {
					Fatal(err)
				}
			}

			err = pmf.Write()
			if err != nil {
				Fatal(err)
			}

			InfoC(fmt.Sprintf(":confetti_ball:%s", Bold(Blue("Mask rules added."))))

		},
	}

	ans.Flags().StringP("file", "f", "",
		"Define the filename without extension or as absolute path where add mask rules.")

	return ans
}
