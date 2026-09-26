/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package repository

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/macaroni-os/anise/pkg/config"
	"github.com/macaroni-os/anise/pkg/helpers"
	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"
	artifact "github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	"github.com/macaroni-os/anise/pkg/v2/repository/mask"

	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
	"gopkg.in/yaml.v3"
)

type StonesSearchTask struct {
	waitGroup *sync.WaitGroup
	Ctx       *context.Context
	semaphore *semaphore.Weighted

	regs    []*regexp.Regexp
	lRegs   []*regexp.Regexp
	catRegs []*regexp.Regexp

	channels []chan ChannelSearchRes

	maskManager *mask.PackagesMaskManager
}

type StonesSearchOpts struct {
	Packages         pkg.DefaultPackages
	Categories       []string
	Names            []string
	Labels           []string
	LabelsMatches    []string
	Matches          []string
	FilesOwner       []string
	Annotations      []string
	Hidden           bool
	AndCondition     bool
	WithFiles        bool
	WithRootfsPrefix bool
	Full             bool
	OnlyPackages     bool
	IgnoreMasks      bool
}

type ArtifactIndex []*artifact.PackageArtifact

type StonesCatalog struct {
	Index ArtifactIndex `json:"index" yaml:"index"`
}

type Stone struct {
	Name        string                 `json:"name" yaml:"name"`
	Category    string                 `json:"category" yaml:"category"`
	Version     string                 `json:"version" yaml:"version"`
	License     string                 `json:"license,omitempty" yaml:"license,omitempty"`
	Repository  string                 `json:"repository,omitempty" yaml:"repository"`
	Hidden      bool                   `json:"hidden,omitempty" yaml:"hidden,omitempty"`
	Files       []string               `json:"files,omitempty" yaml:"files,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Labels      map[string]string      `json:"labels,omitempty" yaml:"labels,omitempty"`
	UseFlags    []string               `json:"use_flags,omitempty" yaml:"use_flags,omitempty"`

	Provides  []*Stone `json:"provides,omitempty" yaml:"provides,omitempty"`
	Requires  []*Stone `json:"requires,omitempty" yaml:"requires,omitempty"`
	Conflicts []*Stone `json:"conflicts,omitempty" yaml:"conflicts,omitempty"`
}

type StonesPack struct {
	Stones []*Stone `json:"stones" yaml:"stones"`
}

type StonesMap struct {
	Stones map[string][]*Stone `json:"stones" yaml:"stones"`
}

type WagonStones struct {
	Catalog *StonesCatalog
}

type ChannelSearchRes struct {
	Stones    *[]*Stone
	Artifacts *[]*artifact.PackageArtifact

	Error error
}

func (so *StonesSearchOpts) CloneWithPkgs(pkgs pkg.DefaultPackages) *StonesSearchOpts {
	ans := &StonesSearchOpts{
		Packages:         pkgs,
		Categories:       so.Categories,
		Labels:           so.Labels,
		LabelsMatches:    so.LabelsMatches,
		Matches:          so.Matches,
		FilesOwner:       so.FilesOwner,
		Annotations:      so.Annotations,
		Hidden:           so.Hidden,
		AndCondition:     so.AndCondition,
		WithFiles:        so.WithFiles,
		WithRootfsPrefix: so.WithRootfsPrefix,
		Full:             so.Full,
		OnlyPackages:     so.OnlyPackages,
		IgnoreMasks:      so.IgnoreMasks,
	}

	return ans
}

func (sp *StonesPack) ToMap() *StonesMap {

	ans := &StonesMap{
		Stones: make(map[string][]*Stone, 1),
	}

	for idx, _ := range sp.Stones {
		ans.Add(sp.Stones[idx])
	}

	return ans
}

func (sm *StonesMap) Add(s *Stone) {
	if val, ok := sm.Stones[s.GetName()]; ok {
		sm.Stones[s.GetName()] = append(val, s)
	} else {
		sm.Stones[s.GetName()] = []*Stone{s}
	}
}

func (sm *StonesMap) AddPack(sp *StonesPack) {
	for idx, _ := range sp.Stones {
		sm.Add(sp.Stones[idx])
	}
}

func NewWagonStones() *WagonStones {
	return &WagonStones{
		Catalog: nil,
	}
}

func NewStone(p *artifact.PackageArtifact, repo string, withFiles, full bool) *Stone {
	ans := &Stone{
		Name:        p.Runtime.Name,
		Category:    p.Runtime.Category,
		Version:     p.Runtime.Version,
		License:     p.Runtime.License,
		Repository:  repo,
		Hidden:      p.Runtime.Hidden,
		Annotations: p.Runtime.Annotations,
		Labels:      p.Runtime.Labels,
	}

	if withFiles {
		ans.Files = p.Files
	}

	if full && p.Runtime != nil {
		if len(p.Runtime.UseFlags) > 0 {
			ans.UseFlags = p.Runtime.UseFlags
		}

		if len(p.Runtime.Provides) > 0 {
			ans.Provides = []*Stone{}

			for idx, _ := range p.Runtime.Provides {
				ans.Provides = append(ans.Provides,
					&Stone{
						Name:     p.Runtime.Provides[idx].GetName(),
						Category: p.Runtime.Provides[idx].GetCategory(),
						Version:  p.Runtime.Provides[idx].GetVersion(),
					},
				)
			}
		}

		if len(p.Runtime.PackageRequires) > 0 {
			ans.Requires = []*Stone{}

			for idx, _ := range p.Runtime.PackageRequires {
				ans.Requires = append(ans.Requires,
					&Stone{
						Name:     p.Runtime.PackageRequires[idx].GetName(),
						Category: p.Runtime.PackageRequires[idx].GetCategory(),
						Version:  p.Runtime.PackageRequires[idx].GetVersion(),
					},
				)
			}
		}

		if len(p.Runtime.PackageConflicts) > 0 {
			ans.Conflicts = []*Stone{}

			for idx, _ := range p.Runtime.PackageConflicts {
				ans.Conflicts = append(ans.Conflicts,
					&Stone{
						Name:     p.Runtime.PackageConflicts[idx].GetName(),
						Category: p.Runtime.PackageConflicts[idx].GetCategory(),
						Version:  p.Runtime.PackageConflicts[idx].GetVersion(),
					},
				)
			}
		}
	}

	return ans
}

func (s *Stone) ToPackage() *pkg.DefaultPackage {
	ans := pkg.NewPackage(s.Name, s.Version, []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
	ans.Labels = s.Labels
	ans.Annotations = s.Annotations
	ans.Hidden = s.Hidden
	ans.Category = s.Category
	ans.License = s.License
	ans.Repository = s.Repository

	if s.Provides != nil && len(s.Provides) > 0 {
		ans.Provides = []*pkg.DefaultPackage{}

		for idx := range s.Provides {
			ans.Provides = append(ans.Provides, s.Provides[idx].ToPackage())
		}
	}

	if s.Requires != nil && len(s.Requires) > 0 {
		ans.PackageRequires = []*pkg.DefaultPackage{}

		for idx := range s.Requires {
			ans.PackageRequires = append(ans.PackageRequires, s.Requires[idx].ToPackage())
		}
	}

	if s.Conflicts != nil && len(s.Conflicts) > 0 {
		ans.PackageConflicts = []*pkg.DefaultPackage{}

		for idx := range s.Conflicts {
			ans.PackageConflicts = append(ans.PackageConflicts, s.Conflicts[idx].ToPackage())
		}
	}

	return ans
}

func (s *Stone) HumanReadableString() string {
	return fmt.Sprintf("%s/%s-%s", s.Category, s.Name, s.Version)
}

func (s *Stone) GetName() string {
	if s.Category != "" && s.Name != "" {
		return fmt.Sprintf("%s/%s", s.Category, s.Name)
	} else if s.Category != "" {
		return s.Category
	} else {
		return s.Name
	}
}

func (s *Stone) GetVersion() string {
	return s.Version
}

func (s *WagonStones) analyzeCatDir(
	categoryDir, categoryName string,
	task *StonesSearchTask,
	opts *StonesSearchOpts,
	ch chan ChannelSearchRes,
	repoName string) {

	ans := []*artifact.PackageArtifact{}

	defer task.waitGroup.Done()

	// Read packages directories
	pkgsDirs, err := ioutil.ReadDir(categoryDir)
	if err != nil {
		task.semaphore.Release(1)
		ch <- ChannelSearchRes{
			nil, nil,
			errors.New(
				fmt.Sprintf("Error on read directory %s: %s",
					categoryDir, err.Error())),
		}
		return
	}

	for _, pkgname := range pkgsDirs {
		if !pkgname.IsDir() {
			Debug(fmt.Sprintf("For repository %s ignoring file %s",
				repoName, pkgname.Name()))
			continue
		}

		if len(opts.Names) > 0 {
			match := false
			// POST: Check if the name of the package matches
			//       one of the names selected

			if helpers.ContainsElem(&opts.Names, pkgname.Name()) {
				match = true
			}

			if !match {
				continue
			}
		}

		if len(opts.Matches) > 0 && opts.AndCondition {
			pstring := fmt.Sprintf("%s/%s", categoryName, pkgname.Name())
			match := false

			for ri, _ := range task.regs {
				if task.regs[ri].MatchString(pstring) {
					match = true
					break
				}
			}

			if !match {
				continue
			}
		}

		pkgdir := filepath.Join(categoryDir, pkgname.Name())

		// Read packages directories
		versionsDirs, err := ioutil.ReadDir(pkgdir)
		if err != nil {
			task.semaphore.Release(1)
			ch <- ChannelSearchRes{
				nil, nil,
				errors.New(
					fmt.Sprintf("Error on read directory %s: %s",
						pkgdir, err.Error())),
			}
			return
		}

		for _, v := range versionsDirs {

			if !v.IsDir() {
				Debug(fmt.Sprintf("For repository %s ignoring file %s",
					repoName, v.Name()))
				continue
			}

			vdir := filepath.Join(pkgdir, v.Name())

			art, err := s.analyzePackageDir(
				vdir, task, opts, repoName,
			)
			if err != nil {
				task.semaphore.Release(1)
				ch <- ChannelSearchRes{
					nil, nil,
					errors.New(
						fmt.Sprintf("Error on analyze directory %s: %s",
							vdir, err.Error())),
				}
				return
			}

			if art != nil {
				ans = append(ans, art)
			}

		}

	}

	Debug(fmt.Sprintf("[%s] For category %s found %d artefacts.",
		repoName, categoryName, len(ans)))

	task.semaphore.Release(1)
	ch <- ChannelSearchRes{nil, &ans, nil}
}

func (s *WagonStones) analyzePackageDir(
	dir string,
	task *StonesSearchTask,
	opts *StonesSearchOpts,
	repoName string) (*artifact.PackageArtifact, error) {

	var art *artifact.PackageArtifact

	defFile := filepath.Join(dir, "definition.yaml")

	// Ignoring directory without definition.yaml file
	if !fileHelper.Exists(defFile) {
		return nil, nil
	}

	metaJsonFile := filepath.Join(dir, "metadata.json")

	if fileHelper.Exists(metaJsonFile) {

		data, err := ioutil.ReadFile(metaJsonFile)
		if err != nil {
			return nil, fmt.Errorf("Error on read file %s: %s",
				metaJsonFile, err.Error())
		}

		art, err = artifact.NewPackageArtifactFromJson(data)
		if err != nil {
			return nil, fmt.Errorf("Error on parse file %s: %s",
				metaJsonFile, err.Error())
		}
		// Free memory
		data = nil
	} else {
		// Read the metadata.file
		metaFile := filepath.Join(dir, "metadata.yaml")
		data, err := ioutil.ReadFile(metaFile)
		if err != nil {
			return nil, fmt.Errorf("Error on read file %s: %s",
				metaFile, err.Error())
		}

		art, err = artifact.NewPackageArtifactFromYaml(data)
		if err != nil {
			return nil, fmt.Errorf("Error on parse file %s: %s",
				metaFile, err.Error())
		}
		// Free memory
		data = nil
	}

	if art.Runtime == nil {
		//fmt.Println("ARTIFACT ", artifact, repoName)
		Warning(fmt.Sprintf(
			"[%s/%s-%s] Found artifact without runtime pkg. Using compile spec package.",
			art.CompileSpec.Package.Category,
			art.CompileSpec.Package.Name,
			art.CompileSpec.Package.Version,
		))

		art.Runtime = art.CompileSpec.Package
	}

	if !opts.IgnoreMasks {
		// POST: Check if the package is masked.
		g, err := art.GetPackage().ToGentooPackage()
		masked, err := task.maskManager.IsMasked(repoName, g)
		if err != nil {
			return nil, err
		} else if masked {
			Debug(fmt.Sprintf("[%s] Package %s masked.",
				repoName, art.GetPackage().HumanReadableString()))
			return nil, nil
		}
	}

	if !opts.Hidden && art.Runtime.Hidden {
		// Exclude hidden packages
		return nil, nil
	}

	match := false

	if len(opts.Names) > 0 {
		if helpers.ContainsElem(&opts.Names, art.Runtime.Name) {
			match = true
		}
	}

	if len(opts.Packages) > 0 {
		for idx, _ := range opts.Packages {

			// Check Provides
			if len(art.Runtime.Provides) > 0 {
				for _, prov := range art.Runtime.Provides {
					if prov.AtomMatches(opts.Packages[idx]) {
						match = true
						break
					}
				}
			}

			if art.Runtime.Category != opts.Packages[idx].GetCategory() {
				continue
			}

			if art.Runtime.Name != opts.Packages[idx].GetName() {
				continue
			}

			// NOTE: Ignore error here because the parsing
			//       is been already validate before.
			gS, _ := opts.Packages[idx].ToGentooPackage()
			gP, _ := art.GetPackage().ToGentooPackage()

			admit, err := gS.Admit(gP)
			if err != nil {
				return nil, fmt.Errorf(
					"Unexpected error on compare %s with %s: %s",
					opts.Packages[idx].HumanReadableString(),
					art.GetPackage().HumanReadableString(),
					err.Error())
			}

			if admit {
				match = true
				break
			}
		} // end for
	}

	if len(opts.Matches) > 0 {
		pstring := art.Runtime.PackageName()

		for ri, _ := range task.regs {
			if task.regs[ri].MatchString(pstring) {
				match = true
				break
			}
		}
	}

	if len(opts.LabelsMatches) > 0 {
		if opts.AndCondition {
			match = false
		} else {
			goto matched
		}

		if !match {
			for ri, _ := range task.lRegs {
				if art.Runtime.MatchLabel(task.lRegs[ri]) {
					match = true
					break
				}
			}
		}
	}

	if len(opts.Labels) > 0 {
		if opts.AndCondition {
			match = false
		} else {
			goto matched
		}

		if !match {
			for _, l := range opts.Labels {
				if art.Runtime.HasLabel(l) {
					match = true
					break
				}
			}
		}
	}

	if len(opts.Categories) > 0 {
		if opts.AndCondition {
			match = false
		} else {
			goto matched
		}

		if !match {
			for ri, _ := range task.catRegs {
				if task.catRegs[ri].MatchString(art.Runtime.Category) {
					match = true
					break
				}
			}
		}
	}

	if len(opts.Annotations) > 0 {
		if opts.AndCondition {
			match = false
		}

		for _, a := range opts.Annotations {
			if art.Runtime.HasAnnotation(a) {
				match = true
				break
			}
		}
	}

	if len(opts.FilesOwner) > 0 {
		if opts.AndCondition {
			match = false
		}

		if len(art.Files) > 0 {
			for _, f := range opts.FilesOwner {
				for fidx, _ := range art.Files {
					if strings.Index(art.Files[fidx], f) >= 0 {
						match = true
						goto matched
					}
				}
			}
		}
	}

matched:
	if match {
		// Propagate repository information
		if art.Runtime != nil {
			art.Runtime.Repository = repoName
		}

		if art.CompileSpec != nil && art.CompileSpec.Package != nil {
			art.CompileSpec.Package.Repository = repoName
		}

		return art, nil
	}

	return nil, nil
}

// The SearchArtifacts instead to read artifacts from memory (catalog)
// it tries to return all artifacts matching with the search
// options reading metadata from files under the treefs directory.
func (s *WagonStones) SearchArtifacts(
	opts *StonesSearchOpts, repoName, repoDir string,
	maskManager *mask.PackagesMaskManager) (*[]*artifact.PackageArtifact, error) {
	ans := []*artifact.PackageArtifact{}

	if len(opts.LabelsMatches) > 0 && len(opts.Matches) > 0 {
		return nil, errors.New("Searching for both regex and labels regex is not supported.")
	}

	ctx := context.TODO()
	task := &StonesSearchTask{
		waitGroup: &sync.WaitGroup{},
		Ctx:       &ctx,
		semaphore: semaphore.NewWeighted(
			int64(config.AniseCfg.GetGeneral().Concurrency),
		),

		regs:    []*regexp.Regexp{},
		lRegs:   []*regexp.Regexp{},
		catRegs: []*regexp.Regexp{},

		channels:    []chan ChannelSearchRes{},
		maskManager: maskManager,
	}
	catMap := make(map[string]bool, 0)

	// Create regexes array

	if len(opts.Matches) > 0 {
		for _, m := range opts.Matches {
			r := regexp.MustCompile(m)
			if r != nil {
				task.regs = append(task.regs, r)
			}
		}
	}

	if len(opts.LabelsMatches) > 0 {
		for _, m := range opts.LabelsMatches {
			r := regexp.MustCompile(m)
			if r != nil {
				task.lRegs = append(task.lRegs, r)
			}
		}
	}

	if len(opts.Categories) > 0 {
		for _, m := range opts.Categories {
			r := regexp.MustCompile(m)
			if r != nil {
				task.catRegs = append(task.catRegs, r)
			}
		}
	}

	start := time.Now()
	repoTreeDir := filepath.Join(repoDir, "treefs")

	files, err := ioutil.ReadDir(repoTreeDir)
	if err != nil {
		return &ans, errors.New(
			fmt.Sprintf("Error on read directory %s: %s",
				repoTreeDir, err.Error()))
	}

	nCategories := 0

	if opts.OnlyPackages {
		// Create the map of the categories researched.
		for _, p := range opts.Packages {
			catMap[p.Category] = true
		}
	}

	// NOTE: A repository directory is in this format
	//       <repo-dir>/<pkg-category>/<pkg-name>/<pkg-version>/
	for _, file := range files {

		if !file.IsDir() {

			if file.Name() != "provides.yaml" {
				Debug(fmt.Sprintf("For repository %s ignoring file %s",
					repoName, file.Name()))
			}
			continue
		}

		categoryDir := file.Name()

		// Skip categories directory directly if the filter is present.
		// If the andCondition is false means that other filter could
		// match packages
		if len(opts.Categories) > 0 && opts.AndCondition {

			match := false
			for ri, _ := range task.catRegs {
				if task.catRegs[ri].MatchString(categoryDir) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		if opts.OnlyPackages {
			if _, ok := catMap[categoryDir]; !ok {
				// POST: if the category is not used I skip directory
				//       parsing.
				continue
			}
		}

		// POST: category matched or no category filter available.
		catDirAbs := filepath.Join(repoTreeDir, categoryDir)

		task.waitGroup.Add(1)

		// Create the channel
		task.channels = append(task.channels, make(chan ChannelSearchRes))

		// Acquire sem
		err = task.semaphore.Acquire(*task.Ctx, 1)
		if err != nil {
			Error("Error on acquire sem " + err.Error())
		}

		go s.analyzeCatDir(catDirAbs, categoryDir, task, opts,
			task.channels[nCategories], repoName)
		nCategories++

	}

	for i := 0; i < nCategories; i++ {

		resp := <-task.channels[i]
		if resp.Error == nil {
			ans = append(ans, *resp.Artifacts...)
		} else {
			err = resp.Error
		}

	}

	task.waitGroup.Wait()

	if opts.OnlyPackages {
		// If OnlyPackages is used then check the provides file
		// directly.
		err := s.searchProvides(repoTreeDir, repoName, task, opts, &ans)
		if err != nil {
			return nil, err
		}

	}

	Debug(fmt.Sprintf("[%s] Search Artifacts in %d µs.",
		repoName,
		time.Now().Sub(start).Nanoseconds()/1e3),
	)
	return &ans, nil
}

func (s *WagonStones) searchProvides(repoTreeDir, repoName string,
	task *StonesSearchTask,
	opts *StonesSearchOpts,
	arts *[]*artifact.PackageArtifact) error {

	providesFile := filepath.Join(repoTreeDir, "provides.yaml")
	providers := NewWagonProvides()

	if fileHelper.Exists(providesFile) {
		err := providers.Load(providesFile)
		if err != nil {
			return err
		}
	}

	isInList := func(pkgstr string, aa *[]*artifact.PackageArtifact) bool {
		inList := false
		for _, p := range *aa {
			if p.GetPackage().HumanReadableString() == pkgstr {
				inList = true
				break
			}
		}
		return inList
	}

	if len(providers.Provides) > 0 {
		//
		ans := *arts

		for _, sp := range opts.Packages {
			for provname, provArts := range providers.Provides {
				if sp.PackageName() == provname {
					for _, p := range provArts {
						if isInList(p.HumanReadableString(), arts) {
							continue
						} else {
							// Analyze package dir
							pkgDir := filepath.Join(repoTreeDir,
								p.Category,
								p.Name,
								p.Version,
							)

							art, err := s.analyzePackageDir(
								pkgDir, task, opts, repoName,
							)
							if err != nil {
								return fmt.Errorf("Error on analyze directory %s: %s",
									pkgDir, err.Error())
							}

							if art != nil {
								ans = append(ans, art)
							}
						}
					}
				}
			}
		}

		*arts = ans
	}

	return nil
}

func (s *WagonStones) SearchArtifactsFromCatalog(
	opts *StonesSearchOpts, repoName string) (*[]*artifact.PackageArtifact, error) {
	ans := []*artifact.PackageArtifact{}

	if s.Catalog == nil {
		return &ans, nil
	}

	if len(opts.LabelsMatches) > 0 && len(opts.Matches) > 0 {
		return nil, errors.New("Searching for both regex and labels regex is not supported.")
	}

	// Create regexes array
	regs := []*regexp.Regexp{}
	lRegs := []*regexp.Regexp{}
	catRegs := []*regexp.Regexp{}

	if len(opts.Matches) > 0 {
		for _, m := range opts.Matches {
			r := regexp.MustCompile(m)
			if r != nil {
				regs = append(regs, r)
			}
		}
	}

	if len(opts.LabelsMatches) > 0 {
		for _, m := range opts.LabelsMatches {
			r := regexp.MustCompile(m)
			if r != nil {
				lRegs = append(lRegs, r)
			}
		}
	}

	if len(opts.Categories) > 0 {
		for _, m := range opts.Categories {
			r := regexp.MustCompile(m)
			if r != nil {
				catRegs = append(catRegs, r)
			}
		}
	}

	for idx, _ := range s.Catalog.Index {
		artifact := s.Catalog.Index[idx]
		if artifact.Runtime == nil {
			//fmt.Println("ARTIFACT ", artifact, repoName)
			Warning(fmt.Sprintf(
				"[%s/%s-%s] Found artifact without runtime pkg. Using compile spec package.",
				artifact.CompileSpec.Package.Category,
				artifact.CompileSpec.Package.Name,
				artifact.CompileSpec.Package.Version,
			))

			artifact.Runtime = artifact.CompileSpec.Package
		}
		if !opts.Hidden && artifact.Runtime.Hidden {
			// Exclude hidden packages
			continue
		}

		match := false

		if len(opts.Names) > 0 {
			if helpers.ContainsElem(&opts.Names, artifact.Runtime.Name) {
				match = true
			}
		}

		// For now only match category and name
		if len(opts.Packages) > 0 {
			for idx, _ := range opts.Packages {
				if artifact.Runtime.Category != opts.Packages[idx].GetCategory() {
					continue
				}

				if artifact.Runtime.Name != opts.Packages[idx].GetName() {
					continue
				}

				match = true
				break
			}
		}

		if len(opts.Matches) > 0 {
			pstring := artifact.Runtime.PackageName()

			for ri, _ := range regs {
				if regs[ri].MatchString(pstring) {
					match = true
					break
				}
			}
		}

		if len(opts.LabelsMatches) > 0 {
			if opts.AndCondition {
				match = false
			} else {
				goto matched
			}

			if !match {
				for ri, _ := range lRegs {
					if artifact.Runtime.MatchLabel(lRegs[ri]) {
						match = true
						break
					}
				}
			}
		}

		if len(opts.Labels) > 0 {
			if opts.AndCondition {
				match = false
			} else {
				goto matched
			}

			if !match {
				for _, l := range opts.Labels {
					if artifact.Runtime.HasLabel(l) {
						match = true
						break
					}
				}
			}
		}

		if len(opts.Categories) > 0 {
			if opts.AndCondition {
				match = false
			} else {
				goto matched
			}

			if !match {
				for ri, _ := range catRegs {
					if catRegs[ri].MatchString(artifact.Runtime.Category) {
						match = true
						break
					}
				}
			}
		}

		if len(opts.Annotations) > 0 {
			if opts.AndCondition {
				match = false
			}

			for _, a := range opts.Annotations {
				if artifact.Runtime.HasAnnotation(a) {
					match = true
					break
				}
			}
		}

		if len(opts.FilesOwner) > 0 {
			if opts.AndCondition {
				match = false
			}

			if len(artifact.Files) > 0 {
				for _, f := range opts.FilesOwner {
					for fidx, _ := range artifact.Files {
						if strings.Index(artifact.Files[fidx], f) >= 0 {
							match = true
							goto matched
						}
					}
				}
			}
		}

	matched:
		if match {
			// Propagate repository information
			if artifact.Runtime != nil {
				artifact.Runtime.Repository = repoName
			}

			if artifact.CompileSpec != nil && artifact.CompileSpec.Package != nil {
				artifact.CompileSpec.Package.Repository = repoName
			}

			ans = append(ans, artifact)
		}
	}

	return &ans, nil

}

func (s *WagonStones) SearchFromCatalog(opts *StonesSearchOpts, repoName string) (*[]*Stone, error) {
	ans := []*Stone{}

	artifactsMatched, err := s.SearchArtifactsFromCatalog(opts, repoName)
	if err != nil {
		return nil, err
	}

	matches := *artifactsMatched

	if len(matches) > 0 {
		for idx, _ := range matches {
			stone := NewStone(matches[idx], repoName, opts.WithFiles, opts.Full)

			if opts.WithRootfsPrefix {
				// TODO: Check how i could avoid to use the global config variable.
				files := []string{}
				for _, f := range stone.Files {
					files = append(files,
						filepath.Join(config.AniseCfg.GetSystem().Rootfs, f))
				}
				stone.Files = files
			}
			ans = append(ans, stone)
		}
	}

	return &ans, nil
}

func (s *WagonStones) Search(
	opts *StonesSearchOpts, repoName, repoDir string,
	m *mask.PackagesMaskManager,
) (*[]*Stone, error) {
	ans := []*Stone{}

	artifactsMatched, err := s.SearchArtifacts(opts, repoName, repoDir, m)
	if err != nil {
		return nil, err
	}

	matches := *artifactsMatched

	if len(matches) > 0 {
		for idx, _ := range matches {
			stone := NewStone(matches[idx], repoName, opts.WithFiles, opts.Full)
			if opts.WithRootfsPrefix {
				// TODO: Check how i could avoid to use the global config variable.
				files := []string{}
				for _, f := range stone.Files {
					files = append(files,
						filepath.Join(config.AniseCfg.GetSystem().Rootfs, f))
				}
				stone.Files = files
			}
			ans = append(ans, stone)

		}
	}

	return &ans, nil
}

func (s *WagonStones) LoadCatalog(identity *WagonIdentity) (*StonesCatalog, error) {
	ans := &StonesCatalog{}

	repobasedir := filepath.Dir(identity.IdentityFile)

	start := time.Now()

	// TODO: Here we need to handle the new repository style
	//       when ready.
	if _, ok := identity.RepositoryFiles[REPOFILE_META_KEY]; ok {
		metafs := filepath.Join(repobasedir, "metafs")
		metafile := filepath.Join(metafs, REPOSITORY_METAFILE)

		Debug(fmt.Sprintf("[%s] Found metafile %s", identity.Name, metafile))

		file, err := os.Open(metafile)
		if err != nil {
			return nil, errors.Wrap(err, "Error on reading file "+metafile)
		}
		defer file.Close()

		//decoder := yaml.NewDecoder(file)
		decoder := yaml.NewDecoder(bufio.NewReader(file))
		err = decoder.Decode(&ans)
		if err != nil {
			return nil, errors.Wrap(err, "Error on parse file "+metafile)
		}
		decoder = nil

	} else {
		return nil, errors.New("No meta field found. Repository is corrupted or to sync.")
	}

	s.Catalog = ans

	Debug(fmt.Sprintf("[%s] metadata loaded in %d µs.", identity.Name,
		time.Now().Sub(start).Nanoseconds()/1e3))

	return ans, nil
}
