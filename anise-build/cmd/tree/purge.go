/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd_tree

import (
	"fmt"
	"os"

	cfg "github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	"github.com/macaroni-os/anise/pkg/v2/tree"

	"github.com/spf13/cobra"
)

func NewTreePurgeIdx(config *cfg.AniseConfig) *cobra.Command {

	var ans = &cobra.Command{
		Use:   "purgeidx [OPTIONS]",
		Short: "Purge tree indexes.",
		Args:  cobra.OnlyValidArgs,
		Run: func(cmd *cobra.Command, args []string) {
			treePaths, _ := cmd.Flags().GetStringArray("tree")
			onlyUpperLevel, _ := cmd.Flags().GetBool("only-upper-level")

			for _, t := range treePaths {
				ti := tree.NewTreeIdx(t, false).DetectMode()
				err := ti.PurgeIndex(!onlyUpperLevel)
				if err != nil {
					fmt.Println("Error on purge indexes: " + err.Error())
					os.Exit(1)
				}
			}

		},
	}

	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}

	flags := ans.Flags()
	flags.Bool("only-upper-level", false,
		"Purge index only the upper level index (true) or in all directories.")
	flags.StringArrayP("tree", "t", []string{path},
		"Path of the tree to use.")

	return ans
}
