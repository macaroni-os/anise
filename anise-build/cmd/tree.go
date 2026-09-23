/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd

import (
	. "github.com/macaroni-os/anise/anise-build/cmd/tree"
	cfg "github.com/macaroni-os/anise/pkg/config"

	"github.com/spf13/cobra"
)

func newTreeCommand(config *cfg.AniseConfig) *cobra.Command {

	var treeGroupCmd = &cobra.Command{
		Use:   "tree [command] [OPTIONS]",
		Short: "Tree operations",
	}

	treeGroupCmd.AddCommand(
		NewTreeGenIdx(config),
		NewTreePkglistCommand(config),
		NewTreeValidateCommand(config),
		NewTreeBumpCommand(),
		NewTreeImageCommand(config),
		NewTreeRender(config),
	)

	return treeGroupCmd
}
