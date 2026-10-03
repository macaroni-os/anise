/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package solver

import (
	"github.com/macaroni-os/anise/pkg/config"
	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	"github.com/macaroni-os/anise/pkg/v2/tree"
)

type RuntimeSolverOpts struct {
	IgnoreConflicts bool
	Force           bool
	NoDeps          bool
}

type RuntimeSolverType int

type RuntimeTreeSolver interface {
	SetForestGuard(g *tree.ForestGuard)
	GetForestGuard() *tree.ForestGuard
	Resolve(pkg *[]*pkg.DefaultPackage) (*artifact.ArtifactsPack, error)
	ResolvePackage(pkg *pkg.DefaultPackage) (*artifact.ArtifactsPack, error)
	ResolvePackageTask(ptask *PackageTask) (*artifact.ArtifactsPack, error)
}

func NewRuntimeSolverOpts() *RuntimeSolverOpts {
	return &RuntimeSolverOpts{
		IgnoreConflicts: false,
		Force:           false,
		NoDeps:          false,
	}
}

func NewRuntimeSolverImplementation(
	stype string,
	cfg *config.AniseConfig,
	opts *RuntimeSolverOpts) *RuntimeTreeSolver {
	var s RuntimeTreeSolver

	switch stype {
	default:
		s = NewRuntimeSolver(cfg, opts)
	}

	return &s
}
