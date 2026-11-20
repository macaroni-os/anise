/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd_query

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	helpers "github.com/macaroni-os/anise/cmd/helpers"
	"github.com/macaroni-os/anise/cmd/util"
	cfg "github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	"github.com/macaroni-os/anise/pkg/subsets"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	wagon "github.com/macaroni-os/anise/pkg/v2/repository"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type queryFileWithSubsets struct {
	Path    string   `json:"path" yaml:"path"`
	Subsets []string `json:"subsets" yaml:"subsets"`
}

func NewQueryFilesCommand(config *cfg.AniseConfig) *cobra.Command {

	var ans = &cobra.Command{
		Use:     "files <pkg1> ... <pkgN> [OPTIONS]",
		Short:   "Show files owned by a specific package.",
		Aliases: []string{"fi", "f"},
		PreRun: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				fmt.Println("Missing package")
				os.Exit(1)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			out, _ := cmd.Flags().GetString("output")
			terminal := out != "yaml" && out != "json"

			util.SetSystemConfig()
			installed, _ := cmd.Flags().GetBool("installed")
			filterSubsets, _ := cmd.Flags().GetBool("subsets")
			withRootfsPrefix, _ := cmd.Flags().GetBool("with-rootfs-prefix")

			// Subset rules match paths relative to the package root.
			searchOpts := &wagon.StonesSearchOpts{
				Categories:       []string{},
				Labels:           []string{},
				LabelsMatches:    []string{},
				Matches:          []string{},
				Hidden:           true,
				AndCondition:     false,
				WithFiles:        true,
				WithRootfsPrefix: withRootfsPrefix && !filterSubsets,
			}

			for _, a := range args {
				pack, err := helpers.ParsePackageStr(config, a)
				if err != nil {
					Fatal("Invalid package string ", a, ": ", err.Error())
				}
				searchOpts.Packages = append(searchOpts.Packages, pack)
			}

			config.GetLogging().SetLogLevel("error")

			if filterSubsets {
				if err := subsets.LoadSubsetsDefintions(config); err != nil {
					Fatal("Error loading subset definitions: ", err.Error())
				}
				if err := subsets.LoadSubsetsConfig(config); err != nil {
					Fatal("Error loading subsets configuration: ", err.Error())
				}
			}

			var res *[]*wagon.Stone
			var err error

			searcher := wagon.NewSearcherSimple(config)
			defer searcher.Close()

			if installed {
				res, err = searcher.SearchInstalled(searchOpts)
			} else {
				res, err = searcher.SearchStones(searchOpts)
			}
			if err != nil {
				Fatal("Error on retrieve packages ", err.Error())
			}

			writer := cmd.OutOrStdout()
			var result any
			if filterSubsets {
				files := []queryFileWithSubsets{}
				for _, s := range *res {
					pkgArtifact := &artifact.PackageArtifact{Runtime: s.ToPackage()}
					analyzer, err := subsets.NewSubsetsAnalyzerWithMatches(
						// Enable subsets and trackMatches
						true, true,
						pkgArtifact.GetSubsets(),
					)
					if err != nil {
						Fatal("Error analyzing subsets for ", s.HumanReadableString(), ": ", err.Error())
					}

					for _, f := range s.Files {
						packagePath := filepath.Join("/", f)
						if !analyzer.IsSelected(packagePath) {
							continue
						}
						if withRootfsPrefix {
							f = filepath.Join(config.GetSystem().Rootfs, f)
						}
						files = append(files, queryFileWithSubsets{
							Path:    f,
							Subsets: analyzer.MatchingSubsets(packagePath),
						})
					}
				}
				if terminal {
					for _, file := range files {
						if len(file.Subsets) == 0 {
							fmt.Fprintln(writer, file.Path)
						} else {
							fmt.Fprintf(writer, "%s [subsets: %s]\n", file.Path, strings.Join(file.Subsets, ", "))
						}
					}
					return
				}
				result = files
			} else {
				if terminal {
					for _, s := range *res {
						for _, path := range s.Files {
							fmt.Fprintln(writer, path)
						}
					}
					return
				}
				paths := []string{}
				for _, s := range *res {
					paths = append(paths, s.Files...)
				}
				result = paths
			}

			var data []byte
			if out == "json" {
				data, err = json.Marshal(result)
			} else {
				data, err = yaml.Marshal(result)
			}
			if err != nil {
				Fatal("Error on marshal data ", err.Error())
			}
			fmt.Fprintln(writer, string(data))
		},
	}

	flags := ans.Flags()

	flags.StringP("output", "o", "terminal",
		"Output format ( Defaults: terminal, available: json,yaml )")
	flags.Bool("installed", false, "Search between system packages")
	flags.Bool("subsets", false, "Exclude files from disabled subsets and show matching subsets.")
	flags.Bool("with-rootfs-prefix", true, "Add prefix of the configured rootfs path.")
	return ans
}
