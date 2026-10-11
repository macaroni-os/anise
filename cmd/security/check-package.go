/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd_repo

import (
	"encoding/json"
	"fmt"

	"github.com/ghodss/yaml"
	helpers "github.com/macaroni-os/anise/cmd/helpers"
	"github.com/macaroni-os/anise/cmd/util"
	cfg "github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/subsets"
	"github.com/macaroni-os/anise/pkg/v2/installer"

	. "github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

func NewSecurityCheckCommand(config *cfg.AniseConfig) *cobra.Command {
	var ans = &cobra.Command{
		Use:   "check-package [OPTIONS] <pkg1> <pkg2> ...",
		Short: "Validate files of installed packages.",
		Long: `Check integrity of one or more installed packages.

	Check a package:

		$ anise security check-package coreutils

	Check all installed packages:

		$ anise security check-package --all -e '.*.cpython-39.pyc' -e 'etc/inittab' -e 'etc/conf.d/hostname'

	Check a package generating a JSON report:

		$ anise security check-package coreutils -o json

	Check a package generating a YAML report:

		$ anise security check-package coreutils -o yaml
`,
		Args: cobra.OnlyValidArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			all, _ := cmd.Flags().GetBool("all")
			if len(args) == 0 && !all {
				Fatal("no packages defined")
			}
			if len(args) > 0 && all {
				Fatal("--all flag used with packages")
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			var toCheck pkg.DefaultPackages
			var results *installer.CheckResults
			var err error

			out, _ := cmd.Flags().GetString("output")
			verbose, _ := cmd.Flags().GetBool("verbose")
			all, _ := cmd.Flags().GetBool("all")
			ignoreRules, _ := cmd.Flags().GetStringSlice("exclude")

			if out != "terminal" {
				config.GetLogging().SetLogLevel("error")
			}

			InfoC(fmt.Sprintf(":rocket:%s %s",
				Bold(Blue("Anise")), Bold(Blue(util.Version()))))

			// Load config protect configs
			installer.LoadConfigProtectConfs(config)
			// Load subsets defintions
			subsets.LoadSubsetsDefintions(config)
			// Load subsets config
			subsets.LoadSubsetsConfig(config)

			aManager := installer.NewArtifactsManager(config)
			defer aManager.Close()

			if len(ignoreRules) > 0 {
				for _, r := range ignoreRules {
					DebugC(fmt.Sprintf("Adding ignore rule %s...", r))
				}
			}

			opts := &installer.CheckOpts{
				Ignore:      true,
				WithSubsets: true,
				Verbose:     verbose,
				IgnoreRules: ignoreRules,
			}

			if all {
				results, err = aManager.IntegrityCheckAll(opts)
			} else {
				for _, a := range args {
					pack, err := helpers.ParsePackageStr(config, a)
					if err != nil {
						Fatal("Invalid package string ", a, ": ", err.Error())
					}
					toCheck = append(toCheck, pack)
				}

				results, err = aManager.IntegrityCheck(opts,
					toCheck...,
				)
			}

			if err != nil {
				Fatal("Error: " + err.Error())
			}

			if out == "json" {
				data, err := json.Marshal(results)
				if err != nil {
					Fatal("error on marshal: " + err.Error())
				}
				fmt.Println(string(data))
			} else if out == "yaml" {
				data, err := yaml.Marshal(results)
				if err != nil {
					Fatal("error on marshal: " + err.Error())
				}
				fmt.Println(string(data))
			} else {

				if results.Corrupted > 0 {
					InfoC(fmt.Sprintf(":warning: %s",
						Bold(Blue(fmt.Sprintf(
							"Analyzed %d packages: %d corrupted.",
							len(results.Packages), results.Corrupted)))))
				} else {
					InfoC(fmt.Sprintf(":confetti_ball:%s",
						Bold(Blue(fmt.Sprintf(
							"Analyzed %d packages: No corrupted packages.",
							len(results.Packages))))))
				}
			}
		},
	}

	flags := ans.Flags()

	flags.StringP("output", "o", "terminal",
		"Output format ( Defaults: terminal, available: json,yaml )")
	flags.Bool("verbose", false, "Increase verbosity for terminal output.")
	flags.Bool("all", false, "Check all installed packages.")
	flags.StringSliceP("exclude", "e", []string{},
		"Add exclude rules on check packages. For example for *.pyc, etc.")

	return ans
}
