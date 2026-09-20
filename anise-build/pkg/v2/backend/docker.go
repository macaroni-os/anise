/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package backend

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cfg "github.com/macaroni-os/anise/pkg/config"
	fhelpers "github.com/macaroni-os/anise/pkg/helpers/file"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/options"

	tarf "github.com/geaaru/tar-formers/pkg/executor"
	tarf_specs "github.com/geaaru/tar-formers/pkg/specs"
)

type Dockerv3 struct {
	Config *cfg.AniseConfig
}

// Mutex to avoid errors on parallel
// setup of the viper object.
var mutex sync.Mutex

func NewDockerv3Backend(c *cfg.AniseConfig) BackendCompiler {
	return &Dockerv3{
		Config: c,
	}
}

func (d *Dockerv3) generateBuildImageHash(art *artifact.PackageArtifact,
	pthin *pkg.PackageThin, opts *options.Compiler) error {
	var psha hash.Hash = sha256.New()

	b, err := json.Marshal(pthin)
	if err != nil {
		return fmt.Errorf(
			"Error on generate build image hash for package %s: %s",
			pthin.PackageName(), err.Error())
	}

	psha.Write(b)

	// For build images using envs, prelude, image, seed
	if len(art.CompileSpec.Env) > 0 {
		for i := range art.CompileSpec.Env {
			psha.Write([]byte(art.CompileSpec.Env[i]))
		}
	}
	if len(art.CompileSpec.Prelude) > 0 {
		for i := range art.CompileSpec.Prelude {
			psha.Write([]byte(art.CompileSpec.Prelude[i]))
		}
	}
	if len(art.CompileSpec.Image) > 0 {
		psha.Write([]byte(art.CompileSpec.Image))
	}

	var h []byte = psha.Sum(nil)

	art.BuildImageHash = hex.EncodeToString(h)
	return nil
}

func (d *Dockerv3) generateFinalImageHash(art *artifact.PackageArtifact,
	pthin *pkg.PackageThin, opts *options.Compiler) error {

	// NOTE: I generate the hashing inside the docker backend
	//       because different technologies uses different logics.

	var psha hash.Hash = sha256.New()

	b, err := json.Marshal(pthin)
	if err != nil {
		return fmt.Errorf(
			"Error on generate build image hash for package %s: %s",
			pthin.PackageName(), err.Error())
	}

	psha.Write(b)

	// For build images using envs, prelude, image, seed
	if len(art.CompileSpec.Env) > 0 {
		for i := range art.CompileSpec.Env {
			psha.Write([]byte(art.CompileSpec.Env[i]))
		}
	}
	if len(art.CompileSpec.Prelude) > 0 {
		for i := range art.CompileSpec.Prelude {
			psha.Write([]byte(art.CompileSpec.Prelude[i]))
		}
	}
	if len(art.CompileSpec.Steps) > 0 {
		for i := range art.CompileSpec.Steps {
			psha.Write([]byte(art.CompileSpec.Steps[i]))
		}
	}
	if len(art.CompileSpec.Image) > 0 {
		psha.Write([]byte(art.CompileSpec.Image))
	}
	if len(art.CompileSpec.Includes) > 0 {
		for i := range art.CompileSpec.Includes {
			psha.Write([]byte(art.CompileSpec.Includes[i]))
		}
	}
	if len(art.CompileSpec.Excludes) > 0 {
		for i := range art.CompileSpec.Excludes {
			psha.Write([]byte(art.CompileSpec.Excludes[i]))
		}
	}

	var h []byte = psha.Sum(nil)

	art.FinalImageHash = hex.EncodeToString(h)
	return nil
}

