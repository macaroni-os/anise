/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package installer

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	cfg "github.com/macaroni-os/anise/pkg/config"
	"github.com/macaroni-os/anise/pkg/helpers"
	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"
	artifact "github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	repos "github.com/macaroni-os/anise/pkg/v2/repository"

	"github.com/pkg/errors"
)

const (
	FinalizerFile = "finalize.yaml"
)

type ArtifactsManager struct {
	Config   *cfg.AniseConfig
	Database pkg.PackageDatabase

	sync.Mutex

	// Temporary struct to review.
	// This requires a lot of RAM when the elaborated
	// packages are with a lot of files. Maybe the map
	// could be created on filesystem in the near future?
	fileIndex map[string]*pkg.DefaultPackage
}

func NewArtifactsManager(config *cfg.AniseConfig) *ArtifactsManager {
	return &ArtifactsManager{
		Config:    config,
		Database:  nil,
		fileIndex: nil,
	}
}

func (m *ArtifactsManager) Setup() {
	if m.Database == nil {
		m.Database = m.Config.GetSystemDB()
	}
}

func (m *ArtifactsManager) Close() {
	if m.Database != nil {
		m.Database.Close()
	}
}

func (m *ArtifactsManager) DownloadPackage(p *artifact.PackageArtifact, r *repos.WagonRepository, msg string) error {
	c := r.Client()
	if c == nil {
		return errors.New("No client could be generated from repository")
	}

	err := c.DownloadArtifact(p, msg)
	if err != nil {
		return errors.Wrap(err, "Error on download artifact")
	}

	err = p.Verify()
	if err != nil {
		return errors.Wrap(err,
			"Artifact integrity check failure for file "+p.CachePath)
	}

	return nil
}

func (m *ArtifactsManager) loadFinalizer(f, defFile string, p *pkg.DefaultPackage) (*repos.AniseFinalizer, error) {

	out, err := helpers.RenderFiles(
		helpers.ChartFile(f),
		defFile,
	)
	if err != nil {
		Warning("Failed rendering finalizer for ",
			p.HumanReadableString(), err.Error())
		return nil, err
	}

	finalizer, err := repos.NewAniseFinalizerFromYaml([]byte(out))
	if err != nil {
		Warning("Failed reading finalizer for ",
			p.HumanReadableString(), err.Error())
		return nil, err
	}

	return finalizer, nil
}

