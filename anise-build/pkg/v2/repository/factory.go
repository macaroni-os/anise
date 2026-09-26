/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package repository

import (
	"archive/tar"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	cfg "github.com/macaroni-os/anise/pkg/config"
	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"
	artifact "github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	wagon "github.com/macaroni-os/anise/pkg/v2/repository"
	"github.com/macaroni-os/anise/pkg/v2/tree"

	tarf "github.com/geaaru/tar-formers/pkg/executor"
	tarf_specs "github.com/geaaru/tar-formers/pkg/specs"
	tools "github.com/geaaru/tar-formers/pkg/tools"
	"github.com/pkg/errors"
)

// WagonFactoryOpts describe contains the
// all possible options to bump a repository
// revision.
type WagonFactoryOpts struct {
	ResetRevision bool

	OutputDir   string
	PackagesDir string

	// Validate package tarball
	CheckPackageTarball bool
	// Add compilertree tarball on bump.
	WithCompilerTree bool

	// Using same compression for all files
	CompressionMode tools.CompressionMode

	// Docker Repository is not yet supported
	// I trace options for that backend for now
	//	PushImage bool

	TreeFilename string
}

type WagonFactory struct {
	Config     *cfg.AniseConfig
	Repository *cfg.AniseRepository
	Provides   *wagon.WagonProvides

	mutex *sync.Mutex
}

func NewWagonFactoryOpts() *WagonFactoryOpts {
	return &WagonFactoryOpts{
		ResetRevision:       false,
		OutputDir:           "",
		PackagesDir:         "",
		CheckPackageTarball: false,
		WithCompilerTree:    false,
		CompressionMode:     tools.Zstd,
		TreeFilename:        wagon.TREE_TARBALL,
	}
}

func NewWagonFactory(config *cfg.AniseConfig, repo *cfg.AniseRepository) *WagonFactory {
	return &WagonFactory{
		Config:     config,
		Repository: repo,
		Provides:   wagon.NewWagonProvides(),
		mutex:      &sync.Mutex{},
	}
}

