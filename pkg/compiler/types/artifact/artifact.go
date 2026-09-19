/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package artifact

import (
	"archive/tar"
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"

	zstd "github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"

	//"strconv"
	"strings"
	"sync"

	tarf "github.com/geaaru/tar-formers/pkg/executor"
	tarf_specs "github.com/geaaru/tar-formers/pkg/specs"
	backend "github.com/macaroni-os/anise/pkg/compiler/backend"
	compression "github.com/macaroni-os/anise/pkg/compiler/types/compression"
	compilerspec "github.com/macaroni-os/anise/pkg/compiler/types/spec"
	. "github.com/macaroni-os/anise/pkg/config"
	"github.com/macaroni-os/anise/pkg/helpers"
	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/solver"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

//	When compiling, we write also a fingerprint.metadata.yaml file with PackageArtifact. In this way we can have another command to create the repository
//
// which will consist in just of an repository.yaml which is just the repository structure with the list of package artifact.
// In this way a generic client can fetch the packages and, after unpacking the tree, performing queries to install packages.
type PackageArtifact struct {
	Path string `json:"path" yaml:"path"`

	Dependencies      []*PackageArtifact                 `json:"dependencies" yaml:"dependencies"`
	CompileSpec       *compilerspec.AniseCompilationSpec `json:"compilespec" yaml:"compilespec"`
	Checksums         Checksums                          `json:"checksums" yaml:"checksums"`
	SourceAssertion   solver.PackagesAssertions          `json:"-" yaml:"-"`
	CompressionType   compression.Implementation         `json:"compressiontype" yaml:"compressiontype"`
	Files             []string                           `json:"files" yaml:"files"`
	PackageCacheImage string                             `json:"package_cacheimage" yaml:"package_cacheimage"`
	Runtime           *pkg.DefaultPackage                `json:"runtime,omitempty" yaml:"runtime,omitempty"`
}

func (p *PackageArtifact) ShallowCopy() *PackageArtifact {
	copy := *p
	return &copy
}

func NewPackageArtifact(path string) *PackageArtifact {
	return &PackageArtifact{Path: path, Dependencies: []*PackageArtifact{}, Checksums: Checksums{}, CompressionType: compression.None}
}

func NewPackageArtifactFromYaml(data []byte) (*PackageArtifact, error) {
	p := &PackageArtifact{Checksums: Checksums{}}
	err := yaml.Unmarshal(data, &p)
	if err != nil {
		return p, err
	}

	return p, err
}

func (a *PackageArtifact) Hash() error {
	return a.Checksums.Generate(a)
}

func (a *PackageArtifact) Verify() error {
	sum := Checksums{}
	if err := sum.Generate(a); err != nil {
		return err
	}

	if err := sum.Compare(a.Checksums); err != nil {
		return err
	}

	return nil
}

func (a *PackageArtifact) WriteYaml(dst string) error {
	// First compute checksum of artifact. When we write the yaml we want to write up-to-date informations.
	err := a.Hash()
	if err != nil {
		return errors.Wrap(err, "Failed generating checksums for artifact")
	}

	// Update runtime package information
	if a.CompileSpec != nil && a.CompileSpec.Package != nil {
		runtime, err := a.CompileSpec.Package.GetRuntimePackage()
		if err != nil {
			return errors.Wrapf(err, "getting runtime package for '%s'", a.CompileSpec.Package.HumanReadableString())
		}

		a.Runtime = runtime
	}

	data, err := yaml.Marshal(a)
	if err != nil {
		return errors.Wrap(err, "While marshalling for PackageArtifact YAML")
	}

	mangle, err := NewPackageArtifactFromYaml(data)
	if err != nil {
		return errors.Wrap(err, "Generated invalid artifact")
	}
	//p := a.CompileSpec.GetPackage().GetPath()

	mangle.CompileSpec.GetPackage().SetPath("")
	for _, ass := range mangle.CompileSpec.GetSourceAssertion() {
		ass.Package.SetPath("")
	}

	data, err = yaml.Marshal(mangle)
	if err != nil {
		return errors.Wrap(err, "While marshalling for PackageArtifact YAML")
	}

	err = ioutil.WriteFile(filepath.Join(dst, a.CompileSpec.GetPackage().GetMetadataFilePath()), data, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "While writing PackageArtifact YAML")
	}
	//a.CompileSpec.GetPackage().SetPath(p)
	return nil
}

