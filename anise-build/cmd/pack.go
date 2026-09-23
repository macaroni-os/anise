/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package cmd

import (
	"os"
	"path/filepath"
	"time"

	helpers "github.com/macaroni-os/anise/cmd/helpers"
	cfg "github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/compression"
	compilerspec "github.com/macaroni-os/anise/pkg/v2/compiler/types/specs"

	"github.com/spf13/cobra"
)

func newPackCommand(config *cfg.AniseConfig) *cobra.Command {

	var packCmd = &cobra.Command{
		Use:   "pack <package name>",
		Short: "pack a custom package",
		Long: `Pack creates a package from a directory, generating the metadata required from a tree to generate a repository.

	Pack can be used to manually replace what "anise-build build" does automatically by reading the packages build.yaml files.

		$ mkdir -p output/etc/foo
		$ echo "my config" > output/etc/foo
		$ anise-build pack foo/bar@1.1 --source output

	Afterwards, you can use the content generated and associate it with a tree and a corresponding definition.yaml file with "anise-build create-repo".
	`,

		PreRun: func(cmd *cobra.Command, args []string) {
			config.Viper.BindPFlag("destination", cmd.Flags().Lookup("destination"))
			config.Viper.BindPFlag("compression", cmd.Flags().Lookup("compression"))
			config.Viper.BindPFlag("source", cmd.Flags().Lookup("source"))
		},
		Run: func(cmd *cobra.Command, args []string) {
			sourcePath := config.Viper.GetString("source")

			dst := config.Viper.GetString("destination")
			compressionType := config.Viper.GetString("compression")
			concurrency := config.GetGeneral().Concurrency

			if len(args) != 1 {
				Fatal("You must specify a package name")
			}

			packageName := args[0]

			p, err := helpers.ParsePackageStr(config, packageName)
			if err != nil {
				Fatal("Invalid package string ", packageName, ": ", err.Error())
			}

			spec := &compilerspec.CompilationSpec{Package: p}
			a := artifact.NewPackageArtifact(filepath.Join(dst, p.GetFingerPrint()+".package.tar"))
			a.CompressionType = compression.Implementation(compressionType)
			err = a.Compress(sourcePath, concurrency)
			if err != nil {
				Fatal("failed compressing ", packageName, ": ", err.Error())
			}
			a.CompileSpec = spec
			filelist, err := a.FileList()
			if err != nil {
				Fatal("failed generating file list for ", packageName, ": ", err.Error())
			}
			a.Files = filelist
			a.CompileSpec.GetPackage().SetBuildTimestamp(time.Now().String())
			err = a.WriteYaml(dst)
			if err != nil {
				Fatal("failed writing metadata yaml file for ", packageName, ": ", err.Error())
			}
		},
	}

	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}
	packCmd.Flags().String("source", path, "Source folder")
	packCmd.Flags().String("destination", path, "Destination folder")
	packCmd.Flags().String("compression", "gzip", "Compression alg: none, gzip")

	return packCmd
}
