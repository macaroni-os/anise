/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package cmd_tree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"

	"github.com/geaaru/pkgs-checker/pkg/gentoo"
	helpers "github.com/macaroni-os/anise/anise-build/cmd/helpers"
	. "github.com/macaroni-os/anise/pkg/config"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"
	tree "github.com/macaroni-os/anise/pkg/v2/tree"

	"github.com/spf13/cobra"
	"golang.org/x/sync/semaphore"
)

type ValidateTask struct {
	TreeIdx    *tree.TreeIdx
	TreeIdxPkg *tree.TreeIdxPkg
	Pn         string

	waitGroup *sync.WaitGroup
	ctx       *context.Context
	semaphore *semaphore.Weighted

	Error error
}

type ValidateOpts struct {
	WithSolver    bool
	OnlyRuntime   bool
	OnlyBuildtime bool
	RegExcludes   []*regexp.Regexp
	RegMatches    []*regexp.Regexp
	Excludes      []string
	Matches       []string

	ForestGuard *tree.ForestGuard

	Mutex      sync.Mutex
	BrokenPkgs int
	BrokenDeps int
	Packages   int

	Errors []error
}

func (o *ValidateOpts) IncrBrokenPkgs() {
	o.Mutex.Lock()
	defer o.Mutex.Unlock()
	o.BrokenPkgs++
}

func (o *ValidateOpts) IncrBrokenDeps() {
	o.Mutex.Lock()
	defer o.Mutex.Unlock()
	o.BrokenDeps++
}

func (o *ValidateOpts) IncrPackages() {
	o.Mutex.Lock()
	defer o.Mutex.Unlock()
	o.Packages++
}

func (o *ValidateOpts) AddError(err error) {
	o.Mutex.Lock()
	defer o.Mutex.Unlock()
	o.Errors = append(o.Errors, err)
}

