/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package cmd_tree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	bhelpers "github.com/macaroni-os/anise/anise-build/cmd/helpers"
	"github.com/macaroni-os/anise/anise-build/pkg/v2/backend"
	"github.com/macaroni-os/anise/anise-build/pkg/v2/solver"
	. "github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/options"

	. "github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type TreePackageResult struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Version  string `json:"version"`
	Path     string `json:"path"`
	Image    string `json:"image"`
}

type TreeResults struct {
	Packages []TreePackageResult `json:"packages"`
}

func NewTreeImageCommand(config *AniseConfig) *cobra.Command {

	var ans = &cobra.Command{
		Use:   "images [OPTIONS]",
		Short: "List of the images of a package",
		PreRun: func(cmd *cobra.Command, args []string) {
			t, _ := cmd.Flags().GetStringArray("tree")
			if len(t) == 0 {
				Fatal("Mandatory tree param missing.")
			}

			if len(args) != 1 {
				Fatal("Expects one package as parameter")
			}
			bhelpers.BindValuesFlags(cmd)
			viper.BindPFlag("image-repository", cmd.Flags().Lookup("image-repository"))

		},
		Run: func(cmd *cobra.Command, args []string) {
			var results TreeResults

			treePath, _ := cmd.Flags().GetStringArray("tree")
			imageRepository := viper.GetString("image-repository")
			backendType, _ := cmd.Flags().GetString("backend")
			//pullRepo, _ := cmd.Flags().GetStringArray("pull-repository")
			values := bhelpers.ValuesFlags()
			templatesDirs := config.Viper.GetStringSlice("templates-dir")
			stype := ""

			out, _ := cmd.Flags().GetString("output")
			if out != "terminal" {
				AniseCfg.GetLogging().SetLogLevel("error")
			}

			buildManager := solver.NewBuildManager(config)
			opts := solver.NewBuildSolverOpts()

			err := buildManager.PrepareSolver(
				stype, opts, treePath,
				templatesDirs, values)
			if err != nil {
				Fatal(err)
			}

			candidates, err := buildManager.BuildPretend(args)
			if err != nil {
				Fatal(err)
			}

			// Prepare build options
			buildOpts := options.NewDefaultCompiler()
			buildOpts.Apply(
				options.WithPushRepository(imageRepository),
				options.WithBackendType(backendType),
			)

			backendService, err := backend.NewBackend(backendType, config)
			if err != nil {
				Fatal(err)
			}
			// Create the backend bridge
			withDeps := true
			results = TreeResults{}

			solutionMap := candidates.ToMap()
			// Generate final image name for every candidate
			for idx := range candidates.Artifacts {
				pThin, err := candidates.Artifacts[idx].ToPackageThin(withDeps, solutionMap)
				if err != nil {
					Fatal(err)
				}

				err = backendService.GenerateFinalImageHash(candidates.Artifacts[idx],
					pThin, buildOpts)
				if err != nil {
					Fatal(err)
				}

				results.Packages = append(results.Packages, TreePackageResult{
					Name:     pThin.GetName(),
					Category: pThin.GetCategory(),
					Version:  pThin.GetVersion(),
					Path:     "",
					Image: fmt.Sprintf("%s:%s", buildOpts.PushImageRepository,
						candidates.Artifacts[idx].FinalImageHash),
				})
			}

			switch out {
			case "yaml":
				y, err := yaml.Marshal(results)
				if err != nil {
					fmt.Printf("err: %v\n", err)
					return
				}
				fmt.Println(string(y))
			case "json":
				y, err := json.Marshal(results)
				if err != nil {
					fmt.Printf("err: %v\n", err)
					return
				}
				fmt.Println(string(y))
			default:

				InfoC(Bold(":whale2: List of images for package:"))
				ndeps := len(results.Packages) - 2
				for i := 0; i < ndeps; i++ {
					p := results.Packages[i]
					InfoC(fmt.Sprintf(":wrench: %s :right_arrow: %s/%s-%s",
						p.Image,
						p.Category, p.Name, p.Version,
					))
				}
				ndeps++
				InfoC(fmt.Sprintf(":package: %s :right_arrow: %s",
					Bold(results.Packages[ndeps].Image),
					Bold(fmt.Sprintf("%s/%s-%s",
						results.Packages[ndeps].Category, results.Packages[ndeps].Name,
						results.Packages[ndeps].Version,
					)),
				))
			}
		},
	}
	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}
	ans.Flags().String("backend", "dockerv3", "backend used (docker)")
	ans.Flags().StringP("output", "o", "terminal", "Output format ( Defaults: terminal, available: json,yaml )")
	ans.Flags().StringArrayP("tree", "t", []string{path}, "Path of the tree to use.")
	ans.Flags().String("image-repository", "anise/cache", "Default base image string for generated image")
	ans.Flags().StringArrayP("pull-repository", "p", []string{}, "A list of repositories to pull the cache from")
	ans.Flags().StringSlice("templates-dir", []string{filepath.Join(path, "templates")},
		"Path of the render templates to use.")

	return ans
}
