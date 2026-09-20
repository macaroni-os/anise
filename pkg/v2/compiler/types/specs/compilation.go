/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package specs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"

	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/options"

	"github.com/mitchellh/hashstructure/v2"
	"github.com/otiai10/copy"
	"golang.org/x/mod/sumdb/dirhash"
	"gopkg.in/yaml.v3"
)

func (cs *CompilationSpec) signature() *Signature {
	return &Signature{
		Image:               cs.Image,
		Steps:               cs.Steps,
		PackageDir:          cs.PackageDir,
		Prelude:             cs.Prelude,
		Seed:                cs.Seed,
		Env:                 cs.Env,
		Retrieve:            cs.Retrieve,
		Unpack:              cs.Unpack,
		Includes:            cs.Includes,
		Excludes:            cs.Excludes,
		Copy:                cs.Copy,
		Requires:            cs.Package.GetRequires(),
		RequiresFinalImages: cs.RequiresFinalImages,
		FlatImage:           cs.FlatImage,
	}
}

func NewCompilationSpec(b []byte, p pkg.Package) (*CompilationSpec, error) {
	var spec CompilationSpec
	var packageDefinition pkg.DefaultPackage
	err := yaml.Unmarshal(b, &spec)
	if err != nil {
		return &spec, err
	}
	err = yaml.Unmarshal(b, &packageDefinition)
	if err != nil {
		return &spec, err
	}

	// Update requires/conflict/provides
	// When we have been passed a bytes slice, parse it as a package
	// and updates requires/conflicts/provides.
	// This is required in order to allow manipulation of such fields with templating
	copy := *p.(*pkg.DefaultPackage)
	spec.Package = &copy
	if len(packageDefinition.GetRequires()) != 0 {
		spec.Package.Requires(packageDefinition.GetRequires())
	}
	if len(packageDefinition.GetConflicts()) != 0 {
		spec.Package.Conflicts(packageDefinition.GetConflicts())
	}
	if len(packageDefinition.GetProvides()) != 0 {
		spec.Package.SetProvides(packageDefinition.GetProvides())
	}
	return &spec, nil
}
func (cs *CompilationSpec) SetBuildOptions(b options.Compiler) {
	cs.BuildOptions = &b
}

func (cs *CompilationSpec) IsValid() (bool, error) {

	if !cs.IsVirtual() {
		if cs.Image == "" {
			if len(cs.Package.GetRequires()) == 0 && len(cs.Copy) == 0 {
				return false,
					errors.New("No requires, image or layer to join found")
			}
		}
	}

	return true, nil
}

func (cs *CompilationSpec) GetPackage() pkg.Package {
	return cs.Package
}

func (cs *CompilationSpec) GetPackageDir() string {
	return cs.PackageDir
}

func (cs *CompilationSpec) SetPackageDir(s string) {
	cs.PackageDir = s
}

func (cs *CompilationSpec) BuildSteps() []string {
	return cs.Steps
}

func (cs *CompilationSpec) ImageUnpack() bool {
	return cs.Unpack
}

func (cs *CompilationSpec) GetPreBuildSteps() []string {
	return cs.Prelude
}

func (cs *CompilationSpec) GetIncludes() []string {
	return cs.Includes
}

func (cs *CompilationSpec) GetExcludes() []string {
	return cs.Excludes
}

func (cs *CompilationSpec) GetRetrieve() []string {
	return cs.Retrieve
}

func (cs *CompilationSpec) GetFlatImage() bool { return cs.FlatImage }

func (cs *CompilationSpec) HasFilters() bool {
	if len(cs.Includes) > 0 || len(cs.Excludes) > 0 {
		return true
	}
	return false
}

// IsVirtual returns true if the spec is virtual.
// A spec is virtual if the package is empty, and it has no image source to unpack from.
func (cs *CompilationSpec) IsVirtual() bool {
	return cs.EmptyPackage() && !cs.HasImageSource()
}

func (cs *CompilationSpec) GetSeedImage() string {
	return cs.Seed
}

func (cs *CompilationSpec) GetImage() string {
	return cs.Image
}

func (cs *CompilationSpec) GetOutputPath() string {
	return cs.OutputPath
}

func (p *CompilationSpec) Rel(s string) string {
	return filepath.Join(p.GetOutputPath(), s)
}

func (cs *CompilationSpec) SetImage(s string) {
	cs.Image = s
}

func (cs *CompilationSpec) SetOutputPath(s string) {
	cs.OutputPath = s
}

func (cs *CompilationSpec) SetSeedImage(s string) {
	cs.Seed = s
}

func (cs *CompilationSpec) EmptyPackage() bool {
	return len(cs.BuildSteps()) == 0 && !cs.UnpackedPackage()
}

func (cs *CompilationSpec) UnpackedPackage() bool {
	// If package_dir was specified in the spec, we want to treat the content of the directory
	// as the root of our archive.  ImageUnpack is implied to be true. override it
	unpack := cs.ImageUnpack()
	if cs.GetPackageDir() != "" {
		unpack = true
	}
	return unpack
}

// HasImageSource returns true when the compilation spec has an image source.
// a compilation spec has an image source when it depends on other packages or have a source image
// explictly supplied
func (cs *CompilationSpec) HasImageSource() bool {
	return (cs.Package != nil && len(cs.GetPackage().GetRequires()) != 0) || cs.GetImage() != "" || (cs.RequiresFinalImages && len(cs.Package.GetRequires()) != 0)
}

