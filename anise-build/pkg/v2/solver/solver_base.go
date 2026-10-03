/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package solver

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	"github.com/macaroni-os/anise/pkg/v2/tree"
)

type BaseSolver struct {
	Config *config.AniseConfig `yaml:",inline" json:",inline"`

	ForestGuard *tree.ForestGuard `yaml:"-" json:"-"`

	CacheMap *artifact.ArtifactsMap `yaml:"-" json:"-"`

	mutex *sync.Mutex `yaml:"-" json:"-"`
}

func NewBaseSolver(cfg *config.AniseConfig) *BaseSolver {
	return &BaseSolver{
		Config:   cfg,
		CacheMap: artifact.NewArtifactsMap(),
		mutex:    &sync.Mutex{},
	}
}

func (s *BaseSolver) ClearCache() {
	s.CacheMap = artifact.NewArtifactsMap()
}

func (s *BaseSolver) GetForestGuard() *tree.ForestGuard { return s.ForestGuard }
func (s *BaseSolver) SetForestGuard(g *tree.ForestGuard) {
	s.ForestGuard = g
}

func (s *BaseSolver) artefactAdmitByCandidates(ptask *PackageTask,
	art *artifact.PackageArtifact) (bool, error) {

	if len(ptask.candidatesDepsMap.Artifacts) > 0 {
		for k, _ := range ptask.candidatesDepsMap.Artifacts {
			candidate := ptask.candidatesDepsMap.Artifacts[k][0]

			admit, err := candidate.GetPackage().Admit(art.GetPackage())
			if err != nil {
				return admit, err
			} else if !admit {
				Debug(fmt.Sprintf("%s NOT admits %s...",
					candidate.GetPackage().HumanReadableString(),
					art.GetPackage().HumanReadableString()))
				return admit, err
			}
		}
	}

	return true, nil
}

func (s *BaseSolver) sortPkgsThinArr(refarr *[]*pkg.PackageThin) error {
	ans := []*pkg.PackageThin{}
	pinject := make(map[string]bool, 0)
	queue := make(map[string]*pkg.PackageThin, 0)

	pkg.SortPackageThinList4Requires(refarr)

	start := time.Now()
	for _, p := range *refarr {

		injected := false

		if !p.HasRequires() {
			ans = append(ans, p)
			pinject[p.PackageName()] = true
			injected = true

		} else {
			allReqok := true

			for _, r := range p.Requires {
				if _, ok := pinject[r.PackageName()]; !ok {
					allReqok = false
					break
				}
			}

			if allReqok {
				ans = append(ans, p)
				pinject[p.PackageName()] = true
				injected = true

			} else {
				queue[p.PackageName()] = p
			}

		}

		if injected {
			// POST: check if the elements in queue
			//       could be injected.

			pkgs2remove := []string{}
			for k, pr := range queue {

				allReqok := true
				for _, r := range pr.Requires {
					if _, ok := pinject[r.PackageName()]; !ok {
						allReqok = false
						break
					}
				}

				if allReqok {
					ans = append(ans, pr)
					pinject[pr.PackageName()] = true
					pkgs2remove = append(pkgs2remove, k)
				}

			}

			for _, rm := range pkgs2remove {
				delete(queue, rm)
			}
		}

	} // end for
	Debug(fmt.Sprintf("First sort iteration done in %d µs. Queue size is %d",
		time.Now().Sub(start).Nanoseconds()/1e3, len(queue)))

	if len(queue) > 0 {
		// TODO: review with a more optimized logic

		loopCyclesDetectorCounter := 0
		lastQueueSize := 0
		detectorExecuted := false

		for len(queue) > 0 {
			if len(queue) == lastQueueSize {
				if loopCyclesDetectorCounter > 3 {
					if detectorExecuted {
						return fmt.Errorf(
							"Unexpected error on sort queue of size %d",
							len(queue))
					}
					detectorExecuted = true

					err := s.depCycleDetector(&queue, &pinject)
					if err != nil {
						return err
					}

				} else {
					loopCyclesDetectorCounter++
				}
			} else {
				lastQueueSize = len(queue)
				loopCyclesDetectorCounter = 0
			}

			pkgs2remove := []string{}
			for k, p := range queue {

				allReqok := true
				for _, r := range p.Requires {
					if _, ok := pinject[r.PackageName()]; !ok {
						// Check if there are dependency cycles.
						// I prefer leave this check only here to reduce
						// the impact of a wrong order.
						rr, _ := queue[r.PackageName()]
						if rr != nil && rr.RequirePackage(p) {
							Warning(fmt.Sprintf(
								"Found dependency cycle between %s and %s. I break cycle. Check the repo.",
								p.HumanReadableString(), r.HumanReadableString()))
							allReqok = true
						} else {
							Debug(fmt.Sprintf("[%s] The dependency %s has deps: %s",
								p.HumanReadableString(), r.HumanReadableString(), r.Requires))
							allReqok = false
						}
						break
					} else {
						Debug(fmt.Sprintf("[%s] Dependency %s already injected.", p.HumanReadableString(),
							r.PackageName()))
					}
				}

				if allReqok {
					ans = append(ans, p)
					pinject[p.PackageName()] = true
					pkgs2remove = append(pkgs2remove, k)
				}

			}

			for _, rm := range pkgs2remove {
				delete(queue, rm)
			}

		}
	}

	*refarr = ans

	return nil
}