func (m *ArtifactsManager) removePackageFiles(s *repos.Stone,
	targetRootfs string,
	preserveSystemEssentialData bool) error {

	var cp *cfg.ConfigProtect
	var err error
	var files []string = []string{}
	annotationDir := ""

	p := s.ToPackage()

	if len(s.Files) > 0 {
		files = s.Files
	} else {
		// Retrieve files from database
		files, err = m.Database.GetPackageFiles(p)
		if err != nil {
			return fmt.Errorf(
				"error on retrieving files list of the package %s: %s",
				s.HumanReadableString(),
				err)
		}
	}

	if !m.Config.ConfigProtectSkip {
		if p.HasAnnotation(string(pkg.ConfigProtectAnnnotation)) {
			dir, ok := p.GetAnnotations()[string(pkg.ConfigProtectAnnnotation)].(string)
			if ok {
				annotationDir = dir
			}
		}

		cp = cfg.NewConfigProtect(annotationDir)
		cp.Map(files)
	}

	toRemove, dirs2Remove, notPresent := fileHelper.OrderFiles(targetRootfs, files)

	mapDirs := make(map[string]int, 0)
	for _, d := range dirs2Remove {
		mapDirs[d] = 1
	}

	sort.Sort(sort.Reverse(sort.StringSlice(toRemove)))

	// Remove from target
	for _, f := range toRemove {
		target := filepath.Join(targetRootfs, f)

		if !m.Config.ConfigProtectSkip && cp.Protected(f) {
			Debug("Preserving protected file:", f)
			continue
		}

		Debug("Removing", target)
		if preserveSystemEssentialData &&
			(strings.HasPrefix(f, m.Config.GetSystem().GetSystemPkgsCacheDirPath()) ||
				strings.HasPrefix(f, m.Config.GetSystem().GetSystemRepoDatabaseDirPath())) {
			Warning("Preserve ", f,
				" which is required by anise ( you have to delete it manually if you really need to)")
			continue
		}

		fi, err := os.Lstat(target)
		if err != nil {
			Warning("File not found (it was before?)", err.Error())
			continue
		}
		switch mode := fi.Mode(); {
		case mode.IsDir():
			if mode&fs.ModeSymlink != 0 {
				Debug(fmt.Sprintf("Directory %s is a link. Ignoring.", target))
			} else {
				files, err := ioutil.ReadDir(target)
				if err != nil {
					Warning("Failed reading folder", target, err.Error())
				}
				if len(files) != 0 {
					Info("DROPPED = Preserving not-empty folder", target)
					continue
				}
			}
		}

		if err = os.Remove(target); err != nil {
			Warning("Failed removing file (maybe not present in the system target anymore ?)", target, err.Error())
		}

		// Add subpaths of the file to ensure that all dirs
		// are injected for the prune phase.
		dirname := filepath.Dir(target)
		words := strings.Split(dirname, string(os.PathSeparator))

		for i := len(words); i > 1; i-- {
			cpath := strings.Join(words[0:i], string(os.PathSeparator))
			if cpath == targetRootfs {
				// Avoid to add on mapDirs system directory outside
				// the rootfs when rootfs != /
				break
			}
			if _, ok := mapDirs[cpath]; !ok {
				mapDirs[cpath] = 1
			}
		}
	}

	// TODO: Check why this?? Not present means errors on stats,
	//       and this could be related to a file that doesn't exist
	//       or not installed because related to a subsets.
	//       Why try to remove it?
	for _, f := range notPresent {
		target := filepath.Join(targetRootfs, f)

		if !m.Config.ConfigProtectSkip && cp.Protected(f) {
			Debug("Preserving protected file:", f)
			continue
		}

		if err = os.Remove(target); err != nil {
			Debug("Failed removing file (not present in the system target)", target, err.Error())
		}
	}

	// Sorting the dirs from the mapDirs keys
	dirs2Remove = []string{}
	for k, _ := range mapDirs {
		dirs2Remove = append(dirs2Remove, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dirs2Remove)))

	Debug("Directories tagged for the check and remove", len(dirs2Remove))

	// Check if directories could be removed.
	for _, f := range dirs2Remove {
		f := filepath.Join(targetRootfs, f)

		if preserveSystemEssentialData &&
			strings.HasPrefix(f, m.Config.GetSystem().GetSystemPkgsCacheDirPath()) ||
			strings.HasPrefix(f, m.Config.GetSystem().GetSystemRepoDatabaseDirPath()) {
			Warning("Preserve ", f,
				" which is required by anise ( you have to delete it manually if you really need to)")
			continue
		}

		if !m.Config.ConfigProtectSkip && cp.Protected(f) {
			Debug("Preserving protected file:", f)
			continue
		}

		// Check if the directory is a link to avoid error with broken links.
		fi, err := os.Lstat(f)
		if err == nil {
			if fi.Mode()&fs.ModeSymlink == 0 {
				if fi.IsDir() {
					files, err := ioutil.ReadDir(f)
					if err != nil {
						Warning("Failed reading folder", f, err.Error())
					}
					Debug("Removing dir", f, "if empty: files ", len(files), ".")

					if len(files) != 0 {
						Debug("Preserving not-empty folder", f)
						continue
					}
				}
			} else {
				Debug(fmt.Sprintf("Directory %s is a link to remove.", f))
			}
		}
		// else remove the file in any case. I consider the file a broken link or
		// a file that doesn't exist.

		if err = os.Remove(f); err != nil {
			Debug("Failed removing file (not present in the system target)", f, err.Error())
		}
	}

	return nil
}

func (m *ArtifactsManager) RemovePackage(s *repos.Stone,
	targetRootfs string,
	preserveSystemEssentialData,
	skipFinalizer bool,
	force bool) error {

	m.Setup()

	err := m.removePackageFiles(s, targetRootfs, preserveSystemEssentialData)
	if err != nil && !force {
		return err
	}

	p := s.ToPackage()

	err = m.Database.RemovePackageFiles(p)
	if err != nil && !force {
		return errors.Wrap(err, "Failed removing package files from database")
	}

	if !skipFinalizer {
		pf, err := m.Database.GetPackageFinalizer(p)
		if err != nil && !force {
			return errors.Wrap(err, "Error on retrieve package finalizer")
		}
		if pf != nil {

			// TODO: check if return the object insted of run uninstall
			finalizer := &repos.AniseFinalizer{
				Shell:     pf.Shell,
				Uninstall: pf.Uninstall,
			}

			err = finalizer.RunUninstall(targetRootfs)
			if err != nil && !force {
				Warning("Failed running finalizer for ",
					p.HumanReadableString(), err.Error())
				return err
			}
		}
	}

	err = m.Database.RemovePackageFinalizer(p)
	if err != nil && !force {
		return errors.Wrap(err, "Failed removing package finalizer from database")
	}
	err = m.Database.RemovePackage(p)
	if err != nil && !force {
		return errors.Wrap(err, "Failed removing package from database")
	}

	return nil
}

