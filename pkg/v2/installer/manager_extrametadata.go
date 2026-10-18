/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package installer

import (
	"archive/tar"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/macaroni-os/anise/pkg/config"
	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/subsets"
	artifact "github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	wagon "github.com/macaroni-os/anise/pkg/v2/repository"

	"github.com/klauspost/compress/zstd"
	. "github.com/logrusorgru/aurora"
)

type CheckOpts struct {
	Ignore      bool
	Verbose     bool
	WithSubsets bool
	IgnoreRules []string
	ForceFetch  bool
}

type CheckResult struct {
	Package        *pkg.DefaultPackage `json:"package" yaml:"package"`
	CorruptedFiles []*CheckResultFile  `json:"corrupted_files,omitempty" yaml:"corrupted_files,omitempty"`
	Ok             bool                `json:"ok" yaml:"ok"`
	SkippedFiles   int64               `json:"skipped_files,omitempty" yaml:"skipped_files,omitempty"`
	AnalyzedFiles  int64               `json:"analyzed_files,omitempty" yaml:"analyzed_files,omitempty"`
}

type CheckResults struct {
	Packages  []*CheckResult `json:"packages" yaml:"packages"`
	Corrupted int64          `json:"corrupted,omitempty" yaml:"corrupted,omitempty"`
}

type CheckResultFile struct {
	Path  string `json:"path" yaml:"path"`
	Error string `json:"error" yaml:"error"`
}

func (m *ArtifactsManager) IntegrityCheckAll(opts *CheckOpts) (*CheckResults, error) {
	artefacts2Check := []*artifact.PackageArtifact{}

	m.Setup()

	// TODO: temporary load in memory all installed packages.
	systemPkgs := m.Database.World()

	// Retrieve artifacts of the installed packages
	for _, p := range systemPkgs {
		art := artifact.NewPackageArtifact("")
		art.Runtime = p.(*pkg.DefaultPackage)
		artefacts2Check = append(artefacts2Check, art)
	}

	return m.integrityCheckPackages(opts, &artefacts2Check)
}

func (m *ArtifactsManager) IntegrityCheck(opts *CheckOpts,
	packs ...*pkg.DefaultPackage) (*CheckResults, error) {

	artefacts2Check := []*artifact.PackageArtifact{}

	m.Setup()

	// TODO: temporary load in memory all installed packages.
	systemPkgs := m.Database.World()
	systemPkgsMap := systemPkgs.ToMap()
	spm := *systemPkgsMap

	// Create a provides map of the installed package.
	// (TODO: Fix this on database. where is used ProvidesDatabase in memory
	//        without a correct setup)
	provMap := make(map[string]*pkg.DefaultPackage, 0)
	for _, p := range systemPkgs {
		if p.HasProvides() {
			for _, pp := range p.GetProvides() {
				provMap[pp.PackageName()] = p.(*pkg.DefaultPackage)
			}
		}
	}

	// Retrieve artifacts of the installed packages
	for _, p := range packs {

		if sp, ok := spm[p.PackageName()]; !ok {
			// POST: Package not found. Search if it's
			//       provided.
			if prov, ok := provMap[p.PackageName()]; ok {
				art := artifact.NewPackageArtifact("")
				art.Runtime = prov
				artefacts2Check = append(artefacts2Check, art)
				continue
			}
		} else {
			// POST: package found

			art := artifact.NewPackageArtifact("")
			art.Runtime = sp
			artefacts2Check = append(artefacts2Check, art)
			continue
		}
		Warning(fmt.Sprintf("%s not found. Ignoring it.", p.PackageName()))
	}

	return m.integrityCheckPackages(opts, &artefacts2Check)
}

