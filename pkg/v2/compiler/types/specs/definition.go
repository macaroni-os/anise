/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package specs

import (
	pkg "github.com/macaroni-os/anise/pkg/package"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/options"
)

type CopyField struct {
	Package     *pkg.DefaultPackage `json:"package,omitempty" yaml:"package,omitempty"`
	Image       string              `json:"image,omitempty" yaml:"image,omitempty"`
	Source      string              `json:"source" yaml:"source"`
	Destination string              `json:"destination" yaml:"destination"`
}

type CompilationSpec struct {
	Steps      []string `json:"steps" yaml:"steps"` // Are run inside a container and the result layer diff is saved
	Env        []string `json:"env" yaml:"env"`
	Prelude    []string `json:"prelude" yaml:"prelude"` // Are run inside the image which will be our builder
	Image      string   `json:"image" yaml:"image"`
	Seed       string   `json:"seed" yaml:"seed"`
	PackageDir string   `json:"package_dir" yaml:"package_dir"`

	Retrieve []string `json:"retrieve" yaml:"retrieve"`

	OutputPath string   `json:"-" yaml:"-"` // Where the build processfiles go
	Unpack     bool     `json:"unpack" yaml:"unpack"`
	Includes   []string `json:"includes" yaml:"includes"`
	Excludes   []string `json:"excludes" yaml:"excludes"`

	BuildOptions *options.Compiler `json:"build_options" yaml:"build_options"`

	Copy []CopyField `json:"copy" yaml:"copy"`

	RequiresFinalImages bool `json:"requires_final_images" yaml:"requires_final_images"`

	Package *pkg.DefaultPackage `json:"package" yaml:"package"`
}

type CompilationSpecLoad struct {
	*pkg.DefaultPackage `json:"-,inline" yaml:"-,inline"`

	Steps      []string `json:"steps,omitempty" yaml:"steps,omitempty"` // Are run inside a container and the result layer diff is saved
	Env        []string `json:"env,omitempty" yaml:"env,omitempty"`
	Prelude    []string `json:"prelude,omitempty" yaml:"prelude,omitempty"` // Are run inside the image which will be our builder
	Image      string   `json:"image,omitempty" yaml:"image,omitempty"`
	Seed       string   `json:"seed,omitempty" yaml:"seed,omitempty"`
	PackageDir string   `json:"package_dir,omitempty" yaml:"package_dir,omitempty"`

	Retrieve []string `json:"retrieve,omitempty" yaml:"retrieve,omitempty"`

	OutputPath string   `json:"-" yaml:"-"` // Where the build processfiles go
	Unpack     bool     `json:"unpack,omitempty" yaml:"unpack,omitempty"`
	Includes   []string `json:"includes,omitempty" yaml:"includes,omitempty"`
	Excludes   []string `json:"excludes,omitempty" yaml:"excludes,omitempty"`

	BuildOptions *options.Compiler `json:"build_options,omitempty" yaml:"build_options,omitempty"`

	Copy []CopyField `json:"copy,omitempty" yaml:"copy,omitempty"`

	RequiresFinalImages bool `json:"requires_final_images,omitempty" yaml:"requires_final_images,omitempty"`
}

// Signature is a portion of the spec that yields a signature for the hash
type Signature struct {
	Image               string
	Steps               []string
	PackageDir          string
	Prelude             []string
	Seed                string
	Env                 []string
	Retrieve            []string
	Unpack              bool
	Includes            []string
	Excludes            []string
	Copy                []CopyField
	Requires            pkg.DefaultPackages
	RequiresFinalImages bool
}

type Compilationspecs []CompilationSpec