func (d *Dockerv3) createBuildDockerfile(art *artifact.PackageArtifact,
	solution *artifact.ArtifactsPack,
	opts *options.Compiler, dockerFile string) error {

	dockerSteps := ""

	// TODO: resolve hash from dependency.
	if art.CompileSpec.Image != "" {
		// POST: We use defined image for build

		dockerSteps = fmt.Sprintf("FROM %s", art.CompileSpec.Image)

	}

	if len(art.GetPackage().GetRequires()) > 0 {

		// NOTE: I consider that all dependencies are elaborated
		//       before this package. This means that all artefacts
		//       are already with the BuilderImageHash and
		//       FinalImageHash attributes valorized.

		solutionMap := solution.ToMap()

		for idx, art := range art.GetPackage().GetRequires() {

			// Retrieve artefact from solution to retrieves
			// all hashes.
			deps, err := solutionMap.GetArtifactsByKey(art.PackageName())
			if err != nil {
				return err
			}

			if idx == 0 {
				dockerSteps += fmt.Sprintf("FROM %s:%s",
					opts.PushImageRepository,
					deps[0].GetFinalImageHash())
			} else {
				dockerSteps += "\n" +
					fmt.Sprintf("COPY --from=%s:%s / /",
						opts.PushImageRepository,
						deps[0].GetFinalImageHash())
			}
		}

	}

	if art.CompileSpec.Copy != nil && len(art.CompileSpec.Copy) > 0 {

		solutionMap := solution.ToMap()

		for _, cf := range art.CompileSpec.Copy {

			if cf.Package != nil {

				// Retrieve artefact from solution to retrieves
				// all hashes.
				deps, err := solutionMap.GetArtifactsByKey(cf.Package.PackageName())
				if err != nil {
					return err
				}

				dockerSteps += "\n" +
					fmt.Sprintf("COPY --from=%s:%s %s %s",
						opts.PushImageRepository,
						deps[0].GetFinalImageHash(),
						cf.Source, cf.Destination)

			} else {

				dockerSteps += "\n" +
					fmt.Sprintf("COPY --from=%s %s %s",
						cf.Image, cf.Source, cf.Destination)

			}

		}

	}

	if dockerSteps == "" {

		dockerSteps = "FROM scratch"

	}

	dockerSteps += "\n" +
		"COPY . /anisebuild\n" +
		"WORKDIR /anisebuild"

	// Set main ENV
	dockerSteps += "\n" +
		fmt.Sprintf("ENV PACKAGE_NAME=%s PACKAGE_CATEGORY=%s PACKAGE_VERSION=%s",
			art.GetPackage().GetName(),
			art.GetPackage().GetCategory(),
			art.GetPackage().GetVersion())

	if len(art.CompileSpec.Env) > 0 {
		dockerSteps += "\nENV"
		for _, e := range art.CompileSpec.Env {
			dockerSteps += fmt.Sprintf(" %s", e)
		}
	}

	if len(art.CompileSpec.Prelude) > 0 {
		for _, r := range art.CompileSpec.Prelude {
			dockerSteps += fmt.Sprintf("\nRUN %s", r)
		}
	}

	Debug(fmt.Sprintf("Build docker file for package %s:\n\n%s\n",
		art.GetPackage().PackageName(),
		dockerSteps))

	return os.WriteFile(dockerFile, []byte(dockerSteps), 0644)
}

func (d *Dockerv3) createFinalDockerfile(art *artifact.PackageArtifact,
	opts *options.Compiler, dockerFile string) error {

	buildTaggedImage := fmt.Sprintf("%s:builder-%s", opts.PushImageRepository,
		art.BuildImageHash)

	dockerSteps := fmt.Sprintf("FROM %s", buildTaggedImage)

	if len(art.CompileSpec.Steps) > 0 {
		for _, r := range art.CompileSpec.Steps {
			dockerSteps += fmt.Sprintf("\nRUN %s", r)
		}
	}

	Debug(fmt.Sprintf("Final docker file for package %s:\n%s\n",
		art.GetPackage().PackageName(),
		dockerSteps))

	return os.WriteFile(dockerFile, []byte(dockerSteps), 0644)
}

