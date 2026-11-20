/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package helpers

import (
	"io"
	"os"

	. "github.com/macaroni-os/anise/pkg/config"

	tarf "github.com/geaaru/tar-formers/pkg/executor"
	tarf_specs "github.com/geaaru/tar-formers/pkg/specs"
	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"
)

func Tar(src, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	fs, err := archive.Tar(src, compression.None)
	if err != nil {
		return err
	}
	defer fs.Close()

	_, err = io.Copy(out, fs)
	if err != nil {
		return err
	}

	err = out.Sync()
	if err != nil {
		return err
	}
	return err
}

func UntarProtect(src, dst string, sameOwner, overwriteDirPerms bool, protectedFiles []string, modifier tarf.TarFileHandlerFunc) error {

	spec := tarf_specs.NewSpecFile()
	spec.SameOwner = sameOwner
	spec.OverwritePerms = overwriteDirPerms
	spec.IgnoreRegexes = []string{
		// prevent 'operation not permitted'
		//"^/dev/",
	}
	spec.IgnoreFiles = []string{}
	spec.EnableMutex = AniseCfg.GetTarFlows().Mutex4Dirs
	spec.MaxOpenFiles = AniseCfg.GetTarFlows().MaxOpenFiles
	spec.BufferSize = AniseCfg.GetTarFlows().CopyBufferSize
	spec.Validate = AniseCfg.GetTarFlows().Validate

	return UntarProtectSpec(
		src, dst, protectedFiles, modifier, spec,
	)
}

func prepareTarformers(in io.Reader, modifier tarf.TarFileHandlerFunc,
	spec *tarf_specs.SpecFile, protectedFiles []string) *tarf.TarFormers {
	tarformers := tarf.NewTarFormers(tarf.GetOptimusPrime().Config)
	tarformers.SetReader(in)

	if modifier != nil && len(protectedFiles) > 0 {
		tarformers.SetFileHandler(modifier)

		spec.TriggeredFiles = protectedFiles
	}

	return tarformers
}

func UntarProtectSpec(src, dst string, protectedFiles []string, modifier tarf.TarFileHandlerFunc, spec *tarf_specs.SpecFile) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tarformers := prepareTarformers(in, modifier, spec, protectedFiles)

	return tarformers.RunTask(spec, dst)
}