func (a *PackageArtifact) GetFileName() string {
	return path.Base(a.Path)
}

func (a *PackageArtifact) genDockerfile() string {
	return `
FROM scratch
COPY . /`
}

// CreateArtifactForFile creates a new artifact from the given file
func CreateArtifactForFile(s string, opts ...func(*PackageArtifact)) (*PackageArtifact, error) {
	if _, err := os.Stat(s); os.IsNotExist(err) {
		return nil, errors.Wrap(err, "artifact path doesn't exist")
	}
	fileName := path.Base(s)
	archive, err := AniseCfg.GetSystem().TempDir("archive")
	if err != nil {
		return nil, errors.Wrap(err, "error met while creating tempdir for "+s)
	}
	defer os.RemoveAll(archive) // clean up
	dst := filepath.Join(archive, fileName)
	if err := fileHelper.CopyFile(s, dst); err != nil {
		return nil, errors.Wrapf(err, "error while copying %s to %s", s, dst)
	}

	artifact, err := AniseCfg.GetSystem().TempDir("artifact")
	if err != nil {
		return nil, errors.Wrap(err, "error met while creating tempdir for "+s)
	}
	a := &PackageArtifact{Path: filepath.Join(artifact, fileName)}

	for _, o := range opts {
		o(a)
	}

	return a, a.Compress(archive, 1)
}

type ImageBuilder interface {
	BuildImage(backend.Options) error
}

// GenerateFinalImage takes an artifact and builds a Docker image with its content
func (a *PackageArtifact) GenerateFinalImage(imageName string, b ImageBuilder, keepPerms bool) (backend.Options, error) {
	builderOpts := backend.Options{}
	archive, err := AniseCfg.GetSystem().TempDir("archive")
	if err != nil {
		return builderOpts, errors.Wrap(err, "error met while creating tempdir for "+a.Path)
	}
	defer os.RemoveAll(archive) // clean up

	uncompressedFiles := filepath.Join(archive, "files")
	dockerFile := filepath.Join(archive, "Dockerfile")

	if err := os.MkdirAll(uncompressedFiles, os.ModePerm); err != nil {
		return builderOpts, errors.Wrap(err, "error met while creating tempdir for "+a.Path)
	}

	if err := a.Unpack(uncompressedFiles, true); err != nil {
		return builderOpts, errors.Wrap(err, "error met while uncompressing artifact "+a.Path)
	}

	empty, err := fileHelper.DirectoryIsEmpty(uncompressedFiles)
	if err != nil {
		return builderOpts, errors.Wrap(err, "error met while checking if directory is empty "+uncompressedFiles)
	}

	// See https://github.com/moby/moby/issues/38039.
	// We can't generate FROM scratch empty images. Docker will refuse to export them
	// workaround: Inject a .virtual empty file
	if empty {
		fileHelper.Touch(filepath.Join(uncompressedFiles, ".virtual"))
	}

	data := a.genDockerfile()
	if err := ioutil.WriteFile(dockerFile, []byte(data), 0644); err != nil {
		return builderOpts, errors.Wrap(err, "error met while rendering artifact dockerfile "+a.Path)
	}

	builderOpts = backend.Options{
		ImageName:      imageName,
		SourcePath:     archive,
		DockerFileName: dockerFile,
		Context:        uncompressedFiles,
	}
	return builderOpts, b.BuildImage(builderOpts)
}

