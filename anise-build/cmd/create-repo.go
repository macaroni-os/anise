/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd

import (
	"os"
	"path/filepath"

	"github.com/macaroni-os/anise/anise-build/pkg/v2/repository"
	helpers "github.com/macaroni-os/anise/cmd/helpers"
	cfg "github.com/macaroni-os/anise/pkg/config"
	wagon "github.com/macaroni-os/anise/pkg/v2/repository"

	tarf_tools "github.com/geaaru/tar-formers/pkg/tools"
	"github.com/spf13/cobra"
)

func newCreateRepoCommand(config *cfg.AniseConfig) *cobra.Command {

	var createrepoCmd = &cobra.Command{
		Use:   "create-repo",
		Short: "Create a anise-build repository from a build",
		Long: `Builds tree metadata from a set of packages and a tree definition:

		$ anise-build create-repo

	Provide specific paths for packages, tree, and metadata output which is generated:

		$ anise-build create-repo --packages my/packages/path --tree my/tree/path --output my/packages/path ...

	Provide name and description of the repository:

		$ anise-build create-repo --name "foo" --description "bar" ...

	Change compression method:
		
		$ anise-build create-repo --tree-compression gzip --meta-compression gzip

	Create a repository from the metadata description defined in the anise-build.yaml config file:

		$ anise-build create-repo --repo repository1
	`,
		PreRun: func(cmd *cobra.Command, args []string) {
			config.Viper.BindPFlag("packages", cmd.Flags().Lookup("packages"))
			config.Viper.BindPFlag("tree", cmd.Flags().Lookup("tree"))
			config.Viper.BindPFlag("output", cmd.Flags().Lookup("output"))
			config.Viper.BindPFlag("backend", cmd.Flags().Lookup("backend"))
			config.Viper.BindPFlag("name", cmd.Flags().Lookup("name"))
			config.Viper.BindPFlag("descr", cmd.Flags().Lookup("descr"))
			config.Viper.BindPFlag("urls", cmd.Flags().Lookup("urls"))
			config.Viper.BindPFlag("type", cmd.Flags().Lookup("type"))
			config.Viper.BindPFlag("tree-compression", cmd.Flags().Lookup("tree-compression"))
			config.Viper.BindPFlag("tree-filename", cmd.Flags().Lookup("tree-filename"))
			//config.Viper.BindPFlag("meta-compression", cmd.Flags().Lookup("meta-compression"))
			//config.Viper.BindPFlag("meta-filename", cmd.Flags().Lookup("meta-filename"))
			config.Viper.BindPFlag("reset-revision", cmd.Flags().Lookup("reset-revision"))
			config.Viper.BindPFlag("repo", cmd.Flags().Lookup("repo"))
			//config.Viper.BindPFlag("from-metadata", cmd.Flags().Lookup("from-metadata"))
			config.Viper.BindPFlag("force-push", cmd.Flags().Lookup("force-push"))
			config.Viper.BindPFlag("push-images", cmd.Flags().Lookup("push-images"))
			config.Viper.BindPFlag("with-compilertree", cmd.Flags().Lookup("with-compilertree"))
		},
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			var repo *cfg.AniseRepository

			treePaths := config.Viper.GetStringSlice("tree")
			dst := config.Viper.GetString("output")

			name := config.Viper.GetString("name")
			descr := config.Viper.GetString("descr")
			urls := config.Viper.GetStringSlice("urls")
			t := config.Viper.GetString("type")
			reset := config.Viper.GetBool("reset-revision")
			treetype := config.Viper.GetString("tree-compression")
			treeName := config.Viper.GetString("tree-filename")
			sourceRepo := config.Viper.GetString("repo")
			checkPackageTarball := config.Viper.GetBool("check-package-tarball")
			withCompilerTree := config.Viper.GetBool("with-compilertree")
			//backendType := config.Viper.GetString("backend")
			//fromRepo, _ := cmd.Flags().GetBool("from-repositories")

			//compilerBackend, err := compiler.NewBackend(backendType)
			helpers.CheckErr(err)
			//force := config.Viper.GetBool("force-push")
			//imagePush := config.Viper.GetBool("push-images")

			opts := repository.NewWagonFactoryOpts()
			opts.ResetRevision = reset
			opts.OutputDir = dst
			opts.PackagesDir = config.Viper.GetString("packages")
			opts.CompressionMode = tarf_tools.ParseCompressionMode(treetype)
			opts.CheckPackageTarball = checkPackageTarball
			opts.WithCompilerTree = withCompilerTree
			if treeName != "" {
				opts.TreeFilename = treeName
			}

			// Prepare Repository instance
			if sourceRepo != "" {
				// Search for system repository
				repo, err = config.GetSystemRepository(sourceRepo)
				helpers.CheckErr(err)

				if len(treePaths) <= 0 {
					treePaths = []string{repo.TreePath}
				}
			} else {
				repo = cfg.NewAniseRepository(name, t, descr, urls, 9999, true, true)
			}

			factory := repository.NewWagonFactory(config, repo)
			err = factory.BumpRevision(treePaths, opts)
			helpers.CheckErr(err)

		},
	}

	path, err := os.Getwd()
	helpers.CheckErr(err)

	flags := createrepoCmd.Flags()
	flags.String("packages", filepath.Join(path, "build"), "Packages folder (output from build)")
	flags.StringSliceP("tree", "t", []string{path}, "Path of the source trees to use.")
	flags.String("output", filepath.Join(path, "build"), "Destination for generated archives. With 'docker' repository type, it should be an image reference (e.g 'foo/bar')")
	flags.String("name", "anise-build", "Repository name")
	flags.String("descr", "anise-build", "Repository description")
	flags.StringSlice("urls", []string{}, "Repository URLs")
	flags.String("type", "disk", "Repository type (disk, http, docker)")
	flags.Bool("reset-revision", false, "Reset repository revision.")
	flags.Bool("check-package-tarball", false, "Validate presence of package tarball.")
	flags.String("repo", "", "Use repository defined in configuration.")
	flags.String("backend", "docker", "backend used (docker,img)")

	flags.Bool("force-push", false, "Force overwrite of docker images if already present online")
	flags.Bool("push-images", false, "Enable/Disable docker image push for docker repositories")
	//flags.Bool("from-metadata", false, "Consider metadata files from the packages folder while indexing the new tree")

	flags.Bool("with-compilertree", false, "Create compiler tree tarball.")
	flags.String("tree-compression", "none", "Compression alg: none (self-autodetect), gzip, zstd")
	flags.String("tree-filename", wagon.TREE_TARBALL, "Repository tree filename")
	//flags.Bool("from-repositories", false, "Consume the user-defined repositories to pull specfiles from")

	return createrepoCmd
}
