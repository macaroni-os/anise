/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	bhelpers "github.com/macaroni-os/anise/anise-build/cmd/helpers"
	solver "github.com/macaroni-os/anise/anise-build/pkg/v2/solver"
	"github.com/macaroni-os/anise/cmd/util"
	cfg "github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/compression"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/options"

	"github.com/logrusorgru/aurora"
	. "github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

func newBuildCommand(config *cfg.AniseConfig) *cobra.Command {

	var buildCmd = &cobra.Command{
		Use:   "build <package name> <package name> <package name> ...",
		Short: "build a package or a tree",
		Long: `Builds one or more packages from a tree (current directory is implied):

		$ anise-build build utils/busybox utils/yq ...

	Builds all packages

		$ anise-build build --all

	Builds only the leaf packages:

		$ anise-build build --full

	Build package revdeps:

		$ anise-build build --revdeps utils/yq

	Build package without dependencies (needs the images already in the host, or either need to be available online):

		$ anise-build build --nodeps utils/yq ...

	Build packages specifying multiple definition trees:

		$ anise-build build --tree overlay/path --tree overlay/path2 utils/yq ...
	`, PreRun: func(cmd *cobra.Command, args []string) {
			config.Viper.BindPFlag("tree", cmd.Flags().Lookup("tree"))
			config.Viper.BindPFlag("destination", cmd.Flags().Lookup("destination"))
			config.Viper.BindPFlag("backend", cmd.Flags().Lookup("backend"))
			config.Viper.BindPFlag("privileged", cmd.Flags().Lookup("privileged"))
			config.Viper.BindPFlag("revdeps", cmd.Flags().Lookup("revdeps"))
			config.Viper.BindPFlag("all", cmd.Flags().Lookup("all"))
			config.Viper.BindPFlag("compression", cmd.Flags().Lookup("compression"))
			config.Viper.BindPFlag("nodeps", cmd.Flags().Lookup("nodeps"))
			config.Viper.BindPFlag("onlydeps", cmd.Flags().Lookup("onlydeps"))
			bhelpers.BindValuesFlags(cmd)
			config.Viper.BindPFlag("backend-args", cmd.Flags().Lookup("backend-args"))

			config.Viper.BindPFlag("image-repository", cmd.Flags().Lookup("image-repository"))
			config.Viper.BindPFlag("push", cmd.Flags().Lookup("push"))
			config.Viper.BindPFlag("pull", cmd.Flags().Lookup("pull"))
			config.Viper.BindPFlag("wait", cmd.Flags().Lookup("wait"))
			config.Viper.BindPFlag("keep-images", cmd.Flags().Lookup("keep-images"))

			bhelpers.BindSolverFlags(cmd)

			config.Viper.BindPFlag("general.show_build_output", cmd.Flags().Lookup("live-output"))
			config.Viper.BindPFlag("backend-args", cmd.Flags().Lookup("backend-args"))

			config.Viper.Unmarshal(&config)
		},
		Run: func(cmd *cobra.Command, args []string) {
			var err error

			treePaths := config.Viper.GetStringSlice("tree")
			templatesDirs := config.Viper.GetStringSlice("templates-dir")
			values := bhelpers.ValuesFlags()

			stype, _ := cmd.Flags().GetString("solver-type")

			dst := config.Viper.GetString("destination")
			concurrency := config.GetGeneral().Concurrency
			backendType := config.Viper.GetString("backend")
			privileged := config.Viper.GetBool("privileged")
			//revdeps := config.Viper.GetBool("revdeps")
			//all := config.Viper.GetBool("all")
			compressionType := config.Viper.GetString("compression")
			imageRepository := config.Viper.GetString("image-repository")
			//wait := config.Viper.GetBool("wait")
			push := config.Viper.GetBool("push")
			pull := config.Viper.GetBool("pull")
			keepImages := config.Viper.GetBool("keep-images")
			//nodeps := config.Viper.GetBool("nodeps")
			//onlydeps := config.Viper.GetBool("onlydeps")
			onlyTarget, _ := cmd.Flags().GetBool("only-target-package")
			//full, _ := cmd.Flags().GetBool("full")
			rebuild, _ := cmd.Flags().GetBool("rebuild")
			backendArgs := config.Viper.GetStringSlice("backend-args")

			out, _ := cmd.Flags().GetString("output")
			if out != "terminal" {
				config.GetLogging().SetLogLevel("error")
			}

			InfoC(fmt.Sprintf(":rocket:%s %s",
				Bold(Blue("Anise Build")), Bold(Blue(util.Version()))))

			pretend, _ := cmd.Flags().GetBool("pretend")
			//fromRepo, _ := cmd.Flags().GetBool("from-repositories")

			buildManager := solver.NewBuildManager(config)

			// Create builder opts
			opts := solver.NewBuildSolverOpts()

			err = buildManager.PrepareSolver(
				stype, opts, treePaths,
				templatesDirs, values)
			if err != nil {
				Fatal(err)
			}

			var candidates *artifact.ArtifactsPack

			if pretend {
				candidates, err = buildManager.BuildPretend(args)
				if err != nil {
					Fatal(err)
				}
			} else {

				// Prepare build options
				buildOpts := options.NewDefaultCompiler()
				buildOpts.Apply(
					options.PushImages(push),
					options.WithPushRepository(imageRepository),
					options.PullFirst(pull),
					options.KeepImg(keepImages),
					options.Privileged(privileged),
					options.Concurrency(concurrency),
					options.WithCompressionType(compression.Implementation(compressionType)),
					options.OnlyTarget(onlyTarget),
					options.Rebuild(rebuild),
					options.BackendArgs(backendArgs),
					options.WithBackendType(backendType),
				)

				candidates, err = buildManager.Build(args, dst, buildOpts)
				if err != nil {
					Fatal(err)
				}

			}

			switch out {
			case "yaml":
				data, err := candidates.YAML()
				if err != nil {
					Fatal(err)
				}
				fmt.Println(string(data))
			case "json":
				data, err := candidates.JSON()
				if err != nil {
					Fatal(err)
				}
				fmt.Println(string(data))
			default:
				tot := len(candidates.Artifacts)
				if tot == 0 {
					Warning(":head_bandage: No packages selected for build!")
				} else {
					if pretend {
						InfoC(fmt.Sprintf(":construction: %s", Bold("Candidates for the build:")))
					} else {
						InfoC(fmt.Sprintf(":construction: %s", Bold("Packages built:")))
					}
					var msg string
					for i := range candidates.Artifacts {

						if candidates.Artifacts[i].IsToGenerate() || !onlyTarget {
							msg = fmt.Sprintf(
								"[%3d of %3d] %-65s - %-15s",
								aurora.Bold(aurora.BrightMagenta(i+1)),
								aurora.Bold(aurora.BrightMagenta(tot)),
								Bold(candidates.Artifacts[i].GetPackage().PackageName()),
								Bold(candidates.Artifacts[i].GetPackage().GetVersion()))
						} else {
							msg = fmt.Sprintf(
								"[%3d of %3d] %-65s - %-15s",
								aurora.BrightMagenta(i+1),
								aurora.BrightMagenta(tot),
								candidates.Artifacts[i].GetPackage().PackageName(),
								candidates.Artifacts[i].GetPackage().GetVersion())
						}
						if pretend {
							Info(fmt.Sprintf(":package:%s", msg))
						} else {
							Info(fmt.Sprintf(":package:%s:check_mark:", msg))
						}
					}
				}
			}

		},
	}

	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}

	flags := buildCmd.Flags()

	flags.StringSliceP("tree", "t", []string{path},
		"Path of the tree to use.")
	flags.StringSlice("templates-dir", []string{filepath.Join(path, "templates")},
		"Path of the render templates to use.")

	flags.String("solver-type", "", "Solver strategy")
	flags.Bool("pretend", false, "Just print what packages will be compiled")
	flags.StringP("output", "o", "terminal", "Output format ( Defaults: terminal, available: json,yaml )")

	flags.String("backend", "docker", "backend used (docker,img)")
	flags.Bool("privileged", true, "Privileged (Keep permissions)")
	flags.Bool("revdeps", false, "Build with revdeps")
	flags.Bool("all", false, "Build all specfiles in the tree")
	flags.Bool("full", false, "Build all packages (optimized)")
	flags.StringSlice("values", []string{}, "Build values file to interpolate with each package")
	flags.StringSliceP("backend-args", "a", []string{}, "Backend args")

	flags.String("destination", filepath.Join(path, "build"), "Destination folder")
	flags.String("compression", "none", "Compression alg: none, gzip, zstd")
	flags.String("image-repository", "anise-build/cache", "Default base image string for generated image")
	flags.Bool("push", false, "Push images to a hub")
	flags.Bool("pull", false, "Pull images from a hub")
	flags.Bool("keep-images", true, "Keep built docker images in the host")
	flags.Bool("nodeps", false, "Build only the target packages, skipping deps (it works only if you already built the deps locally, or by using --pull) ")
	flags.Bool("onlydeps", false, "Build only package dependencies")
	flags.Bool("only-target-package", false, "Build packages of only the required target. Otherwise builds all the necessary ones not present in the destination")
	flags.Bool("live-output", config.GetGeneral().ShowBuildOutput, "Enable live output of the build phase.")
	flags.Bool("rebuild", false, "To combine with --pull. Allows to rebuild the target package even if an image is available, against a local values file")
	flags.StringArrayP("pull-repository", "p", []string{}, "A list of repositories to pull the cache from")
	//flags.Bool("from-repositories", false, "Consume the user-defined repositories to pull specfiles from")
	/*
		flags.Bool("wait", false, "Don't build all intermediate images, but wait for them until they are available")

	*/
	return buildCmd
}