func (m *ArtifactsManager) integrityCheckPackages(opts *CheckOpts,
	artefacts2CheckRef *[]*artifact.PackageArtifact) (*CheckResults, error) {
	ans := &CheckResults{
		Packages: []*CheckResult{},
	}
	mapRepos := make(map[string]*wagon.WagonRepository, 0)
	artefacts2Check := *artefacts2CheckRef

	var wr *wagon.WagonRepository
	// Load extra metadata
	for idx := range artefacts2Check {

		repoName := artefacts2Check[idx].Runtime.GetRepository()

		// Skip validation of packages added with anise-portage-converter
		if repoName == "scm" {
			InfoC(Bold(Yellow(fmt.Sprintf(":warning: %-30s - %-25s - Package of scm repo could not be verified. Skipped.",
				artefacts2Check[idx].Runtime.PackageName(),
				artefacts2Check[idx].Runtime.GetVersion(),
			))))
			ans.Packages = append(ans.Packages,
				&CheckResult{
					Package: artefacts2Check[idx].Runtime,
					Ok:      false,
				})
			continue
		}

		// Retrieve WagonRepository object
		if _, ok := mapRepos[repoName]; !ok {
			repobasedir := m.Config.GetSystem().GetRepoDatabaseDirPath(repoName)
			repo, err := m.Config.GetSystemRepository(repoName)
			if err != nil {
				Error(
					fmt.Sprintf("Repository not found for artefact %s",
						artefacts2Check[idx].GetPackage().HumanReadableString()))
				continue
			}
			wr = wagon.NewWagonRepository(repo)
			err = wr.ReadWagonIdentify(repobasedir)
			if err != nil {
				Error("Error on read repository identity file: " + err.Error())
				continue
			}

			mapRepos[repoName] = wr
		} else {
			wr = mapRepos[repoName]
		}

		err := m.DownloadPackageExtraMetadata(
			artefacts2Check[idx], wr, opts.ForceFetch,
		)
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				InfoC(Bold(Yellow(fmt.Sprintf(":warning: %-30s - %-25s - metadata file for validation not found. Skipped.",
					artefacts2Check[idx].Runtime.PackageName(),
					artefacts2Check[idx].Runtime.GetVersion(),
				))))
				ans.Packages = append(ans.Packages,
					&CheckResult{
						Package: artefacts2Check[idx].Runtime,
						Ok:      false,
					})
				continue
			}
		}

		err = m.PackageLoadExtraMetadata(
			artefacts2Check[idx], wr,
		)
		if err != nil {
			Error(err)
			continue
		}

		// Check package files
		res, err := m.checkPackageFiles(opts, artefacts2Check[idx])
		if err != nil {
			Error(err)
			continue
		}

		ans.Packages = append(ans.Packages, res)

		if res.Ok {
			InfoC(Bold(fmt.Sprintf(":white_check_mark:%-30s - %-25s - Package ok.",
				artefacts2Check[idx].GetPackage().PackageName(),
				artefacts2Check[idx].GetPackage().GetVersion(),
			)))
		} else {
			ans.Corrupted++
			InfoC(Bold(Red((fmt.Sprintf(":fire:%-30s - %-25s - Package corrupted.",
				artefacts2Check[idx].GetPackage().PackageName(),
				artefacts2Check[idx].GetPackage().GetVersion(),
			)))))
		}

	}

	return ans, nil
}

