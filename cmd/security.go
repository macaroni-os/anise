/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd

import (
	. "github.com/macaroni-os/anise/cmd/security"
	cfg "github.com/macaroni-os/anise/pkg/config"

	"github.com/spf13/cobra"
)

func newSecurityCommand(config *cfg.AniseConfig) *cobra.Command {

	var ans = &cobra.Command{
		Use:   "security [command] [OPTIONS]",
		Short: "Validate packages and rootfs.",
	}

	ans.AddCommand(
		NewSecurityCheckCommand(config),
	)

	return ans
}
