/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package backend

import (
	"fmt"

	cfg "github.com/macaroni-os/anise/pkg/config"
	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/options"
)

const (
	// Keep old values for compatibility
	DockerBackend   = "docker"
	DockerBackendV2 = "dockerv2"
	DockerBackendV3 = "dockerv3"
)

type BackendCompiler interface {
	CreateBuildImage(art *artifact.PackageArtifact, builddir string,
		solution *artifact.ArtifactsPack, opts *options.Compiler) error
	CreateFinalImage(art *artifact.PackageArtifact, builddir string,
		solution *artifact.ArtifactsPack, opts *options.Compiler) error
	GeneratePackage(art *artifact.PackageArtifact,
		builddir string, opts *options.Compiler) error

	GenerateFinalImageHash(art *artifact.PackageArtifact,
		pthin *pkg.PackageThin, opts *options.Compiler) error
	GenerateBuildImageHash(art *artifact.PackageArtifact,
		pthin *pkg.PackageThin, opts *options.Compiler) error
}

func NewBackend(s string, c *cfg.AniseConfig) (BackendCompiler, error) {
	var compilerBackend BackendCompiler

	switch s {
	case DockerBackend, DockerBackendV2, DockerBackendV3:
		compilerBackend = NewDockerv3Backend(c)
	default:
		return nil, fmt.Errorf("invalid backend %s. Unsupported", s)
	}

	return compilerBackend, nil
}