// Compress is responsible to archive and compress to the artifact Path.
// It accepts a source path, which is the content to be archived/compressed
// and a concurrency parameter.
func (a *PackageArtifact) Compress(src string, concurrency int) error {
	var tarFile string

	cleanup := func() {
		os.RemoveAll(tarFile) // Remove original
		Debug("Removed artifact", tarFile)
	}

	switch a.CompressionType {

	case compression.Zstandard:
		err := helpers.Tar(src, a.Path)
		if err != nil {
			return err
		}
		original, err := os.Open(a.Path)
		if err != nil {
			return err
		}
		defer original.Close()

		zstdFile := a.getCompressedName()
		bufferedReader := bufio.NewReader(original)

		// Open a file for writing.
		dst, err := os.Create(zstdFile)
		if err != nil {
			return err
		}

		enc, err := zstd.NewWriter(dst)
		if err != nil {
			return err
		}
		_, err = io.Copy(enc, bufferedReader)
		if err != nil {
			enc.Close()
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}

		tarFile = a.Path
		defer cleanup()

		a.Path = zstdFile
		return nil
	case compression.GZip:
		err := helpers.Tar(src, a.Path)
		if err != nil {
			return err
		}
		original, err := os.Open(a.Path)
		if err != nil {
			return err
		}
		defer original.Close()

		gzipfile := a.getCompressedName()
		bufferedReader := bufio.NewReader(original)

		// Open a file for writing.
		dst, err := os.Create(gzipfile)
		if err != nil {
			return err
		}
		// Create gzip writer.
		w := gzip.NewWriter(dst)
		w.SetConcurrency(1<<20, concurrency)
		defer w.Close()
		defer dst.Close()
		_, err = io.Copy(w, bufferedReader)
		if err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		tarFile = a.Path
		defer cleanup()
		//	a.CompressedPath = gzipfile
		a.Path = gzipfile
		return nil
		//a.Path = gzipfile

	// Defaults to tar only (covers when "none" is supplied)
	default:
		return helpers.Tar(src, a.getCompressedName())
	}
}

func (a *PackageArtifact) getCompressedName() string {
	switch a.CompressionType {
	case compression.Zstandard:
		return a.Path + ".zst"

	case compression.GZip:
		return a.Path + ".gz"
	}
	return a.Path
}

// GetUncompressedName returns the artifact path without the extension suffix
func (a *PackageArtifact) GetUncompressedName() string {
	switch a.CompressionType {
	case compression.Zstandard, compression.GZip:
		return strings.TrimSuffix(a.Path, filepath.Ext(a.Path))
	}
	return a.Path
}

func hashContent(bv []byte) string {
	hasher := sha1.New()
	hasher.Write(bv)
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	return sha
}

func hashFileContent(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
}

func tarModifierWrapperFunc(path, dst string, header *tar.Header, content io.Reader,
	opts *tarf.TarFileOperation, t *tarf.TarFormers) error {

	// If the destination path already exists I rename target file name with postfix.
	var destPath string

	buffer := bytes.Buffer{}
	if content != nil {
		if _, err := buffer.ReadFrom(content); err != nil {
			return err
		}
	}
	tarHash := hashContent(buffer.Bytes())

	switch header.Typeflag {
	case tar.TypeReg:
		destPath = filepath.Join(dst, path)
	default:

		// Nothing to do. I return original reader
		return nil
	}

	// Always skip the write for tarformers because the io.Reader is been
	// already consumed.
	opts.Skip = true

	existingHash := ""
	setfileprops := false
	f, err := os.Lstat(destPath)
	if err == nil {
		Debug("File exists already, computing hash for", destPath)
		hash, herr := hashFileContent(destPath)
		if herr == nil {
			existingHash = hash
		}
	} else {
		setfileprops = true
	}

	Debug("Existing file hash: ", existingHash, "Tar file hashsum: ", tarHash)
	// We want to protect file only if the hash of the files are differing OR the file size are
	differs := (existingHash != "" && existingHash != tarHash) || (err != nil && f != nil && header.Size != f.Size())

	// Check if exists
	if fileHelper.Exists(destPath) && differs {
		for i := 1; i < 1000; i++ {
			name := filepath.Join(filepath.Join(filepath.Dir(path),
				fmt.Sprintf("._cfg%04d_%s", i, filepath.Base(path))))

			if fileHelper.Exists(name) {
				continue
			}

			Info(fmt.Sprintf("Found protected file %s. Creating %s.", destPath,
				filepath.Join(dst, name)))
			path = name
			break
		}
	}

	info := header.FileInfo()
	// Write the file
	_, err = t.CreateFile(dst, path, info.Mode(), bytes.NewReader(buffer.Bytes()), header)
	if err != nil {
		return err
	}
	if setfileprops {
		meta := tarf_specs.NewFileMeta(header)
		return t.SetFileProps(filepath.Join(dst, path), &meta, false)
	}
	return nil
}

