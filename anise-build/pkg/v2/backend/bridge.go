/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package backend

import (
	"fmt"

	cfg "github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/options"

	"github.com/logrusorgru/aurora"
)

type BackendBridge struct {
	Config *cfg.AniseConfig

	Opts *options.Compiler
}

func NewBackendBridge(c *cfg.AniseConfig, opts *options.Compiler) *BackendBridge {
	return &BackendBridge{
		Config: c,
		Opts:   opts,
	}
}

func (bb *BackendBridge) GetConfig() *cfg.AniseConfig   { return bb.Config }
func (bb *BackendBridge) GetOptions() *options.Compiler { return bb.Opts }

func (bb *BackendBridge) BuildArtifact(dst string,
	art *artifact.PackageArtifact,
	solution *artifact.ArtifactsPack,
	genPackage bool,
	pos int) error {

	var msg string

	if genPackage {
		msg = fmt.Sprintf(
			"[%3d of %3d]",
			aurora.Bold(aurora.BrightMagenta(pos)),
			aurora.Bold(aurora.BrightMagenta(len(solution.Artifacts))))
	} else {
		msg = fmt.Sprintf(
			"[%3d of %3d]",
			aurora.BrightMagenta(pos),
			aurora.BrightMagenta(len(solution.Artifacts)))
	}
	InfoC(fmt.Sprintf(":package:%s [%s] Compiling ... :coffee:",
		msg, art.GetPackage().HumanReadableString()))

	// Create Backend instance
	backendService, err := NewBackend(bb.Opts.BackendType, bb.Config)
	if err != nil {
		return err
	}

	// Building build image with the specified instance
	err = backendService.CreateBuildImage(art, dst, solution, bb.Opts)
	if err != nil {
		return err
	}

	// Building final image
	err = backendService.CreateFinalImage(art, dst, solution, bb.Opts)
	if err != nil {
		return err
	}

	// Generate package if needed. Normally genPackage is false
	// when this method is called to build a dependency.
	if genPackage {
		err = backendService.GeneratePackage(art, dst, bb.Opts)
		if err != nil {
			return err
		}
	}

	return nil
}
