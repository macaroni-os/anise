/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package solver

import (
	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	"github.com/macaroni-os/anise/pkg/v2/tree"
)

type PackageTask struct {
	Tree            *tree.TreeIdx
	Version         *tree.TreeIdxPkg
	PackageSelector *pkg.DefaultPackage
	Artifact        *artifact.PackageArtifact

	DepsSelectorsMap      map[string][]*DependencySelector
	ConflictsSelectorsMap map[string][]*DependencySelector

	availablesDepsMap *artifact.ArtifactsMap
	candidatesDepsMap *artifact.ArtifactsMap

	Solution *artifact.ArtifactsPack
}

type DependencySelector struct {
	Selector    *pkg.DefaultPackage
	TreeIndexes []*tree.TreeIdx
}

func NewPackageTask(tree *tree.TreeIdx, version *tree.TreeIdxPkg,
	selector *pkg.DefaultPackage) *PackageTask {

	return &PackageTask{
		Tree:                  tree,
		Version:               version,
		PackageSelector:       selector,
		DepsSelectorsMap:      make(map[string][]*DependencySelector, 0),
		ConflictsSelectorsMap: make(map[string][]*DependencySelector, 0),
		availablesDepsMap:     artifact.NewArtifactsMap(),
		candidatesDepsMap:     artifact.NewArtifactsMap(),
	}
}

func NewDependencySelector(s *pkg.DefaultPackage) *DependencySelector {
	return &DependencySelector{
		Selector:    s,
		TreeIndexes: []*tree.TreeIdx{},
	}
}

func NewDependencySelectorWithIdx(s *pkg.DefaultPackage, idx *[]*tree.TreeIdx) *DependencySelector {
	return &DependencySelector{
		Selector:    s,
		TreeIndexes: *idx,
	}
}

func (pt *PackageTask) AddDependency(s *DependencySelector) bool {
	selectors, present := pt.DepsSelectorsMap[s.Selector.PackageName()]
	if present {
		// POST: A selector with the same package name is already present.
		//       Check if the current selector is already present else
		//       I add it.
		for idx := range selectors {
			if selectors[idx].Selector.Version == s.Selector.Version {
				// POST: Dependency selector already present. Ignore it.
				return false
			}
		}

		selectors = append(selectors, s)
	} else {
		// POST: No existing selector with the selector package name are present.
		selectors = []*DependencySelector{s}
	}
	pt.DepsSelectorsMap[s.Selector.PackageName()] = selectors

	return true
}

func (pt *PackageTask) AddConflict(p *pkg.DefaultPackage) {
	selectors, present := pt.ConflictsSelectorsMap[p.PackageName()]
	if present {
		// POST: The conflicts with the selector package name are already present.
		//       Check if the current selector is already available and I can ignore it.
		for idx := range selectors {
			if selectors[idx].Selector.Version == p.Version {
				// POST: Conflict with the same selector already present.
				return
			}
		}

		selectors = append(selectors, NewDependencySelector(p))
	} else {
		// POST: No existing conflicts with the selector package name are present.
		selectors = []*DependencySelector{NewDependencySelector(p)}
	}
	pt.ConflictsSelectorsMap[p.PackageName()] = selectors
}