func (m *ArtifactsManager) ReinstallPackage(
	s *repos.Stone,
	p *artifact.PackageArtifact,
	r *repos.WagonRepository,
	targetRootfs string,
	preserveSystemEssentialData bool,
	force bool) error {

	if p.Runtime == nil {
		return errors.New("Artifact without Runtime package definition")
	}

	m.Setup()

	err := m.removePackageFiles(s, targetRootfs, preserveSystemEssentialData)
	if err != nil && !force {
		return err
	}

	if force {
		pkg := s.ToPackage()
		// With force i reinstall also database files
		m.Database.RemovePackageFiles(pkg)
		m.Database.RemovePackageFinalizer(pkg)
		m.Database.RemovePackage(pkg)

		err = m.RegisterPackage(p, r, force)
		if err != nil {
			return err
		}
	}

	return m.InstallPackage(p, r, targetRootfs)
}

func (m *ArtifactsManager) InstallPackage(p *artifact.PackageArtifact, r *repos.WagonRepository, targetRootfs string) error {

	if p.Runtime == nil {
		return errors.New("Artifact without Runtime package definition")
	}

	start := time.Now()

	// PRE: the package is already downloaded and available in cache
	//      directory.

	// TODO: Check if it's needed an option to disable the
	//       second argument related to subsets feature.
	//       For now i enable it always.
	err := p.Unpack(targetRootfs, true)
	if err != nil {
		return errors.Wrap(err,
			fmt.Sprintf("Unpack package %s failed", p.Runtime.HumanReadableString()))
	}

	Debug(fmt.Sprintf("Unpack package %s completed in %d µs.",
		p.Runtime.HumanReadableString(),
		time.Now().Sub(start).Nanoseconds()/1e3))

	return nil
}

func (m *ArtifactsManager) RegisterPackage(p *artifact.PackageArtifact, r *repos.WagonRepository, force bool) error {

	pp := p.GetPackage()
	if pp == nil {
		return errors.New("Artifact without Runtime package definition")
	}

	start := time.Now()

	m.Setup()

	// Set package files on local database
	err := m.Database.SetPackageFiles(
		&pkg.PackageFile{
			PackageFingerprint: pp.GetFingerPrint(),
			Files:              p.Files,
		},
	)

	if err != nil && !force {
		return errors.Wrap(err, "Register package files on database")
	}

	Debug(fmt.Sprintf("Register package (set files ) %s completed in %d µs.",
		pp.HumanReadableString(),
		time.Now().Sub(start).Nanoseconds()/1e3))

	// Set finalizer if present
	if r != nil {
		// NOTE: r is nil when the package is sync from anise-portage-converter.

		repoTreefs := r.GetTreePath(m.Config.GetSystem().GetSystemReposDirPath())
		pkgdir := p.GetPackageTreePath(repoTreefs)
		finalizeFile := filepath.Join(pkgdir, FinalizerFile)
		defFile := filepath.Join(pkgdir, pkg.PackageDefinitionFile)

		if fileHelper.Exists(finalizeFile) {
			finalizer, err := m.loadFinalizer(finalizeFile, defFile, pp)
			if err != nil {
				return err
			}

			err = m.Database.SetPackageFinalizer(
				&pkg.PackageFinalizer{
					PackageFingerprint: pp.GetFingerPrint(),
					Shell:              finalizer.Shell,
					Install:            finalizer.Install,
					Uninstall:          finalizer.Uninstall,
				},
			)
			if err != nil && !force {
				return errors.Wrap(err,
					fmt.Sprintf("Register package %s", pp.HumanReadableString()))
			}
		}
	}

	// NOTE: for now postpone the registration of the package
	//       on the database

	_, err = m.Database.CreatePackage(pp)
	if err != nil {
		return errors.Wrap(err, "Failed register package")
	}

	Debug(fmt.Sprintf("Register package %s completed in %d µs.",
		pp.HumanReadableString(),
		time.Now().Sub(start).Nanoseconds()/1e3))

	return nil
}