func (s *BaseSolver) prepareCandidates(ptask *PackageTask) error {

	var art *artifact.PackageArtifact = nil

	// Sort keys to ensure reproducible build order
	keys := ptask.availablesDepsMap.GetKeys()
	sort.Strings(keys)

	for _, pn := range keys {
		validVersion := false
		versions, err := ptask.availablesDepsMap.GetSortedArtifactsByKey(pn)
		if err != nil {
			return err
		}

		for idx := range versions {
			art = versions[idx]

			// Check if the artefact is admitted from the package to build
			admit, err := ptask.Artifact.GetPackage().Admit(art.GetPackage())
			if err != nil {
				return err
			}
			if !admit {
				continue
			}

			// Check if the artefact is admitted from selected candidates
			admit, err = s.artefactAdmitByCandidates(ptask, art)
			if err != nil {
				return err
			}

			if admit {
				validVersion = true
			}

		}

		if !validVersion {
			return fmt.Errorf("No valid candidates found for dependency %s", pn)
		}

		ptask.candidatesDepsMap.Add(art)
	}

	return nil
}

func (s *BaseSolver) depCycleDetector(queueRef *map[string]*pkg.PackageThin, pinjectRef *map[string]bool) error {
	elems := []*pkg.PackageThin{}
	queue := *queueRef
	pinject := *pinjectRef

	// Convert map to list
	for _, p := range queue {
		elems = append(elems, p)
	}

	// Sort elements list
	pkg.SortPackageThinList4Requires(&elems)

	nelems := len(elems)
	for idx, p := range elems {
		stack := []*pkg.PackageThin{}
		Debug(fmt.Sprintf(
			"[%d of %d] Checking %s with %d requires...", idx+1, nelems, p.HumanReadableString(),
			len(p.Requires)))

		err := s.recursiveCheckDeps(p, &stack, queueRef, pinjectRef)
		if err != nil {
			return err
		}

		// Check that all dependencies are availables.
		for _, r := range p.Requires {
			_, injected := pinject[r.PackageName()]
			_, inQueue := queue[r.PackageName()]

			if !injected && !inQueue {
				return fmt.Errorf("[%s] Found dependency not resolvable %s",
					p.PackageName(), r.HumanReadableString())
			}
		}
	}

	return nil
}

func (s *BaseSolver) recursiveCheckDeps(p *pkg.PackageThin,
	stackRef *[]*pkg.PackageThin, queueRef *map[string]*pkg.PackageThin,
	pinjectRef *map[string]bool) error {

	Debug(fmt.Sprintf("[%s] with stack %s and requires %d", p, *stackRef, len(p.Requires)))
	if pkg.PackageThinIsInList(p, stackRef) {
		Warning(fmt.Sprintf(
			":ambulance: Found deps cycle for package %s: %s. Trying to break it. Please, fix specs or open an issue!!!",
			p.PackageName(), *stackRef))

		p.BreakCyclesOnRequires(stackRef)
	}
	*stackRef = append(*stackRef, p)
	queue := *queueRef
	pinject := *pinjectRef

	for _, r := range p.Requires {
		// The requires of the package doesn't contain the dependencies.
		// I need to use the queue element.

		rq, ok := queue[r.PackageName()]
		if !ok {
			// Check if the dependencies is already injected
			_, injected := pinject[r.PackageName()]

			if !injected {
				return fmt.Errorf("[%s] Error on retrieve requires %s on queue",
					p.HumanReadableString(), r.PackageName())
			}
			// POST: Ignoring this dependency
			Debug(fmt.Sprintf("[%s] Dependency %s already injected. Ignoring.",
				p.HumanReadableString(), r.HumanReadableString()))
			continue
		}

		err := s.recursiveCheckDeps(rq, stackRef, queueRef, pinjectRef)
		if err != nil {
			return err
		}
	}

	return nil

}