func (d *Dockerv3) CreateBuildImage(art *artifact.PackageArtifact,
	builddir string,
	solution *artifact.ArtifactsPack,
	opts *options.Compiler) error {

	// Using artefacts package thin to build the package Thin with
	// the selected version from the solution and generate
	// the hashing of the build image.

	withDeps := true
	pThin, err := art.ToPackageThin(withDeps, solution.ToMap())
	if err != nil {
		return err
	}

	err = d.generateBuildImageHash(art, pThin, opts)
	if err != nil {
		return err
	}

	remoteBuildertaggedImage := fmt.Sprintf("%s:builder-%s", opts.PushImageRepository,
		art.BuildImageHash)

	InfoC(fmt.Sprintf(
		":factory: Building image %s for %s",
		remoteBuildertaggedImage,
		art.GetPackage().HumanReadableString()))

	// Build staging directory
	buildPkgdir := filepath.Join(builddir,
		art.GetPackage().HumanReadableString())
	workdir := filepath.Join(buildPkgdir, "workdir")

	// Create package build directory
	err = fhelpers.EnsureDir(buildPkgdir + "/")
	if err != nil {
		return err
	}

	// Copy the working directory on staging dir
	err = fhelpers.CopyDir(art.Path, workdir)
	if err != nil {
		return fmt.Errorf("Error on copy file on workdir %s: %s",
			workdir, err.Error())
	}

	// Generate docker file
	dockerFile := filepath.Join(buildPkgdir,
		fmt.Sprintf("%s-builder.dockerfile", art.BuildImageHash))

	Debug(fmt.Sprintf("Creating file %s", dockerFile))

	// Prepare dockerfile for build image
	err = d.createBuildDockerfile(art, solution, opts, dockerFile)
	if err != nil {
		return err
	}

	buildImage := true

	// TODO: Fix support of different PullImageRepository/PushImageRepository
	if opts.PullFirst {
		err := d.PullImage(art, opts, remoteBuildertaggedImage)
		if err == nil {
			buildImage = false
		} else {
			Warning("Failed to download '" + remoteBuildertaggedImage +
				"'. Will keep going and build the image unless you use --fatal")
			Warning(err.Error())
		}
	}

	if buildImage {
		err = d.BuildImage(art, opts,
			workdir, dockerFile, remoteBuildertaggedImage)
		if err != nil {
			return err
		}
	}

	if opts.Push {
		err = d.PushImage(art, remoteBuildertaggedImage)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Dockerv3) BuildImage(art *artifact.PackageArtifact,
	opts *options.Compiler,
	workdir, dockerFile, imagename string) error {

	// NOTE: For squashing image add --squash option from backend-args.
	context := "."
	buildarg := append(opts.BackendArgs, "-f", dockerFile, "-t", imagename, context)
	buildarg = append([]string{"build"}, buildarg...)

	Info(":whale2: Building image " + imagename)
	cmd := exec.Command("docker", buildarg...)
	Debug(fmt.Sprintf(
		"Running command: %s %s", "docker", strings.Join(buildarg, " ")))
	cmd.Dir = workdir
	err := runCommand(cmd)
	if err != nil {
		return err
	}

	Info(":whale: Building image " + imagename + " :check_mark:")

	return nil
}

func (d *Dockerv3) PushImage(art *artifact.PackageArtifact,
	imagename string) error {

	pushargs := []string{"push", imagename}

	Info(":whale2: Pushing image", imagename, "...")

	cmd := exec.Command("docker", pushargs...)
	Debug(fmt.Sprintf(
		"Running command: %s %s", "docker", strings.Join(pushargs, " ")))
	err := runCommand(cmd)
	if err != nil {
		return err
	}

	Info(":whale: Pushed image:", imagename)

	return nil
}

func (d *Dockerv3) PullImage(art *artifact.PackageArtifact,
	opts *options.Compiler,
	imagename string) error {

	pullargs := []string{"pull", imagename}

	Info(":whale2: Trying pulling image", imagename, "...")

	cmd := exec.Command("docker", pullargs...)
	Debug(fmt.Sprintf(
		"Running command: %s %s", "docker", strings.Join(pullargs, " ")))
	err := runCommand(cmd)
	if err != nil {
		return err
	}

	Info(":whale: Pulled image:", imagename)

	return nil
}

func (d *Dockerv3) deleteContainer(name string) {
	deleteargs := []string{"rm", name}

	Debug(":whale: deleting container with name" + name)
	out, err := exec.Command("docker", deleteargs...).CombinedOutput()
	if err != nil {
		Warning("Failed delete container " + name + " for image: " + string(out))
	} else {
		Debug("Container " + name + " removed.")
	}
}

func (d *Dockerv3) createTarFormers() *tarf.TarFormers {
	mutex.Lock()
	defer mutex.Unlock()

	// Create config
	c := tarf_specs.NewConfig(d.Config.Viper)
	c.GetGeneral().Debug = d.Config.GetGeneral().Debug
	c.GetLogging().Level = d.Config.GetLogging().Level

	ans := tarf.NewTarFormers(c)

	return ans
}

func (d *Dockerv3) CreateFinalImage(art *artifact.PackageArtifact,
	builddir string,
	solution *artifact.ArtifactsPack,
	opts *options.Compiler) error {

	withDeps := true
	pThin, err := art.ToPackageThin(withDeps, solution.ToMap())
	if err != nil {
		return err
	}

	err = d.generateFinalImageHash(art, pThin, opts)
	if err != nil {
		return err
	}

	remotetaggedImage := fmt.Sprintf("%s:%s", opts.PushImageRepository,
		art.FinalImageHash)

	InfoC(fmt.Sprintf(
		":factory: Prepare generation of final image %s for %s",
		remotetaggedImage,
		art.GetPackage().HumanReadableString()))

	// Build staging directory
	buildPkgdir := filepath.Join(builddir,
		art.GetPackage().HumanReadableString())
	workdir := filepath.Join(buildPkgdir, "workdir")

	// Generate docker file
	dockerFile := filepath.Join(buildPkgdir,
		fmt.Sprintf("%s.dockerfile", art.FinalImageHash))

	Debug(fmt.Sprintf("Creating file %s", dockerFile))

	// Prepare dockerfile for tag package image
	err = d.createFinalDockerfile(art, opts, dockerFile)
	if err != nil {
		return err
	}

	buildImage := true

	// TODO: Fix support of different PullImageRepository/PushImageRepository
	if opts.PullFirst {
		err := d.PullImage(art, opts, remotetaggedImage)
		if err == nil {
			buildImage = false
		} else {
			Warning("Failed to download '" + remotetaggedImage +
				"'. Will keep going and build the image unless you use --fatal")
			Warning(err.Error())
		}
	}

	if buildImage {
		err = d.BuildImage(art, opts,
			workdir, dockerFile, remotetaggedImage)
		if err != nil {
			return err
		}
	}

	if opts.Push {
		err = d.PushImage(art, remotetaggedImage)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Dockerv3) ExportImage(art *artifact.PackageArtifact,
	opts *options.Compiler,
	extractdir string) error {

	remotetaggedImage := fmt.Sprintf("%s:%s", opts.PushImageRepository,
		art.FinalImageHash)

	if !strings.HasSuffix(extractdir, "/") {
		extractdir = extractdir + "/"
	}

	// Create the container from specified image
	createargs := []string{
		"create", remotetaggedImage,
		"-c", "sleep", "1",
	}
	Debug(":whale: Creating container from image " + remotetaggedImage)

	// Creating a fake container to use for the export.
	out, err := exec.Command("docker", createargs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed creating container for image %s: %s",
			remotetaggedImage, err.Error())
	}
	idcontainer := strings.TrimRight(string(out), "\n")
	Debug(":whale: Container for image " + remotetaggedImage + " (id " + idcontainer + ") created.")
	defer d.deleteContainer(idcontainer)

	// Prepare tar-formers stuff to execute
	tarformers := d.createTarFormers()
	spec := tarf_specs.NewSpecFile()
	spec.IgnoreFiles = []string{
		"/.dockerenv",
	}
	spec.MapEntities = false
	spec.SameChtimes = false
	// In general this must be always set a true.
	spec.SameOwner = d.Config.GetGeneral().SameOwner
	spec.BrokenLinksFatal = true
	spec.Summary = true

	args := []string{}

	if art.CompileSpec.PackageDir == "" {
		args = []string{"export", idcontainer}

		Debug(fmt.Sprintf(":whale: Extracts container files to %s...",
			extractdir))

	} else {

		// Prepare cp command where get stdout pipe.
		// The source path is in the format <container-id>:/path
		sourcePath := idcontainer + ":" + art.CompileSpec.PackageDir
		// The destpath must consider that dockers on cp get only
		// the final directory. So I need manually fix the Rename rule
		// for tarformers.
		paths := strings.Split(art.CompileSpec.PackageDir, "/")
		replacePrefix := art.CompileSpec.PackageDir
		if len(paths) > 2 {
			// Avoid to set final / because we replace with empty string.
			replacePrefix = "/" + paths[len(paths)-1]
		} else if replacePrefix[len(replacePrefix)-1:] == "/" {
			replacePrefix = replacePrefix[0 : len(replacePrefix)-1]
		}

		Debug(fmt.Sprintf(":whale: Copy container file from %s to %s (replace string %s)...",
			sourcePath, extractdir, replacePrefix))

		args = []string{"cp", "-a", sourcePath, "-"}

		spec.RenamePath = []tarf_specs.RenameRule{
			tarf_specs.RenameRule{
				Source: replacePrefix,
				Dest:   "",
			},
		}
	}

	dockerCmd := exec.Command("docker", args...)

	buffered := !d.Config.GetGeneral().ShowBuildOutput
	writer := NewBackendWriter(buffered)

	outReader, err := dockerCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed get stdout pipe for image %s: %s",
			remotetaggedImage, err.Error())
	}
	dockerCmd.Stderr = writer

	tarformers.SetReader(outReader)

	DebugC("Run docker " + strings.Join(args, " "))
	err = dockerCmd.Start()
	if err != nil {
		return fmt.Errorf("error on start docker cp command: %s", err.Error())
	}

	err = tarformers.RunTask(spec, extractdir)
	if err != nil {
		return fmt.Errorf("failed process container tarball: %s", err.Error())
	}

	err = dockerCmd.Wait()
	if err != nil {
		return fmt.Errorf("failed wait command for image %s: %s",
			remotetaggedImage, err.Error())
	}

	if dockerCmd.ProcessState.ExitCode() != 0 {
		return fmt.Errorf("Container export failed for image %s: %s",
			remotetaggedImage, err.Error())
	}

	Debug(":whale: Exported image:", remotetaggedImage)

	art.Metadata = artifact.NewExtraMetadata()
	art.Metadata.Files = tarformers.GetSummary().Files

	// Keep compatibility until the files field will be
	// removed.
	art.Files = []string{}
	for _, f := range art.Metadata.Files {
		if f.Type == tar.TypeReg || f.Type == tar.TypeRegA {
			art.Files = append(art.Files, f.Name)
		}
	}

	return nil
}