func (w *WagonFactory) createPackage(f string, idx *[]*tree.TreeIdx,
	opts *WagonFactoryOpts, treefsDir string) error {
	metaFile := filepath.Join(opts.PackagesDir, f)
	packageFilenamePrefix := filepath.Join(opts.PackagesDir,
		strings.TrimSuffix(f, ".metadata.yaml")+".package.tar")
	packageFilename := ""

	indexes := *idx

	// Read the metadata.yaml file
	data, err := os.ReadFile(metaFile)
	if err != nil {
		return fmt.Errorf("Error on read file %s: %s",
			f, err.Error())
	}

	art, err := artifact.NewPackageArtifactFromYaml(data)
	if err != nil {
		return fmt.Errorf("Error on parse file %s: %s",
			f, err.Error())
	}
	// Free memory
	data = nil

	if opts.CheckPackageTarball {
		// We need to support the change of the compression with
		// multiple compression at the same time.
		// Check the zst extension by first case.
		packageFilename = packageFilenamePrefix + string(tools.Zstd)

		if !fileHelper.Exists(packageFilename) {
			packageFilename = packageFilenamePrefix + string(tools.Gzip)

			if !fileHelper.Exists(packageFilename) {

				packageFilename = packageFilenamePrefix
				Warning(fmt.Sprintf(
					"[%s/%s-%s] Found metadata file %s without runtime pkg tarball. Ignoring it.",
					art.CompileSpec.Package.Category,
					art.CompileSpec.Package.Name,
					art.CompileSpec.Package.Version,
					f,
				))
				return nil
			}

		}
	}

	if art.Runtime == nil {
		Warning(fmt.Sprintf(
			"[%s/%s-%s] Found artifact without runtime pkg. Using compile spec package.",
			art.CompileSpec.Package.Category,
			art.CompileSpec.Package.Name,
			art.CompileSpec.Package.Version,
		))

		art.Runtime = art.CompileSpec.Package
	}

	pkgInTree := false
	definitionFile := ""
	for i, _ := range indexes {
		tipkg, ok := indexes[i].GetPackageVersion(
			art.GetPackage().PackageName(),
			art.GetPackage().GetVersion(),
		)
		if ok {
			pkgInTree = true
			definitionFile = filepath.Join(
				indexes[i].TreePath, indexes[i].BaseDir, tipkg.Path)
			break
		}
	}

	if !pkgInTree {
		Debug(fmt.Sprintf(
			"[%s] No more in the tree. Ignoring the package.",
			art.GetPackage().HumanReadableString()))
		return nil
	}

	Debug(fmt.Sprintf("[%s] Using definition file %s",
		art.GetPackage().HumanReadableString(), definitionFile))

	treePkgdir := fmt.Sprintf(
		"%s/%s/%s/%s",
		treefsDir, art.GetPackage().GetCategory(),
		art.GetPackage().GetName(),
		art.GetPackage().GetVersion(),
	)

	var dp *pkg.DefaultPackage
	if filepath.Base(definitionFile) == "collection.yaml" {
		coll, err := tree.ReadCollectionFile(definitionFile)
		if err != nil {
			return err
		}
		dp, err = coll.GetPackage(art.GetPackage().PackageName(),
			art.GetPackage().GetVersion())
		if err != nil {
			return err
		}
	} else {
		dp, err = tree.ReadDefinitionFile(definitionFile)
		if err != nil {
			return err
		}
	}

	// Check exists finalize.yaml
	finalizeFile := ""
	if fileHelper.Exists(filepath.Join(
		filepath.Dir(definitionFile), "finalize.yaml")) {
		finalizeFile = filepath.Join(
			filepath.Dir(definitionFile), "finalize.yaml")
	}

	// Users could update package definition.yaml without
	// the need to bump a new version. This method merge
	// definition.yaml values over the artefact structure.
	art.MergeDefinition(dp)

	// Creating treefs directory of the package
	w.mutex.Lock()
	if err = os.MkdirAll(treePkgdir, os.ModePerm); err != nil {
		w.mutex.Unlock()
		return err
	}

	if len(dp.Provides) > 0 {
		// Write provides on map
		for _, prov := range dp.Provides {
			w.Provides.Add(prov.PackageName(), art.GetPackage())
		}
	}

	w.mutex.Unlock()

	metaJsonFile := filepath.Join(treePkgdir, "metadata.json")
	err = art.WriteMetadataJson(metaJsonFile)
	if err != nil {
		return fmt.Errorf(
			"Error on create file %s: %s", metaJsonFile, err.Error())
	}

	treeDefFile := filepath.Join(treePkgdir, "definition.yaml")
	err = tree.WriteDefinitionFile(dp, treeDefFile)
	if err != nil {
		return fmt.Errorf(
			"Error on create file %s: %s", treeDefFile, err.Error())
	}

	if finalizeFile != "" {
		// Copy finalize
		err := fileHelper.CopyFile(finalizeFile,
			filepath.Join(treePkgdir, "finalize.yaml"))
		if err != nil {
			return fmt.Errorf(
				"Error on copy finalize of pkg %s: %s",
				dp.HumanReadableString(), err.Error())
		}
	}

	return nil
}

func (w *WagonFactory) createTreeFs(idx *[]*tree.TreeIdx,
	opts *WagonFactoryOpts, treefsDir string) error {
	var regexRepo = regexp.MustCompile(`.metadata.yaml$`)

	// Open packages dir
	dirEntries, err := os.ReadDir(opts.PackagesDir)
	if err != nil {
		return errors.New("Error on read dir " + opts.PackagesDir + ":" +
			err.Error())
	}

	for _, file := range dirEntries {
		if file.IsDir() {
			continue
		}

		if !regexRepo.MatchString(file.Name()) {
			Debug("File", file.Name(), "skipped.")
			continue
		}

		err = w.createPackage(file.Name(), idx, opts, treefsDir)
		if err != nil {
			return err
		}
	}

	// Create the provides.yaml file under treefs directory
	providesFile := filepath.Join(treefsDir, "provides.yaml")
	err = w.Provides.WriteProvidesYAML(providesFile)
	if err != nil {
		Error(fmt.Sprintf(
			"Error on creating provides file %s: %s",
			providesFile,
			err.Error()),
		)
		return err
	}

	return nil
}

