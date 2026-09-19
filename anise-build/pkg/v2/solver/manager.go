/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package solver

import (
	"fmt"

	"github.com/macaroni-os/anise/anise-build/pkg/v2/backend"
	cfg "github.com/macaroni-os/anise/pkg/config"
	fhelpers "github.com/macaroni-os/anise/pkg/helpers/file"
	. "github.com/macaroni-os/anise/pkg/logger"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/options"
	"github.com/macaroni-os/anise/pkg/v2/render"
	"github.com/macaroni-os/anise/pkg/v2/tree"

	. "github.com/logrusorgru/aurora"
)

type BuildManager struct {
	Config *cfg.AniseConfig

	Solver BuilderSolver
}

func NewBuildManager(c *cfg.AniseConfig) *BuildManager {
	return &BuildManager{
		Config: c,
		Solver: nil,
	}
}

func (bm *BuildManager) GetConfig() *cfg.AniseConfig { return bm.Config }
func (bm *BuildManager) GetSolver() BuilderSolver    { return bm.Solver }

func (bm *BuildManager) PrepareSolver(stype string,
	opts *BuildSolverOpts,
	tpaths []string,
	templatesDirs []string,
	renderValues []string) error {

	s := NewBuildSolverImplementation(stype, bm.Config, opts)
	bm.Solver = *s

	// Load the index file
	forestGuard := tree.NewForestGuard(bm.Config)
	err := forestGuard.LoadTrees(tpaths)
	if err != nil {
		return fmt.Errorf("error on read tree indexes: %s", err.Error())
	}

	// Setup the forest guard for the solver
	bm.Solver.SetForestGuard(forestGuard)

	// Creating render engine for build
	rEngine := render.NewRenderEngine(bm.Config)
	err = rEngine.LoadTemplates(templatesDirs)
	if err != nil {
		return fmt.Errorf("fail to load render templates dirs: %s", err.Error())
	}

	err = rEngine.LoadDefaultValues(renderValues)
	if err != nil {
		return fmt.Errorf("fail to load render default values: %s", err.Error())
	}

	bm.Solver.SetRenderEngine(rEngine)

	return nil
}

func (bm *BuildManager) BuildPretend(selectors []string) (*artifact.ArtifactsPack, error) {
	ans := artifact.NewArtifactsPack()
	apMap := artifact.NewArtifactsMap()

	for _, selector := range selectors {
		// Resolve the selector string in DefaultPackage object
		pkgSelector, err := bm.Solver.GetForestGuard().ResolveSelector(selector)
		if err != nil {
			return ans, err
		}

		// Retrieve the packages selected and their dependencies
		singlePack, err := bm.Solver.ResolvePackage(pkgSelector)
		if err != nil {
			return ans, err
		}

		// Add only packages not yet injected
		for _, part := range singlePack.Artifacts {
			_, missed := apMap.MatchVersion(part)
			if missed != nil {
				apMap.Add(part)
				ans.Add(part)
			} // else package is alreade present.
		}

	}

	return ans, nil
}

func (bm *BuildManager) Build(
	selectors []string,
	destination string,
	opts *options.Compiler) (*artifact.ArtifactsPack, error) {

	ans := artifact.NewArtifactsPack()
	tasksMap := make(map[string]*PackageTask, 0)
	tasksKeys := []string{}

	// We need / at the end to have EnsureDir works correctly.
	if destination[len(destination)-1:] != "/" {
		destination += "/"
	}

	// Retrieve the packages candidates for build (without deps) from
	// selectors
	for _, selector := range selectors {
		// Resolve the selector string in DefaultPackage object
		pkgSelector, err := bm.Solver.GetForestGuard().ResolveSelector(
			selector,
		)
		if err != nil {
			return ans, err
		}

		InfoC(Bold(fmt.Sprintf(":eyes: Resolving selector...               %s",
			pkgSelector.HumanReadableString())))

		// Retrieve all the matched versions
		indexes, err := bm.Solver.GetForestGuard().SearchPackage(
			pkgSelector,
		)
		if err != nil {
			return ans, err
		}
		if len(indexes) == 0 {
			return ans, fmt.Errorf("No candidates for selector %s found.",
				pkgSelector.HumanReadableString())
		}

		vMap := make(map[string]bool, 0)

		for _, ti := range indexes {
			// POST: the indexes contains one or more TreeIdx and
			//       in every TreeIdx we have one or more versions

			versions, present := ti.GetPackageVersions(
				pkgSelector.PackageName())
			if !present {
				return ans, fmt.Errorf(
					"unexpected error on retrieve vesrions for package %s and tree with basedir %s",
					pkgSelector.PackageName(), ti.BaseDir)
			}

			for _, tv := range versions {

				if _, processed := vMap[tv.Version]; processed {
					// Avoid to elaborate the same version
					// multiple times. Using always the first.
					continue
				}
				vMap[tv.Version] = true

				DebugC(fmt.Sprintf(":construction: Creating package task for %s-%s",
					pkgSelector.PackageName(), tv.Version))
				ptask := NewPackageTask(ti, tv, pkgSelector)
				_, err := bm.Solver.ResolvePackageTask(ptask)
				if err != nil {
					return ans, err
				}

				key := fmt.Sprintf("%s-%s", pkgSelector.PackageName(),
					tv.Version,
				)

				tasksMap[key] = ptask
				tasksKeys = append(tasksKeys, key)

			}

		}

	}

	// Validate destination address
	err := fhelpers.EnsureDir(destination)
	if err != nil {
		return ans, err
	}

	elaboratedPkgsMap := artifact.NewArtifactsMap()

	for _, pkgstr := range tasksKeys {

		ptask, _ := tasksMap[pkgstr]

		err = bm.buildPackageTask(ptask, ptask.Solution,
			elaboratedPkgsMap, opts,
			destination,
		)

		if err != nil {
			return ans, err
		}

	}

	ans.Artifacts = *elaboratedPkgsMap.ToList()

	return ans, nil
}

func (bm *BuildManager) buildPackageTask(
	ptask *PackageTask,
	solution *artifact.ArtifactsPack,
	elaboratedPkgsMap *artifact.ArtifactsMap,
	opts *options.Compiler, dst string) error {

	// Create the backend bridge
	bridge := backend.NewBackendBridge(bm.Config, opts)

	for idx, p := range solution.Artifacts {
		elabp, _ := elaboratedPkgsMap.MatchVersion(p)
		if elabp != nil {
			// POST: package already elaborated. Skipping.
			continue
		}

		generatePackage := !opts.PackageTargetOnly
		if !generatePackage {
			// Set a true for final target package
			if ptask.Artifact.GetPackage().AtomMatches(p.GetPackage()) {
				generatePackage = true
			}
		}

		err := bridge.BuildArtifact(dst, p, solution, generatePackage, idx+1)
		if err != nil {
			return err
		}

		elaboratedPkgsMap.Add(p)
	}

	return nil
}