func (a *PackageArtifact) GetProtectFiles() []string {
	ans := []string{}
	annotationDir := ""

	if !AniseCfg.ConfigProtectSkip {

		// a.CompileSpec could be nil when artifact.Unpack is used for tree tarball
		if a.CompileSpec != nil &&
			a.CompileSpec.GetPackage().HasAnnotation(string(pkg.ConfigProtectAnnnotation)) {
			dir, ok := a.CompileSpec.GetPackage().GetAnnotations()[string(pkg.ConfigProtectAnnnotation)].(string)
			if ok {
				annotationDir = dir
			}
		}
		// TODO: check if skip this if we have a.CompileSpec nil

		cp := NewConfigProtect(annotationDir)
		cp.Map(a.Files)

		ans = cp.GetProtectFiles(true)
	}

	return ans
}

// Unpack Untar and decompress (TODO) to the given path
func (a *PackageArtifact) Unpack(dst string, enableSubsets bool) error {
	if !strings.HasPrefix(dst, "/") {
		return errors.New("destination must be an absolute path")
	}

	// Create protected file list
	protectedFiles := a.GetProtectFiles()
	// Create untar specs
	spec := a.GetTarFormersSpec(enableSubsets)

	switch a.CompressionType {
	case compression.Zstandard:
		// Create the uncompressed archive
		archive, err := os.Create(a.Path + ".uncompressed")
		if err != nil {
			return err
		}
		defer os.RemoveAll(a.Path + ".uncompressed")
		defer archive.Close()

		original, err := os.Open(a.Path)
		if err != nil {
			return errors.Wrap(err, "Cannot open "+a.Path)
		}
		defer original.Close()

		bufferedReader := bufio.NewReader(original)

		d, err := zstd.NewReader(bufferedReader)
		if err != nil {
			return err
		}
		defer d.Close()

		_, err = io.Copy(archive, d)
		if err != nil {
			return errors.Wrap(err, "Cannot copy to "+a.Path+".uncompressed")
		}

		err = helpers.UntarProtectSpec(a.Path+".uncompressed", dst,
			protectedFiles, tarModifierWrapperFunc, spec)
		if err != nil {
			return err
		}
		return nil
	case compression.GZip:
		// Create the uncompressed archive
		archive, err := os.Create(a.Path + ".uncompressed")
		if err != nil {
			return err
		}
		defer os.RemoveAll(a.Path + ".uncompressed")
		defer archive.Close()

		original, err := os.Open(a.Path)
		if err != nil {
			return errors.Wrap(err, "Cannot open "+a.Path)
		}
		defer original.Close()

		bufferedReader := bufio.NewReader(original)
		r, err := gzip.NewReader(bufferedReader)
		if err != nil {
			return err
		}
		defer r.Close()

		_, err = io.Copy(archive, r)
		if err != nil {
			return errors.Wrap(err, "Cannot copy to "+a.Path+".uncompressed")
		}

		err = helpers.UntarProtectSpec(a.Path+".uncompressed", dst,
			protectedFiles, tarModifierWrapperFunc, spec)
		if err != nil {
			return err
		}
		return nil
	// Defaults to tar only (covers when "none" is supplied)
	default:
		return helpers.UntarProtect(a.Path, dst,
			AniseCfg.GetGeneral().SameOwner,
			AniseCfg.GetGeneral().OverwriteDirPerms,
			protectedFiles, tarModifierWrapperFunc)
	}
}