func (d *Dockerv3) GeneratePackage(art *artifact.PackageArtifact,
	builddir string, opts *options.Compiler) error {

	// Build staging directory
	buildPkgdir := filepath.Join(builddir,
		art.GetPackage().HumanReadableString())
	pkgExtractDir := filepath.Join(buildPkgdir, "extractroofs")

	// Create package build directory
	err := fhelpers.EnsureDir(pkgExtractDir + "/")
	if err != nil {
		return err
	}

	// Extract files from the tagged image.
	err = d.ExportImage(art, opts, pkgExtractDir)
	if err != nil {
		return err
	}

	// Create metadata of the package: file list, checksums, sizes, etc.

	if art.CompileSpec.GetPackageDir() != "" {
		Info(":tophat: Packing from output dir", art.CompileSpec.GetPackageDir())
	}

	art.Path = filepath.Join(builddir, art.GetPackage().GetFingerPrint()+".package.tar")
	art.CompressionType = opts.CompressionType

	if err := art.Compress(pkgExtractDir, d.Config.GetGeneral().Concurrency); err != nil {
		return fmt.Errorf("error met while creating package archive: %s", err.Error())
	}

	art.CompileSpec.GetPackage().SetBuildTimestamp(time.Now().String())

	// Generate metadata.yaml file.
	// Set CachePath equals to Path in order to use deprecated Checksum calculation.
	//art.CachePath = art.Path
	metadataFile := filepath.Join(builddir, art.GetPackage().GetFingerPrint()+
		"."+pkg.PackageMetaSuffix)
	err = art.WriteYaml(metadataFile)
	if err != nil {
		return err
	}

	// Generate extra-metadata.json file
	err = art.WriteExtraMetadata(builddir)
	if err != nil {
		return err
	}

	return nil
}