func (m *ArtifactsManager) CheckFileConflicts(
	toInstall *[]*artifact.PackageArtifact,
	toRemove *[]*artifact.PackageArtifact,
	checkSystem bool,
	safeCheck bool,
	targetRootfs string,
) error {

	m.Setup()

	Info(":guard:Checking for file conflicts...")
	start := time.Now()

	conflictsFound := false

	// This requires a lot of RAM when the elaborated
	// packages are with a lot of files. Maybe the map
	// could be created on filesystem in the near future?
	filesToInstall := make(map[string]string, 0)
	filesToRemove := make(map[string]string, 0)

	if checkSystem {
		for _, a := range *toRemove {
			pp := a.GetPackage()
			files := a.Files
			if len(files) == 0 {
				// Trying to retrieve the list of the files
				// from the database.
				files, _ = m.Database.GetPackageFiles(pp)
			}

			for _, f := range files {
				filesToRemove[f] = pp.HumanReadableString()
			}
		}
	}

	// NOTE: Instead of load in memory the list
	//       of the files of every installed package
	//       and consume high memory I do it only for
	//       the list of the packages to install.
	//       The check validate if a package file is present
	//       on target system or between the list of
	//       files of the packages to install.
	//       if a file is already present I will
	//       check if the file is also on fileRmIndex map.
	for _, a := range *toInstall {
		pp := a.GetPackage()

		for _, f := range a.Files {
			if pkg, ok := filesToInstall[f]; ok {
				if safeCheck {
					Warning(fmt.Errorf(
						"file %s conflict between package %s and %s",
						f, pkg, pp.HumanReadableString(),
					))
					conflictsFound = true
				} else {
					return fmt.Errorf(
						"file %s conflict between package %s and %s",
						f, pkg, pp.HumanReadableString(),
					)
				}
			}

			filesToInstall[f] = pp.HumanReadableString()

			if checkSystem {
				tFile := filepath.Join(targetRootfs, f)

				// Check if the file is present on the target path.
				if fileHelper.Exists(tFile) {

					// Check if the file is in the list of the file to remove
					if _, ok := filesToRemove[f]; !ok {
						exists, p, err := m.ExistsPackageFile(f)
						if err != nil {
							return errors.Wrap(err, "failed checking into system db")
						}
						if exists {
							if safeCheck {

								Warning(fmt.Errorf(
									"file conflict between '%s' and '%s' ( file: %s )",
									p.HumanReadableString(),
									pp.HumanReadableString(),
									f,
								))
								conflictsFound = true
							} else {
								return fmt.Errorf(
									"file conflict between '%s' and '%s' ( file: %s )",
									p.HumanReadableString(),
									pp.HumanReadableString(),
									f,
								)
							}
						}
					} // else ignoring file.
				}

			}

		} // end for a.Files

	} // end for toInstall

	m.Lock()
	defer m.Unlock()
	m.fileIndex = nil

	if conflictsFound {
		Info(fmt.Sprintf(
			":heavy_check_mark: Conflicts ignored (executed in %d µs).",
			time.Now().Sub(start).Nanoseconds()/1e3))
	} else {
		Info(fmt.Sprintf(
			":heavy_check_mark: No conflicts found (executed in %d µs).",
			time.Now().Sub(start).Nanoseconds()/1e3))
	}

	return nil
}

func (m *ArtifactsManager) ExecuteFinalizer(
	a *artifact.PackageArtifact,
	r *repos.WagonRepository,
	postInstall bool,
	targetRootfs string) error {

	repoTreefs := r.GetTreePath(m.Config.GetSystem().GetSystemReposDirPath())
	pkgdir := a.GetPackageTreePath(repoTreefs)
	finalizeFile := filepath.Join(pkgdir, FinalizerFile)
	defFile := filepath.Join(pkgdir, pkg.PackageDefinitionFile)

	if a.Runtime == nil && a.CompileSpec.Package == nil {
		return errors.New("Invalid artifact without Package metadata")
	}

	p := a.GetPackage()

	if fileHelper.Exists(finalizeFile) {

		finalizer, err := m.loadFinalizer(finalizeFile, defFile, p)
		if err != nil {
			return err
		}

		Info("Executing finalizer for " + p.HumanReadableString())
		if postInstall {
			err = finalizer.RunInstall(targetRootfs)
		} else {
			err = finalizer.RunUninstall(targetRootfs)
		}
		if err != nil {
			Warning("Failed running finalizer for ",
				p.HumanReadableString(), err.Error())
			return err
		}
	}

	return nil
}

// NOTE: These methods will be replaced soon
func (m *ArtifactsManager) ExistsPackageFile(file string) (bool, *pkg.DefaultPackage, error) {
	Debug("Checking if file ", file, "belongs to any package")
	m.buildIndexFiles()
	m.Lock()
	defer m.Unlock()
	if p, exists := m.fileIndex[file]; exists {
		Debug(file, "belongs already to", p.HumanReadableString())

		return exists, p, nil
	}
	Debug(file, "doesn't belong to any package")
	return false, nil, nil
}

func (m *ArtifactsManager) buildIndexFiles() {
	m.Lock()
	defer m.Unlock()

	Debug("Building index files...")
	start := time.Now()

	// Check if cache is empty or if it got modified
	if m.fileIndex == nil {

		// This requires a lot of RAM when the elaborated
		// packages are with a lot of files. Maybe the map
		// could be created on filesystem in the near future?
		m.fileIndex = make(map[string]*pkg.DefaultPackage)
		for _, p := range m.Database.World() {
			files, _ := m.Database.GetPackageFiles(p)
			for _, f := range files {
				m.fileIndex[f] = p.(*pkg.DefaultPackage)
			}
		}
	}

	Debug(fmt.Sprintf("Build index files completed in %d µs.",
		time.Now().Sub(start).Nanoseconds()/1e3))
}