// FileList generates the list of file of a package from the local archive
func (a *PackageArtifact) FileList() ([]string, error) {
	var tr *tar.Reader
	archiveDir, err := AniseCfg.GetSystem().TempDir(
		fmt.Sprintf("%s", filepath.Base(a.Path)))
	if err != nil {
		return []string{}, err
	}
	cleandir := func() {
		os.RemoveAll(archiveDir)
	}
	defer cleandir()

	switch a.CompressionType {
	case compression.Zstandard:
		archive, err := os.Create(filepath.Join(archiveDir,
			filepath.Base(a.Path)+".uncompressed"))
		if err != nil {
			return []string{}, err
		}
		defer archive.Close()

		original, err := os.Open(a.Path)
		if err != nil {
			return []string{}, errors.Wrap(err, "Cannot open "+a.Path)
		}
		defer original.Close()

		bufferedReader := bufio.NewReader(original)
		r, err := zstd.NewReader(bufferedReader)
		if err != nil {
			return []string{}, err
		}
		defer r.Close()
		tr = tar.NewReader(r)
	case compression.GZip:
		// Create the uncompressed archive
		archive, err := os.Create(filepath.Join(archiveDir,
			filepath.Base(a.Path)+".uncompressed"))
		if err != nil {
			return []string{}, err
		}
		defer archive.Close()

		original, err := os.Open(a.Path)
		if err != nil {
			return []string{}, errors.Wrap(err, "Cannot open "+a.Path)
		}
		defer original.Close()

		bufferedReader := bufio.NewReader(original)
		r, err := gzip.NewReader(bufferedReader)
		if err != nil {
			return []string{}, err
		}
		defer r.Close()
		tr = tar.NewReader(r)

	// Defaults to tar only (covers when "none" is supplied)
	default:
		tarFile, err := os.Open(a.Path)
		if err != nil {
			return []string{}, errors.Wrap(err, "Could not open package archive")
		}
		defer tarFile.Close()
		tr = tar.NewReader(tarFile)

	}

	var files []string
	// untar each segment
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return []string{}, err
		}
		// determine proper file path info
		finfo := hdr.FileInfo()
		fileName := hdr.Name
		if finfo.Mode().IsDir() {
			continue
		}
		files = append(files, fileName)

		// if a dir, create it, then go to next segment
	}

	return files, nil
}

type CopyJob struct {
	Src, Dst string
	Artifact string
}

func worker(i int, wg *sync.WaitGroup, s <-chan CopyJob) {
	defer wg.Done()

	for job := range s {
		_, err := os.Lstat(job.Dst)
		if err != nil {
			Debug("Copying ", job.Src)
			if err := fileHelper.DeepCopyFile(job.Src, job.Dst); err != nil {
				Warning("Error copying", job, err)
			}
		}
	}
}

func compileRegexes(regexes []string) []*regexp.Regexp {
	var result []*regexp.Regexp
	for _, i := range regexes {
		r, e := regexp.Compile(i)
		if e != nil {
			Warning("Failed compiling regex:", e)
			continue
		}
		result = append(result, r)
	}
	return result
}

type ArtifactNode struct {
	Name string `json:"Name"`
	Size int    `json:"Size"`
}
type ArtifactDiffs struct {
	Additions []ArtifactNode `json:"Adds"`
	Deletions []ArtifactNode `json:"Dels"`
	Changes   []ArtifactNode `json:"Mods"`
}

type ArtifactLayer struct {
	FromImage string        `json:"Image1"`
	ToImage   string        `json:"Image2"`
	Diffs     ArtifactDiffs `json:"Diff"`
}