func (m *ArtifactsManager) checkPackageFiles(
	opts *CheckOpts, p *artifact.PackageArtifact) (*CheckResult, error) {

	ans := &CheckResult{
		Package:        p.Runtime,
		CorruptedFiles: []*CheckResultFile{},
		Ok:             true,
	}

	if p.Metadata == nil || p.Metadata.Files == nil {
		return nil, fmt.Errorf("artifact %s without metadata")
	}

	if len(p.Metadata.Files) == 0 {
		return ans, nil
	}

	// Prepare subsets analyzer to exclude
	// files not installed
	enableSubsets := true
	analyzer, err := subsets.NewSubsetsAnalyzer(enableSubsets,
		p.GetSubsets())
	if err != nil {
		return ans, err
	}
	if len(opts.IgnoreRules) > 0 {
		err = analyzer.AddIgnoreRegexes(opts.IgnoreRules)
		if err != nil {
			return ans, err
		}
	}

	for _, fIdentity := range p.Metadata.Files {

		if fIdentity.Type == tar.TypeDir ||
			fIdentity.Type == tar.TypeLink ||
			fIdentity.Type == tar.TypeBlock ||
			fIdentity.Type == tar.TypeSymlink {
			// Ignore directories and block device file
			// Ignore symlink, hardlink for now.
			ans.SkippedFiles++
			continue
		}

		// Skip file not selected by subsets
		tocheck := analyzer.IsSelected(fIdentity.Name)
		if !tocheck {
			ans.SkippedFiles++
			continue
		}

		ans.AnalyzedFiles++

		// Generate hashes of the file
		fpath := filepath.Join(config.AniseCfg.GetSystem().Rootfs, fIdentity.Name)
		if !fileHelper.Exists(fpath) {
			if opts.Verbose {
				InfoC(fmt.Sprintf(":police_car_light: %s - File %s not present.",
					p.Runtime.HumanReadableString(), fpath,
				))
			}
			ans.CorruptedFiles = append(
				ans.CorruptedFiles,
				&CheckResultFile{
					Path:  fpath,
					Error: fmt.Sprintf("file not present"),
				})
			continue
		}

		hashes, err := fileHelper.GetFileHashes(fpath)
		if err != nil {
			return ans, err
		}

		// Compare hash blake2b
		if hashes.Blake2b() != fIdentity.Checksum.Blake2b {
			if opts.Verbose {
				InfoC(fmt.Sprintf(":police_car_light:%-30s - %-25s - File %s corrupted (%s vs %s)",
					p.Runtime.PackageName(), p.Runtime.GetVersion(), fpath,
					hashes.Blake2b(), fIdentity.Checksum.Blake2b,
				))
			}
			ans.CorruptedFiles = append(
				ans.CorruptedFiles,
				&CheckResultFile{
					Path: fpath,
					Error: fmt.Sprintf(
						"Altered/corrupted file with different blake2b hash (%s vs %s)",
						hashes.Blake2b(), fIdentity.Checksum.Blake2b,
					),
				})
			continue
		}

		if hashes.Sha512() != fIdentity.Checksum.Sha512 {
			if opts.Verbose {
				InfoC(fmt.Sprintf(":police_car_light:%-30s - %-25s - File %s corrupted (%s vs %s)",
					p.Runtime.PackageName(), p.Runtime.GetVersion(), fpath,
					hashes.Sha512(), fIdentity.Checksum.Sha512,
				))
			}
			ans.CorruptedFiles = append(
				ans.CorruptedFiles,
				&CheckResultFile{
					Path: fpath,
					Error: fmt.Sprintf(
						"Altered/corrupted file with different sha512 hash (%s vs %s)",
						hashes.Sha512(), fIdentity.Checksum.Sha512,
					),
				})
			continue
		}

		DebugC(fmt.Sprintf(":mag:%-30s - %-25s - File %s ok - :check_mark:",
			p.Runtime.PackageName(), p.Runtime.GetVersion(), fpath,
		))
	}

	if len(ans.CorruptedFiles) > 0 {
		ans.Ok = false
	}

	return ans, nil
}

func (m *ArtifactsManager) DownloadPackageExtraMetadata(
	p *artifact.PackageArtifact, w *wagon.WagonRepository, force bool) error {

	repobasedir := m.Config.GetSystem().GetRepoDatabaseDirPath(w.Identity.Name)
	extrafs := filepath.Join(repobasedir, "extras")

	extrametaFile := p.GetPackage().GetFingerPrint() +
		artifact.ExtraMetadataSuffix + ".zstd"
	repoExtrametaFile := filepath.Join(extrafs, extrametaFile)

	if fileHelper.Exists(repoExtrametaFile) {
		if force {
			err := os.RemoveAll(repoExtrametaFile)
			if err != nil {
				return fmt.Errorf("error on clean file %s", extrametaFile)
			}
		} else {
			return nil
		}
	}

	c := w.Client()
	if c == nil {
		return fmt.Errorf("no client could be generated from repository %s", w.Identity.Name)
	}

	file, err := c.DownloadFile(extrametaFile)
	if err != nil {
		return err
	}

	err = fileHelper.CopyFile(file, repoExtrametaFile)
	if err != nil {
		return err
	}

	return nil
}

func (m *ArtifactsManager) PackageLoadExtraMetadata(
	p *artifact.PackageArtifact, w *wagon.WagonRepository) error {

	repobasedir := m.Config.GetSystem().GetRepoDatabaseDirPath(w.Identity.Name)
	extrafs := filepath.Join(repobasedir, "extras")

	extrametaFile := p.GetPackage().GetFingerPrint() +
		artifact.ExtraMetadataSuffix + ".zstd"
	repoExtrametaFile := filepath.Join(extrafs, extrametaFile)

	if !fileHelper.Exists(repoExtrametaFile) {
		return fmt.Errorf("file %s not present", extrametaFile)
	}

	file, err := os.Open(repoExtrametaFile)
	if err != nil {
		return fmt.Errorf("cannot open " + repoExtrametaFile)
	}
	defer file.Close()

	bufferedReader := bufio.NewReader(file)
	r, err := zstd.NewReader(bufferedReader)
	if err != nil {
		return err
	}
	defer r.Close()

	var metadata artifact.ExtraMetadata
	if err := json.NewDecoder(r).Decode(&metadata); err != nil {
		return fmt.Errorf("error on decode extra metadata of package %s",
			p.GetPackage().HumanReadableString())
	}

	p.Metadata = &metadata

	return nil
}
