/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package cmd

import (
	"fmt"

	config "github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	"github.com/macaroni-os/anise/pkg/subsets"
	"github.com/macaroni-os/anise/pkg/v2/installer"

	"github.com/spf13/cobra"
)

func newConfigCommand(cfg *config.AniseConfig) *cobra.Command {
	var ans = &cobra.Command{
		Use:     "config",
		Short:   "Print config",
		Long:    `Show anise-build configuration`,
		Aliases: []string{"c"},
		Run: func(cmd *cobra.Command, args []string) {
			// Load config protect configs
			installer.LoadConfigProtectConfs(cfg)
			// Load subsets defintions
			subsets.LoadSubsetsDefintions(cfg)
			// Load subsets config
			subsets.LoadSubsetsConfig(cfg)

			data, err := cfg.YAML()
			if err != nil {
				Fatal(err.Error())
			}

			fmt.Println(string(data))
		},
	}

	return ans
}