func (w *WagonFactory) createCompilerTreeTarball(opts *WagonFactoryOpts,
	treePaths []string) (*wagon.WagonDocument, error) {
	// NOTE: For the compilertree using always zstd compression for now.
	tarball := filepath.Join(opts.OutputDir, "compilertree.tar.zst")
	document := wagon.NewWagonDocument("compilertree.tar.zst")

	// Prepare tar-formers specs
	s := tarf_specs.NewSpecFile()
	s.SameChtimes = true
	s.Writer = tarf_specs.NewWriter()
	// Ignore .git and build directory
	ignoreRegexp, _ := regexp.Compile("^.git|^build")

	archiveDirMap := make(map[string]bool, 0)
	for _, t := range treePaths {
		ctree := filepath.Join(t, "..")
		Debug(fmt.Sprintf("[%s] Using compiler tree %s",
			w.Repository.Name, ctree))
		// For compiler tree I get the upper directory of the tree
		archiveDirMap[ctree] = true
	}

	// Sort the directories with the the bigger first
	archiveDirs := []string{}
	for p := range archiveDirMap {
		archiveDirs = append(archiveDirs, p)
	}
	sort.Strings(archiveDirs)
	s.Writer.ArchiveDirs = archiveDirs

	// Prepare tarball creation
	topts := tools.NewTarCompressionOpts(true)
	defer topts.Close()
	topts.Mode = tools.GetCompressionMode(tarball)

	err := tools.PrepareTarWriter(tarball, topts)
	if err != nil {
		return nil, err
	}
	document.SetCompressionType(tools.Zstd)

	// Tarformers handler to drop tree fs directory prefix from
	// files to archive in the tarball.
	handler := func(path, newpath string,
		header *tar.Header, tw *tar.Writer,
		opts *tarf.TarFileOperation, t *tarf.TarFormers) error {

		// For all files/directory i need to drop the treefs path
		opts.Rename = true

		for _, s := range archiveDirs {
			if strings.HasPrefix(newpath, s) {
				opts.NewName, _ = filepath.Rel(s, newpath)
				if ignoreRegexp.MatchString(opts.NewName) {
					opts.Skip = true
				}
				break
			}
		}

		return nil
	}

	tarformers := tarf.NewTarFormers(tarf.GetOptimusPrime().Config)
	if topts.CompressWriter != nil {
		tarformers.SetWriter(topts.CompressWriter)

	} else {
		tarformers.SetWriter(topts.FileWriter)
	}
	tarformers.SetFileWriterHandler(handler)

	err = tarformers.RunTaskWriter(s)
	if err != nil {
		return nil, err
	}
	// We need to close file else the sha is not elaborated
	// correctly.
	topts.Close()

	// Generate sha of the tarball
	tarballSha, err := fileHelper.Sha256Sum(tarball)
	if err != nil {
		return nil, err
	}

	document.Checksums[string(artifact.SHA256)] = tarballSha

	Debug(fmt.Sprintf("[%s] Generated %s with sha256 %s",
		w.Repository.Name, tarball, tarballSha))

	return document, nil
}

func (w *WagonFactory) createTreeTarball(opts *WagonFactoryOpts,
	treefsDir string) (*wagon.WagonDocument, error) {
	tarballSha := ""

	tarball := filepath.Join(opts.OutputDir, opts.TreeFilename)
	document := wagon.NewWagonDocument(opts.TreeFilename)

	// Prepare tar-formers specs
	s := tarf_specs.NewSpecFile()
	s.SameChtimes = true
	s.Writer = tarf_specs.NewWriter()
	s.Writer.ArchiveDirs = []string{treefsDir}

	// Prepare tarball creation
	topts := tools.NewTarCompressionOpts(true)
	defer topts.Close()

	if opts.CompressionMode != tools.None {
		topts.UseExt = false
		topts.Mode = opts.CompressionMode
	} else {
		topts.Mode = tools.GetCompressionMode(tarball)
	}

	err := tools.PrepareTarWriter(tarball, topts)
	if err != nil {
		return nil, err
	}

	document.SetCompressionType(topts.Mode)

	// Tarformers handler to drop tree fs directory prefix from
	// files to archive in the tarball.
	handler := func(path, newpath string,
		header *tar.Header, tw *tar.Writer,
		opts *tarf.TarFileOperation, t *tarf.TarFormers) error {

		// For all files/directory i need to drop the treefs path
		opts.Rename = true
		opts.NewName, _ = filepath.Rel(treefsDir, newpath)
		return nil
	}

	tarformers := tarf.NewTarFormers(tarf.GetOptimusPrime().Config)
	if topts.CompressWriter != nil {
		tarformers.SetWriter(topts.CompressWriter)

	} else {
		tarformers.SetWriter(topts.FileWriter)
	}
	tarformers.SetFileWriterHandler(handler)

	err = tarformers.RunTaskWriter(s)
	if err != nil {
		return nil, err
	}
	// We need to close file else the sha is not elaborated
	// correctly.
	topts.Close()

	// Generate sha of the tarball
	tarballSha, err = fileHelper.Sha256Sum(tarball)
	if err != nil {
		return nil, err
	}

	document.Checksums[string(artifact.SHA256)] = tarballSha

	Debug(fmt.Sprintf("[%s] Generated %s with sha256 %s",
		w.Repository.Name, tarball, tarballSha))

	return document, nil
}

