/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd_tree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bhelpers "github.com/macaroni-os/anise/anise-build/cmd/helpers"
	cfg "github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/specs"
	"github.com/macaroni-os/anise/pkg/v2/render"
	"github.com/macaroni-os/anise/pkg/v2/tree"

	"github.com/spf13/cobra"
)

func NewTreeRender(config *cfg.AniseConfig) *cobra.Command {
	var ans = &cobra.Command{
		Use:   "render [OPTIONS] <package-selector>",
		Short: "Show rendered build.yaml file of a selected package.",
		Args:  cobra.OnlyValidArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			bhelpers.BindValuesFlags(cmd)

			if len(args) == 0 {
				Fatal("No package selected.")
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			treePaths, _ := cmd.Flags().GetStringArray("tree")
			templatesDirs, _ := cmd.Flags().GetStringArray("templates-dir")
			output, _ := cmd.Flags().GetString("output")
			values := bhelpers.ValuesFlags()
			pkgstr := args[0]

			if output != "terminal" {
				config.GetLogging().SetLogLevel("error")
			}

			// Creating render engine for build
			rEngine := render.NewRenderEngine(config)
			err := rEngine.LoadTemplates(templatesDirs)
			if err != nil {
				Fatal("fail to load render templates dirs:", err.Error())
			}

			err = rEngine.LoadDefaultValues(values)
			if err != nil {
				Fatal("fail to load render default values:", err.Error())
			}

			// Load the index file
			forestGuard := tree.NewForestGuard(config)
			err = forestGuard.LoadTrees(treePaths, false)
			if err != nil {
				Fatal("fail to read tree indexes:", err.Error())
			}

			matches, err := forestGuard.Search(pkgstr)
			if err != nil {
				Fatal(fmt.Sprintf(
					"fail to search package for string %s: %s",
					pkgstr, err.Error()))
			}

			if len(matches) == 0 {
				Warning(fmt.Sprintf(
					"No packages found with the selector %s",
					pkgstr))
				os.Exit(1)
			}

			for _, tidx := range matches {
				for k, v := range tidx.Map {
					for _, ver := range v {
						InfoC(fmt.Sprintf(":eye: Processing package %s-%s...",
							k, ver.Version))

						pkgPath := filepath.Join(tidx.TreePath, tidx.BaseDir,
							filepath.Dir(ver.Path))

						defFile := filepath.Join(pkgPath, filepath.Base(ver.Path))
						buildFile := filepath.Join(pkgPath, "build.yaml")

						DebugC(":brain:Using buildfile:\t", buildFile)
						DebugC(":brain:Using package specs:\t", defFile)

						var cs *specs.CompilationSpecLoad
						if filepath.Base(defFile) == "collection.yaml" {

							words := strings.Split(k, "/")
							atom := pkg.NewPackageWithCatThin(words[0], words[1],
								ver.Version)

							cs, err = tree.ReadBuildFileFromCollection(buildFile, defFile,
								rEngine, atom, map[string]interface{}{})
						} else {
							cs, err = tree.ReadBuildFile(buildFile, defFile,
								rEngine, map[string]interface{}{})
						}
						if err != nil {
							Fatal(fmt.Sprintf(
								"error on rendering package %s-%s: %s",
								k, ver.Version, err.Error()))
						}

						// Disable build options
						cs.BuildOptions = nil

						var buildcontent string

						if output == "terminal" || output == "" || output == "yaml" {
							out, err := cs.YAML()
							if err != nil {
								Fatal(fmt.Sprintf(
									"error on marshal package %s-%s: %s",
									k, ver.Version, err.Error()))
							}

							buildcontent = string(out)
						} else {
							out, err := cs.Json()
							if err != nil {
								Fatal(fmt.Sprintf(
									"error on marshal package %s-%s: %s",
									k, ver.Version, err.Error()))
							}
							buildcontent = string(out)
						}

						fmt.Println()
						fmt.Println(buildcontent)
						fmt.Println()
					}
				}
			}

		},
	}

	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}

	flags := ans.Flags()
	flags.StringArrayP("tree", "t", []string{path},
		"Path of the tree to use.")
	flags.StringSlice("values", []string{}, "Build values file to interpolate with each package")
	flags.StringArray("templates-dir", []string{filepath.Join(path, "templates")},
		"Path of the render templates to use.")
	ans.Flags().StringP("output", "o", "terminal",
		"Output format ( Defaults: terminal, available: terminal,json,yaml )")

	return ans
}