func validatePackage(task *ValidateTask, opts *ValidateOpts,
	ch chan ValidateTask) {

	validpkg := true
	var err, lastError error
	var pruntime *pkg.DefaultPackage

	defer task.waitGroup.Done()

	defFile := filepath.Join(task.TreeIdx.TreePath,
		task.TreeIdx.BaseDir,
		task.TreeIdxPkg.Path,
	)
	pkgstr := fmt.Sprintf("%s-%s", task.Pn, task.TreeIdxPkg.Version)
	gpkg, _ := gentoo.ParsePackageStr(pkgstr)

	if len(opts.Matches) > 0 {
		matched := false
		for _, rgx := range opts.RegMatches {
			if rgx.MatchString(pkgstr) {
				matched = true
				break
			}
		}

		if !matched {
			task.semaphore.Release(1)
			ch <- ValidateTask{
				Pn:         task.Pn,
				TreeIdx:    task.TreeIdx,
				TreeIdxPkg: task.TreeIdxPkg,
			}
			return
		}
	}

	opts.IncrPackages()

	if len(opts.Excludes) > 0 {
		excluded := false
		for _, rgx := range opts.RegExcludes {
			if rgx.MatchString(pkgstr) {
				excluded = true
				break
			}
		}

		if excluded {
			task.semaphore.Release(1)
			ch <- ValidateTask{
				Pn:         task.Pn,
				TreeIdx:    task.TreeIdx,
				TreeIdxPkg: task.TreeIdxPkg,
			}
			return
		}
	}

	if task.TreeIdxPkg.IsCollection() {

		// Load collection.yaml and retrieve package data.
		collection, err := tree.ReadCollectionFile(defFile)
		if err != nil {
			task.semaphore.Release(1)
			ch <- ValidateTask{
				Pn:         task.Pn,
				TreeIdx:    task.TreeIdx,
				TreeIdxPkg: task.TreeIdxPkg,
				Error: fmt.Errorf(
					"Error on read collection.yaml %s: %s",
					defFile, err.Error()),
			}
			return
		}

		pruntime, err = collection.GetPackageFromGentooPkg(gpkg)
		if err != nil {
			task.semaphore.Release(1)
			ch <- ValidateTask{
				Pn:         task.Pn,
				TreeIdx:    task.TreeIdx,
				TreeIdxPkg: task.TreeIdxPkg,
				Error: fmt.Errorf(
					"Error on retrieve package %s from %s: %s",
					gpkg.GetPVR(), defFile,
					err.Error()),
			}
			return
		}

	} else {

		// Load definition.yaml
		pruntime, err = tree.ReadDefinitionFile(defFile)
		if err != nil {
			task.semaphore.Release(1)
			ch <- ValidateTask{
				Pn:         task.Pn,
				TreeIdx:    task.TreeIdx,
				TreeIdxPkg: task.TreeIdxPkg,
				Error: fmt.Errorf(
					"Error on read definition.yaml %s: %s",
					defFile, err.Error()),
			}
			return
		}

	}

	// Check if all runtime dependencies are present in the tree
	numRuntimeDeps := len(pruntime.GetRequires())

	if numRuntimeDeps > 0 {

		for _, dep := range pruntime.GetRequires() {

			depOk := false
			trees, _ := opts.ForestGuard.SearchPackage(dep)

			for _, ti := range trees {
				versions, _ := ti.GetPackageVersions(dep.PackageName())

				for _, ver := range versions {
					pkg2check := &pkg.DefaultPackage{
						Name:     pruntime.Name,
						Category: pruntime.Category,
						Version:  ver.Version,
					}

					valid, err := pruntime.Admit(pkg2check)
					if err != nil {
						lastError = err
						break
					}
					if valid {
						depOk = true
						// POST: This version could be used from the package.
						break
					}

				}

				if depOk {
					break
				}
			}

			if !depOk {
				// Check if the dependency is a provides
				provides, _ := opts.ForestGuard.SearchProvides(dep)

				for _, ti := range provides {
					provs, _ := ti.GetPackageProvides(dep.PackageName())

					for _, ver := range provs {

						gprov, _ := gentoo.ParsePackageStr(
							fmt.Sprintf("%s-%s", ver.PkgName, ver.PkgVersion))

						depWithProvides := &pkg.DefaultPackage{
							Name:     gprov.GetPN(),
							Category: gprov.Category,
							Version:  ver.PkgVersion,
						}

						trees, _ := opts.ForestGuard.SearchPackage(depWithProvides)

						if len(trees) == 0 {
							continue
						}

						valid, err := pruntime.Admit(depWithProvides)
						if err != nil {
							lastError = err
							break
						}
						if valid {
							depOk = true
							// POST: This version could be used from the package.
							break
						}

					}

					if depOk {
						break
					}
				}
			}

			if !depOk {

				opts.IncrBrokenDeps()

				lastError = fmt.Errorf(
					"[runtime] [%s] Dependency %s not found :fire:.",
					pruntime.HumanReadableString(),
					dep.HumanReadableString())

			} else {

				DebugC(fmt.Errorf(
					"[runtime] [%s] Dependency %s :heavy_check_mark:",
					pruntime.HumanReadableString(),
					dep.HumanReadableString()))
			}

		}

	}

	if lastError != nil {
		opts.IncrBrokenPkgs()
		task.semaphore.Release(1)
		ch <- ValidateTask{
			Pn:         task.Pn,
			TreeIdx:    task.TreeIdx,
			TreeIdxPkg: task.TreeIdxPkg,
			Error:      lastError,
		}
		return
	}

	task.semaphore.Release(1)
	if !validpkg {
		ch <- ValidateTask{
			Pn:         task.Pn,
			TreeIdx:    task.TreeIdx,
			TreeIdxPkg: task.TreeIdxPkg,
			Error:      lastError,
		}
	} else {
		ch <- ValidateTask{
			Pn:         task.Pn,
			TreeIdx:    task.TreeIdx,
			TreeIdxPkg: task.TreeIdxPkg,
		}
	}

}

func initOpts(config *AniseConfig, opts *ValidateOpts, onlyRuntime, onlyBuildtime,
	withSolver bool, treePaths []string) {

	var err error

	opts.OnlyBuildtime = onlyBuildtime
	opts.OnlyRuntime = onlyRuntime
	opts.WithSolver = withSolver
	opts.BrokenPkgs = 0
	opts.BrokenDeps = 0

	// Load the index file
	opts.ForestGuard = tree.NewForestGuard(config)

	for _, treePath := range treePaths {
		Info(fmt.Sprintf(":deciduous_tree: Loading %s...", treePath))
		// Load singular path for time to improve user messages
		tIdx := tree.NewTreeIdx(treePath, true)

		if tIdx.HasIndex() {
			err = tIdx.Read(treePath)
		} else {
			Warning("Tree without index. Run `anise-build tree genidx`. Trying to generate indexes in memory")
			err = tIdx.Generate(treePath,
				&tree.GenOpts{
					DryRun:   false,
					OnlyMain: true,
				})
		}
		if err != nil {
			Fatal(err.Error())
		}

		opts.ForestGuard.Trees = append(opts.ForestGuard.Trees, tIdx)
	}

	opts.RegExcludes, err = helpers.CreateRegexArray(opts.Excludes)
	if err != nil {
		Fatal(err.Error())
	}
	opts.RegMatches, err = helpers.CreateRegexArray(opts.Matches)
	if err != nil {
		Fatal(err.Error())
	}

}