func (w *WagonFactory) BumpRevision(treePaths []string, opts *WagonFactoryOpts) error {
	aurora := GetAurora()
	// NOTE: To bump a repository the indexes must be available.
	//       This speedup process and memory consume.

	if len(treePaths) == 0 {
		return errors.New("No tree paths available")
	}

	if opts.PackagesDir == "" {
		return errors.New("Packages directory not defined")
	}

	if opts.OutputDir == "" {
		return errors.New("Output directory not defined")
	}

	idx := []*tree.TreeIdx{}

	// Load tree indexes
	for _, t := range treePaths {
		Debug(fmt.Sprintf("Loading tree %s", t))
		ti := tree.NewTreeIdx(t, false).DetectMode()
		err := ti.Read(t)
		if err != nil {
			return fmt.Errorf(
				"Error reading tree %s: %s",
				t, err.Error())
		}
		idx = append(idx, ti)
	}

	// Create a clone of the repository
	repo := w.Repository.Clone()
	repo.Authentication = make(map[string]string, 0)

	wIdentity := wagon.NewWagonIdentify(repo)

	// If exist repository.yaml retrieve wagon identify
	prevReposFile := filepath.Join(opts.PackagesDir, "repository.yaml")
	if fileHelper.Exists(prevReposFile) {
		wIdentity.Load(prevReposFile)
		tsec, _ := strconv.ParseInt(wIdentity.GetLastUpdate(), 10, 64)
		InfoC(
			aurora.Bold(
				aurora.Red(fmt.Sprintf(
					":house:Repository: %s existing revision %s and last update %s...",
					aurora.Bold(aurora.Green(wIdentity.GetName())).String(),
					aurora.Bold(aurora.Green(fmt.Sprintf("%3d", wIdentity.GetRevision()))).String(),
					aurora.Bold(aurora.Green(time.Unix(tsec, 0).String())).String(),
				))))
	}
	wIdentity.PurgeFiles()

	// Create working dir where build tree tarball.
	treefsDir, err := w.Config.GetSystem().TempDir("tree")
	if err != nil {
		return err
	}
	//defer os.RemoveAll(treefsDir)

	Debug("Using temporary tree path:", treefsDir)

	// Create treefs filesystem
	err = w.createTreeFs(&idx, opts, treefsDir)
	if err != nil {
		return err
	}

	// Ensure that the output directory is present.
	if err := os.MkdirAll(opts.OutputDir, os.ModePerm); err != nil {
		return err
	}

	// Create tarball tree.tar[.gz|.zst]
	docTree, err := w.createTreeTarball(opts, treefsDir)
	if err != nil {
		return err
	}

	if opts.WithCompilerTree {
		// Create tarball compilertree.tar[.gz|.zst]
		docCompiler, err := w.createCompilerTreeTarball(opts, treePaths)
		if err != nil {
			return err
		}
		wIdentity.RepositoryFiles[wagon.REPOFILE_COMPILER_TREE_KEY] = docCompiler
	}

	// Update Identify file
	wIdentity.RepositoryFiles[wagon.REPOFILE_TREEV2_KEY] = docTree
	wIdentity.BumpRevision()
	if opts.ResetRevision {
		wIdentity.AniseRepository.Revision = 1
	}

	tsec, _ := strconv.ParseInt(wIdentity.GetLastUpdate(), 10, 64)
	InfoC(
		aurora.Bold(
			aurora.Red(fmt.Sprintf(
				":house:Repository: %s creating revision %s and last update %s...",
				aurora.Bold(aurora.Green(wIdentity.GetName())).String(),
				aurora.Bold(aurora.Green(fmt.Sprintf("%3d", wIdentity.GetRevision()))).String(),
				aurora.Bold(aurora.Green(time.Unix(tsec, 0).String())).String(),
			))))
	// Write identify file
	identifyFilePath := filepath.Join(opts.OutputDir, "repository.yaml")
	err = wIdentity.Write(identifyFilePath)
	if err != nil {
		return err
	}

	return nil
}
