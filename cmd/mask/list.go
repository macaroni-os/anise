/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd_mask

import (
	"fmt"

	cfg "github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	"github.com/macaroni-os/anise/pkg/v2/repository/mask"

	"github.com/spf13/cobra"
)

func NewMaskListCommand(config *cfg.AniseConfig) *cobra.Command {
	var ans = &cobra.Command{
		Use:     "list [OPTIONS]",
		Short:   "Show mask rules.",
		Args:    cobra.OnlyValidArgs,
		Aliases: []string{"li"},
		Run: func(cmd *cobra.Command, args []string) {
			enabled, _ := cmd.Flags().GetBool("enabled")

			maskManager := mask.NewPackagesMaskManager(config)
			err := maskManager.LoadAllFiles(enabled)
			if err != nil {
				Fatal(err)
			}

			for _, file := range maskManager.Files {
				for _, rule := range file.Rules {
					fmt.Println(rule)
				}
			}

		},
	}

	flags := ans.Flags()
	flags.Bool("enabled", false, "Show only enabled rules.")

	return ans
}
