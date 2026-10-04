/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd

import (
	cmd_mask "github.com/macaroni-os/anise/cmd/mask"
	cfg "github.com/macaroni-os/anise/pkg/config"

	"github.com/spf13/cobra"
)

func newMaskCommand(config *cfg.AniseConfig) *cobra.Command {

	var ans = &cobra.Command{
		Use:   "mask [command] [OPTIONS]",
		Short: "Manage mask",
	}

	ans.AddCommand(
		cmd_mask.NewMaskAddCommand(config),
		cmd_mask.NewMaskDelCommand(config),
		cmd_mask.NewMaskDisableCommand(config),
		cmd_mask.NewMaskEnableCommand(config),
		cmd_mask.NewMaskListCommand(config),
	)

	return ans
}