func NewTreeValidateCommand(config *AniseConfig) *cobra.Command {
	var excludes []string
	var matches []string
	var treePaths []string
	var opts ValidateOpts

	var ans = &cobra.Command{
		Use:   "validate [OPTIONS]",
		Short: "Validate a tree or a list of packages",
		Args:  cobra.OnlyValidArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			onlyRuntime, _ := cmd.Flags().GetBool("only-runtime")
			onlyBuildtime, _ := cmd.Flags().GetBool("only-buildtime")

			if len(treePaths) < 1 {
				Fatal("Mandatory tree param missing.")
			}
			if onlyRuntime && onlyBuildtime {
				Fatal("Both --only-runtime and --only-buildtime options are not possibile.")
			}
		},
		Run: func(cmd *cobra.Command, args []string) {

			withSolver, _ := cmd.Flags().GetBool("with-solver")
			onlyRuntime, _ := cmd.Flags().GetBool("only-runtime")
			onlyBuildtime, _ := cmd.Flags().GetBool("only-buildtime")

			opts.Excludes = excludes
			opts.Matches = matches
			initOpts(config, &opts, onlyRuntime, onlyBuildtime, withSolver, treePaths)

			channels := []chan ValidateTask{}

			ctx := context.TODO()
			var wg = new(sync.WaitGroup)
			semaphore := semaphore.NewWeighted(
				int64(config.GetGeneral().Concurrency),
			)

			nPkgs := 0
			for _, tree := range opts.ForestGuard.GetTrees() {

				for pn, treePkgs := range tree.Map {
					for _, pv := range treePkgs {
						task := &ValidateTask{
							Pn:         pn,
							TreeIdx:    tree,
							TreeIdxPkg: pv,
							waitGroup:  wg,
							ctx:        &ctx,
							semaphore:  semaphore,
						}

						channels = append(channels, make(chan ValidateTask))

						wg.Add(1)
						semaphore.Acquire(ctx, 1)

						go validatePackage(task, &opts, channels[nPkgs])
						nPkgs++
					}

				}
			}

			for i := 0; i < nPkgs; i++ {

				resp := <-channels[i]
				if resp.Error != nil {
					opts.AddError(resp.Error)
				}

			}
			wg.Wait()

			stringerrs := []string{}
			for _, e := range opts.Errors {
				stringerrs = append(stringerrs, e.Error())
			}
			sort.Strings(stringerrs)
			for _, e := range stringerrs {
				Error(e)
			}

			if len(stringerrs) != 0 {
				Fatal(fmt.Sprintf("Found %d broken packages and %d broken deps.",
					opts.BrokenPkgs, opts.BrokenDeps))
				Fatal("Errors: " + strconv.Itoa(len(stringerrs)))
			} else {
				Info(fmt.Sprintf(":white_check_mark: Elaborated %d packages. All good!",
					opts.Packages))
				os.Exit(0)
			}
		},
	}
	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}
	ans.Flags().Bool("only-runtime", false, "Check only runtime dependencies.")
	ans.Flags().Bool("only-buildtime", false, "Check only buildtime dependencies.")
	ans.Flags().BoolP("with-solver", "s", false,
		"Enable check of requires also with solver.")
	ans.Flags().StringSliceVarP(&treePaths, "tree", "t", []string{path},
		"Path of the tree to use.")
	ans.Flags().StringSliceVarP(&excludes, "exclude", "e", []string{},
		"Exclude matched packages from analysis. (Use string as regex).")
	ans.Flags().StringSliceVarP(&matches, "matches", "m", []string{},
		"Analyze only matched packages. (Use string as regex).")

	return ans
}
