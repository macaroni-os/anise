/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package solver

import (
	"fmt"
	"path/filepath"

	"github.com/macaroni-os/anise/pkg/config"
	"github.com/macaroni-os/anise/pkg/helpers"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/specs"
	"github.com/macaroni-os/anise/pkg/v2/render"
	"github.com/macaroni-os/anise/pkg/v2/tree"

	. "github.com/logrusorgru/aurora"
)

type BuildSolver struct {
	*BaseSolver

	RenderEngine *render.RenderEngine `yaml:"-" json:"-"`
	Opts         *BuildSolverOpts     `yaml:"-" json:"-"`
}

func NewBuildSolver(cfg *config.AniseConfig,
	opts *BuildSolverOpts) *BuildSolver {
	return &BuildSolver{
		BaseSolver: NewBaseSolver(cfg),
		Opts:       opts,
	}
}

func (s *BuildSolver) SetRenderEngine(re *render.RenderEngine) { s.RenderEngine = re }
func (s *BuildSolver) GetRenderEngine() *render.RenderEngine   { return s.RenderEngine }

func (s *BuildSolver) Resolve(pkgs *[]*pkg.DefaultPackage) (*artifact.ArtifactsPack, error) {
	ans := artifact.NewArtifactsPack()
	apMap := artifact.NewArtifactsMap()

	for _, p := range *pkgs {
		// Retrieve the packages selected and their dependencies
		singlePack, err := s.ResolvePackage(p)
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

func (s *BuildSolver) ResolvePackage(p *pkg.DefaultPackage) (*artifact.ArtifactsPack, error) {
	ans := artifact.NewArtifactsPack()
	apMap := artifact.NewArtifactsMap()
	vMap := make(map[string]bool, 0)

	InfoC(Bold(fmt.Sprintf(":eyes: Resolving selector...               %s",
		p.HumanReadableString())))

	// Retrieve all versions matched by selector
	indexes, err := s.ForestGuard.SearchPackage(p)
	if err != nil {
		return ans, err
	}

	if len(indexes) == 0 {
		return ans, fmt.Errorf("No candidates for selector %s found.",
			p.HumanReadableString())
	}

	for _, ti := range indexes {
		// POST: the indexes contains one or more TreeIdx and
		//       in every TreeIdx we have one or more versions

		versions, present := ti.GetPackageVersions(p.PackageName())
		if !present {
			return ans, fmt.Errorf(
				"unexpected error on retrieve versions for package %s and tree with basedir %s",
				p.PackageName(), ti.BaseDir)
		}

		for _, tv := range versions {

			if _, processed := vMap[tv.Version]; processed {
				// Avoid to elaborate the same version
				// multiple times. Using always the first.
				continue
			}
			vMap[tv.Version] = true

			DebugC(fmt.Sprintf(":construction: Creating package task for %s-%s",
				p.PackageName(), tv.Version))
			ptask := NewPackageTask(ti, tv, p)
			pack, err := s.ResolvePackageTask(ptask)
			if err != nil {
				return ans, err
			}

			// Add only packages not yet injected
			for _, part := range pack.Artifacts {
				_, missed := apMap.MatchVersion(part)
				if missed != nil {
					apMap.Add(part)
					ans.Add(part)
				} // else package is alreade present.
			}
		}
	}

	return ans, nil
}

func (s *BuildSolver) LoadCompilationSpec(
	t *tree.TreeIdx, vtree *tree.TreeIdxPkg, p *pkg.DefaultPackage) (*specs.CompilationSpecLoad, string, error) {

	var cs *specs.CompilationSpecLoad
	var err error

	// Using render engine to read build.yaml
	pkgPath := filepath.Join(t.TreePath, t.BaseDir, filepath.Dir(vtree.Path))

	defFile := filepath.Join(pkgPath, filepath.Base(vtree.Path))
	buildFile := filepath.Join(pkgPath, "build.yaml")

	DebugC(fmt.Sprintf(":brain:For %s-%s using buildfile:\t\t%s",
		p.PackageName(), vtree.Version, buildFile))

	DebugC(fmt.Sprintf(
		":brain:For %s-%s using package specs:\t\t%s",
		p.PackageName(), vtree.Version, defFile))

	if filepath.Base(defFile) == "collection.yaml" {

		atom := pkg.NewPackageWithCatThin(
			p.Category, p.Name,
			vtree.Version)

		cs, err = tree.ReadBuildFileFromCollection(buildFile, defFile,
			s.RenderEngine, atom, map[string]interface{}{})
	} else {
		cs, err = tree.ReadBuildFile(buildFile, defFile,
			s.RenderEngine, map[string]interface{}{})
	}
	if err != nil {
		return nil, "", fmt.Errorf(
			"error on rendering package %s-%s: %s",
			p.PackageName(), vtree.Version, err.Error())
	}

	return cs, pkgPath, nil
}

func (s *BuildSolver) ResolvePackageTask(ptask *PackageTask) (*artifact.ArtifactsPack, error) {
	// The stack array is used to catch dependencies cycles.
	stack := []string{}

	// Resolve recors
	return s.resolvePackage(ptask, stack)
}

func (s *BuildSolver) resolvePackage(ptask *PackageTask, stack []string) (*artifact.ArtifactsPack, error) {
	ans := artifact.NewArtifactsPack()

	if helpers.ContainsElem(&stack, ptask.PackageSelector.PackageName()) {
		// POST: this package is already been elaboratored. Stop dep cycle.
		return ans, nil
	}

	InfoC(fmt.Sprintf(":dart: :right_arrow: Elaborating package...           %s",
		Bold(fmt.Sprintf("%s-%s",
			ptask.PackageSelector.PackageName(), ptask.Version.Version))))

	stack = append(stack, ptask.PackageSelector.PackageName())

	cs, pkgPath, err := s.LoadCompilationSpec(ptask.Tree, ptask.Version, ptask.PackageSelector)
	if err != nil {
		return ans, err
	}

	// Stage1. Before elaborate all dependencies I try to retrieve all dependencies
	//         selectors to validate AND conditions.

	// Check if the package has conflicts
	if cs.DefaultPackage != nil && len(cs.DefaultPackage.GetConflicts()) > 0 {
		for _, conflict := range cs.DefaultPackage.GetConflicts() {

			if conflict.GetVersion() == "" {
				conflict.Version = ">=0"
			}

			ptask.AddConflict(conflict)
		}
	}

	// Check if the package has dependencies to recursively resolve them
	if cs.DefaultPackage != nil && len(cs.DefaultPackage.GetRequires()) > 0 {

		for _, dep := range cs.DefaultPackage.GetRequires() {

			if dep.GetVersion() == "" {
				dep.Version = ">=0"
			}

			DebugC(fmt.Sprintf(":satellite: [%s] Processing dependency selector %s ...",
				cs.DefaultPackage.HumanReadableString(), dep.HumanReadableString()))

			// Retrieve all available packages of the selected dependency
			// The packages not admitted by the selector are dropped from
			// the list.
			reqIdx, err := s.ForestGuard.SearchPackage(dep)
			if err != nil {
				return ans, err
			}

			// Fragments and sort all availables version. (it drops duplicates too).
			// Sort in reverse order (newest before old).
			reqIdx = *tree.FragmentTrees(&reqIdx, dep.PackageName(), true)

			ptask.AddDependency(NewDependencySelectorWithIdx(dep, &reqIdx))

			// Iterate for every version available of the analyzed dependencies. I will
			// add informations in the availablesDepsMap of the package task.
			// This phase wants retrieve and load the packages metadata of all
			// dependencies and versions available. The identification of the
			// build order of these dependencies is done later.

			for _, tidx := range reqIdx {

				// NOTE: Every TreeIdx contains only one version
				versions, _ := tidx.GetPackageVersions(dep.PackageName())

				deps, err := s.recursiveLoadDep(ptask, tidx, versions[0], dep, stack)
				if err != nil {
					return ans, err
				}

				for idx := range deps.Artifacts {
					ptask.availablesDepsMap.Add(deps.Artifacts[idx])
				}
			}

		}

	}

	if cs.Copy != nil && len(cs.Copy) > 0 {

		for _, c := range cs.Copy {
			dep := c.Package
			if dep == nil {
				continue
			}

			if dep.GetVersion() == "" {
				dep.Version = ">=0"
			}

			DebugC(fmt.Sprintf(":satellite: [%s] Processing copy dependency selector %s ...",
				cs.DefaultPackage.HumanReadableString(), dep.HumanReadableString()))

			// Retrieve all available packages of the selected dependency
			// The packages not admitted by the selector are dropped from
			// the list.
			reqIdx, err := s.ForestGuard.SearchPackage(dep)
			if err != nil {
				return ans, err
			}

			// Fragments and sort all availables version. (it drops duplicates too).
			// Sort in reverse order (newest before old).
			reqIdx = *tree.FragmentTrees(&reqIdx, dep.PackageName(), true)

			// Iterate for every version available of the analyzed dependencies. I will
			// add informations in the availablesDepsMap of the package task.
			// This phase wants retrieve and load the packages metadata of all
			// dependencies and versions available. The identification of the
			// build order of these dependencies is done later.

			for _, tidx := range reqIdx {

				// NOTE: Every TreeIdx contains only one version
				versions, _ := tidx.GetPackageVersions(dep.PackageName())

				deps, err := s.recursiveLoadDep(ptask, tidx, versions[0], dep, stack)
				if err != nil {
					return ans, err
				}

				for idx := range deps.Artifacts {
					ptask.availablesDepsMap.Add(deps.Artifacts[idx])
				}
			}

		}

	}

	// Create Package artifact with path sets to the home directory
	// for the build.
	ptask.Artifact = artifact.NewPackageArtifact(pkgPath)
	ptask.Artifact.CompileSpec = cs.ToSpec()
	ptask.Artifact.ToGenerate = true

	// Stage2: resolve recursively candidates using availables artifacts
	if len(ptask.availablesDepsMap.Artifacts) > 0 {
		err := s.prepareCandidates(ptask)
		if err != nil {
			return ans, err
		}
	}

	// Setup selected dependencies on final Artifact object.
	if len(ptask.candidatesDepsMap.Artifacts) > 0 {
		// Build the package thin array
		pthinarr := []*pkg.PackageThin{}

		for _, art := range ptask.candidatesDepsMap.Artifacts {
			pt := art[0].GetPackage().ToPackageThin()
			pthinarr = append(pthinarr, pt)
		}

		err := s.sortPkgsThinArr(&pthinarr)
		if err != nil {
			return nil, err
		}

		for idx := range pthinarr {
			arts, _ := ptask.candidatesDepsMap.GetArtifactsByKey(pthinarr[idx].PackageName())

			ans.Add(arts[0])
		}

	} // else no dependencies for building.

	// Add at the end of the list the package to build
	ans.Add(ptask.Artifact)

	ptask.candidatesDepsMap = nil
	ptask.availablesDepsMap = nil
	ptask.Solution = ans

	return ans, nil
}

func (s *BuildSolver) recursiveLoadDep(ptask *PackageTask,
	t *tree.TreeIdx, vtree *tree.TreeIdxPkg,
	selector *pkg.DefaultPackage, stack []string) (*artifact.ArtifactsPack, error) {

	ans := artifact.NewArtifactsPack()

	if helpers.ContainsElem(&stack, selector.PackageName()) {
		// POST: this package is already been elaboratored. Stop dep cycle.
		return ans, nil
	}

	InfoC(fmt.Sprintf(":dart:    :chains: Elaborating dependency...     %s",
		fmt.Sprintf("%s-%s", selector.PackageName(), vtree.Version)))

	stack = append(stack, selector.PackageName())

	cs, pkgPath, err := s.LoadCompilationSpec(t, vtree, selector)
	if err != nil {
		return ans, err
	}

	artDep := artifact.NewPackageArtifact(pkgPath)
	artDep.CompileSpec = cs.ToSpec()

	ans.Add(artDep)

	// Check if the package has dependencies to recursively resolve them
	if cs.DefaultPackage != nil && len(cs.DefaultPackage.GetRequires()) > 0 {

		for _, dep := range cs.DefaultPackage.GetRequires() {

			if dep.GetVersion() == "" {
				dep.Version = ">=0"
			}

			DebugC(fmt.Sprintf(":satellite: [%s] Processing dependency %s ...",
				cs.DefaultPackage.HumanReadableString(), dep.HumanReadableString()))

			// Retrieve all available packages of the selected dependency
			// The packages not admitted by the selector are dropped from
			// the list.
			reqIdx, err := s.ForestGuard.SearchPackage(dep)
			if err != nil {
				return ans, err
			}

			// Fragments and sort all availables version. (it drops duplicates too).
			// Sort in reverse order (newest before old).
			reqIdx = *tree.FragmentTrees(&reqIdx, dep.PackageName(), true)

			for _, tidx := range reqIdx {

				// NOTE: Every TreeIdx contains only one version
				versions, _ := tidx.GetPackageVersions(dep.PackageName())

				deps, err := s.recursiveLoadDep(ptask, tidx, versions[0], dep, stack)
				if err != nil {
					return ans, err
				}

				if len(deps.Artifacts) > 0 {
					ans.AppendPack(deps)
				}
			}

		}

	}

	if cs.Copy != nil && len(cs.Copy) > 0 {

		for _, c := range cs.Copy {
			dep := c.Package
			if dep == nil {
				// Ignore copy from image.
				continue
			}

			if dep.GetVersion() == "" {
				dep.Version = ">=0"
			}

			DebugC(fmt.Sprintf(":satellite: [%s] Processing copy dependency selector %s ...",
				cs.DefaultPackage.HumanReadableString(), dep.HumanReadableString()))

			// Retrieve all available packages of the selected dependency
			// The packages not admitted by the selector are dropped from
			// the list.
			reqIdx, err := s.ForestGuard.SearchPackage(dep)
			if err != nil {
				return ans, err
			}

			// Fragments and sort all availables version. (it drops duplicates too).
			// Sort in reverse order (newest before old).
			reqIdx = *tree.FragmentTrees(&reqIdx, dep.PackageName(), true)

			// Iterate for every version available of the analyzed dependencies. I will
			// add informations in the availablesDepsMap of the package task.
			// This phase wants retrieve and load the packages metadata of all
			// dependencies and versions available. The identification of the
			// build order of these dependencies is done later.

			for _, tidx := range reqIdx {

				// NOTE: Every TreeIdx contains only one version
				versions, _ := tidx.GetPackageVersions(dep.PackageName())

				deps, err := s.recursiveLoadDep(ptask, tidx, versions[0], dep, stack)
				if err != nil {
					return ans, err
				}

				if len(deps.Artifacts) > 0 {
					ans.AppendPack(deps)
				}
			}

		}

	}

	return ans, nil
}
