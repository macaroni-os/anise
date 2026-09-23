/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package cmd_tree

import (
	"fmt"

	. "github.com/macaroni-os/anise/pkg/logger"
	spectooling "github.com/macaroni-os/anise/pkg/spectooling"
	tree "github.com/macaroni-os/anise/pkg/v2/tree"
	version "github.com/macaroni-os/anise/pkg/versioner"

	"github.com/spf13/cobra"
)

func NewTreeBumpCommand() *cobra.Command {

	var ans = &cobra.Command{
		Use:   "bump [OPTIONS]",
		Short: "Bump a new package build version.",
		Args:  cobra.OnlyValidArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			df, _ := cmd.Flags().GetString("definition-file")
			if df == "" {
				Fatal("Mandatory definition.yaml path missing.")
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			spec, _ := cmd.Flags().GetString("definition-file")
			toStdout, _ := cmd.Flags().GetBool("to-stdout")
			pkgVersion, _ := cmd.Flags().GetString("pkg-version")
			pack, err := tree.ReadDefinitionFile(spec)
			if err != nil {
				Fatal(err.Error())
			}

			if pkgVersion != "" {
				validator := &version.WrappedVersioner{}
				err := validator.Validate(pkgVersion)
				if err != nil {
					Fatal("Invalid version string: " + err.Error())
				}
				pack.SetVersion(pkgVersion)
			} else {
				// Retrieve version build section with Gentoo parser
				err = pack.BumpBuildVersion()
				if err != nil {
					Fatal("Error on increment build version: " + err.Error())
				}
			}
			if toStdout {
				data, err := spectooling.NewDefaultPackageSanitized(pack).Yaml()
				if err != nil {
					Fatal("Error on yaml conversion: " + err.Error())
				}
				fmt.Println(string(data))
			} else {

				err = tree.WriteDefinitionFile(pack, spec)
				if err != nil {
					Fatal("Error on write definition file: " + err.Error())
				}

				fmt.Printf("Bumped package %s/%s-%s.\n", pack.Category, pack.Name, pack.Version)
			}
		},
	}

	ans.Flags().StringP("pkg-version", "p", "", "Set a specific package version")
	ans.Flags().StringP("definition-file", "f", "", "Path of the definition to bump.")
	ans.Flags().BoolP("to-stdout", "o", false, "Bump package to output.")

	return ans
}