// ExtractArtifactFromDelta extracts deltas from ArtifactLayer from an image in tar format
func ExtractArtifactFromDelta(src, dst string, layers []ArtifactLayer, concurrency int, keepPerms bool, includes []string, excludes []string, t compression.Implementation) (*PackageArtifact, error) {

	archive, err := AniseCfg.GetSystem().TempDir("archive")
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating tempdir for archive")
	}
	defer os.RemoveAll(archive) // clean up

	if strings.HasSuffix(src, ".tar") {
		rootfs, err := AniseCfg.GetSystem().TempDir("rootfs")
		if err != nil {
			return nil, errors.Wrap(err, "Error met while creating tempdir for rootfs")
		}
		defer os.RemoveAll(rootfs) // clean up
		err = helpers.Untar(src, rootfs, keepPerms, AniseCfg.GetGeneral().OverwriteDirPerms)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while unpacking rootfs")
		}
		src = rootfs
	}

	toCopy := make(chan CopyJob)

	var wg = new(sync.WaitGroup)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(i, wg, toCopy)
	}

	// Handle includes in spec. If specified they filter what gets in the package

	if len(includes) > 0 && len(excludes) == 0 {
		includeRegexp := compileRegexes(includes)
		for _, l := range layers {
			// Consider d.Additions (and d.Changes? - warn at least) only
		ADDS:
			for _, a := range l.Diffs.Additions {
				for _, i := range includeRegexp {
					if i.MatchString(a.Name) {
						toCopy <- CopyJob{Src: filepath.Join(src, a.Name), Dst: filepath.Join(archive, a.Name), Artifact: a.Name}
						continue ADDS
					}
				}
			}
			for _, a := range l.Diffs.Changes {
				Debug("File ", a.Name, " changed")
			}
			for _, a := range l.Diffs.Deletions {
				Debug("File ", a.Name, " deleted")
			}
		}

	} else if len(includes) == 0 && len(excludes) != 0 {
		excludeRegexp := compileRegexes(excludes)
		for _, l := range layers {
			// Consider d.Additions (and d.Changes? - warn at least) only
		ADD:
			for _, a := range l.Diffs.Additions {
				for _, i := range excludeRegexp {
					if i.MatchString(a.Name) {
						continue ADD
					}
				}
				toCopy <- CopyJob{Src: filepath.Join(src, a.Name), Dst: filepath.Join(archive, a.Name), Artifact: a.Name}
			}
			for _, a := range l.Diffs.Changes {
				Debug("File ", a.Name, " changed")
			}
			for _, a := range l.Diffs.Deletions {
				Debug("File ", a.Name, " deleted")
			}
		}

	} else if len(includes) != 0 && len(excludes) != 0 {
		includeRegexp := compileRegexes(includes)
		excludeRegexp := compileRegexes(excludes)

		for _, l := range layers {
			// Consider d.Additions (and d.Changes? - warn at least) only
		EXCLUDES:
			for _, a := range l.Diffs.Additions {
				for _, i := range includeRegexp {
					if i.MatchString(a.Name) {
						for _, e := range excludeRegexp {
							if e.MatchString(a.Name) {
								continue EXCLUDES
							}
						}
						toCopy <- CopyJob{Src: filepath.Join(src, a.Name), Dst: filepath.Join(archive, a.Name), Artifact: a.Name}
						continue EXCLUDES
					}
				}
			}
			for _, a := range l.Diffs.Changes {
				Debug("File ", a.Name, " changed")
			}
			for _, a := range l.Diffs.Deletions {
				Debug("File ", a.Name, " deleted")
			}
		}

	} else {
		// Otherwise just grab all
		for _, l := range layers {
			// Consider d.Additions (and d.Changes? - warn at least) only
			for _, a := range l.Diffs.Additions {
				Debug("File ", a.Name, " added")
				toCopy <- CopyJob{Src: filepath.Join(src, a.Name), Dst: filepath.Join(archive, a.Name), Artifact: a.Name}
			}
			for _, a := range l.Diffs.Changes {
				Debug("File ", a.Name, " changed")
			}
			for _, a := range l.Diffs.Deletions {
				Debug("File ", a.Name, " deleted")
			}
		}
	}

	close(toCopy)
	wg.Wait()

	a := NewPackageArtifact(dst)
	a.CompressionType = t
	err = a.Compress(archive, concurrency)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating package archive")
	}
	return a, nil
}