func (cs *CompilationSpec) Hash() (string, error) {
	// build a signature, we want to be part of the hash only the fields that are relevant for build purposes
	signature := cs.signature()
	h, err := hashstructure.Hash(signature, hashstructure.FormatV2, nil)
	if err != nil {
		return "", err
	}
	sum, err := dirhash.HashDir(cs.Package.Path, "", dirhash.DefaultHash)
	if err != nil {
		return fmt.Sprint(h), err
	}
	return fmt.Sprint(h, sum), err
}

func (cs *CompilationSpec) CopyRetrieves(dest string) error {
	var err error
	if len(cs.Retrieve) > 0 {
		for _, s := range cs.Retrieve {
			matches, err := filepath.Glob(cs.Rel(s))

			if err != nil {
				continue
			}

			for _, m := range matches {
				err = copy.Copy(m, filepath.Join(dest, filepath.Base(m)))
			}
		}
	}
	return err
}

func (cs *CompilationSpec) genDockerfile(image string, steps []string) string {
	spec := `
FROM ` + image + `
COPY . /anisebuild
WORKDIR /anisebuild
ENV PACKAGE_NAME=` + cs.Package.GetName() + `
ENV PACKAGE_VERSION=` + cs.Package.GetVersion() + `
ENV PACKAGE_CATEGORY=` + cs.Package.GetCategory()

	if len(cs.Retrieve) > 0 {
		for _, s := range cs.Retrieve {
			spec = spec + `
ADD ` + s + ` /anisebuild/`
		}
	}

	for _, c := range cs.Copy {
		if c.Image != "" {
			copyLine := fmt.Sprintf("\nCOPY --from=%s %s %s\n", c.Image, c.Source, c.Destination)
			spec = spec + copyLine
		}
	}

	for _, s := range cs.Env {
		spec = spec + `
ENV ` + s
	}

	for _, s := range steps {
		spec = spec + `
RUN ` + s
	}
	return spec
}

// RenderBuildImage renders the dockerfile of the image used as a pre-build step
func (cs *CompilationSpec) RenderBuildImage() (string, error) {
	return cs.genDockerfile(cs.GetSeedImage(), cs.GetPreBuildSteps()), nil

}

// RenderStepImage renders the dockerfile used for the image used for building the package
func (cs *CompilationSpec) RenderStepImage(image string) (string, error) {
	return cs.genDockerfile(image, cs.BuildSteps()), nil
}

func (cs *CompilationSpec) WriteBuildImageDefinition(path string) error {
	data, err := cs.RenderBuildImage()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, []byte(data), 0644)
}

func (cs *CompilationSpec) WriteStepImageDefinition(fromimage, path string) error {
	data, err := cs.RenderStepImage(fromimage)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, []byte(data), 0644)
}

func (cs *CompilationSpec) YAML() ([]byte, error) {
	return yaml.Marshal(cs)
}

func (cs *CompilationSpec) Json() ([]byte, error) {
	return json.Marshal(cs)
}

func NewComplationSpecLoad() *CompilationSpecLoad {
	return &CompilationSpecLoad{
		DefaultPackage:      &pkg.DefaultPackage{},
		Steps:               []string{},
		Env:                 []string{},
		Prelude:             []string{},
		Retrieve:            []string{},
		BuildOptions:        options.NewDefaultCompiler(),
		RequiresFinalImages: false,
		FlatImage:           false,
	}
}

func (csl *CompilationSpecLoad) ToSpec() *CompilationSpec {
	return &CompilationSpec{
		Steps:               csl.Steps,
		Env:                 csl.Env,
		Prelude:             csl.Prelude,
		Image:               csl.Image,
		Seed:                csl.Seed,
		PackageDir:          csl.PackageDir,
		Retrieve:            csl.Retrieve,
		Unpack:              csl.Unpack,
		Includes:            csl.Includes,
		Excludes:            csl.Excludes,
		BuildOptions:        csl.BuildOptions,
		Copy:                csl.Copy,
		RequiresFinalImages: csl.RequiresFinalImages,
		Package:             csl.DefaultPackage,
		FlatImage:           csl.FlatImage,
	}
}

func (cs *CompilationSpecLoad) EmptyPackage() bool {
	return len(cs.BuildSteps()) == 0 && !cs.UnpackedPackage()
}

func (cs *CompilationSpecLoad) UnpackedPackage() bool {
	// If package_dir was specified in the spec, we want to treat the content of the directory
	// as the root of our archive.  ImageUnpack is implied to be true. override it
	unpack := cs.ImageUnpack()
	if cs.GetPackageDir() != "" {
		unpack = true
	}
	return unpack
}

// HasImageSource returns true when the compilation spec has an image source.
// a compilation spec has an image source when it depends on other packages or have a source image
// explictly supplied
func (cs *CompilationSpecLoad) HasImageSource() bool {
	return (cs.DefaultPackage != nil && len(cs.DefaultPackage.GetRequires()) != 0) || cs.GetImage() != "" || (cs.RequiresFinalImages && len(cs.DefaultPackage.GetRequires()) != 0)
}

// IsVirtual returns true if the spec is virtual.
// A spec is virtual if the package is empty, and it has no image source to unpack from.
func (cs *CompilationSpecLoad) IsVirtual() bool {
	return cs.EmptyPackage() && !cs.HasImageSource()
}

func (cs *CompilationSpecLoad) BuildSteps() []string {
	return cs.Steps
}

func (cs *CompilationSpecLoad) ImageUnpack() bool {
	return cs.Unpack
}

func (cs *CompilationSpecLoad) GetImage() string {
	return cs.Image
}

func (cs *CompilationSpecLoad) GetPackageDir() string {
	return cs.PackageDir
}

func (cs *CompilationSpecLoad) YAML() ([]byte, error) {
	return yaml.Marshal(cs)
}

func (cs *CompilationSpecLoad) Json() ([]byte, error) {
	return json.Marshal(cs)
}
