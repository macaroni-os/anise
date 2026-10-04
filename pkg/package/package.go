/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package pkg

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/macaroni-os/anise/pkg/helpers/docker"
	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"
	"github.com/macaroni-os/anise/pkg/helpers/match"
	"github.com/macaroni-os/anise/pkg/helpers/tools"
	version "github.com/macaroni-os/anise/pkg/versioner"

	gentoo "github.com/geaaru/pkgs-checker/pkg/gentoo"
	"github.com/ghodss/yaml"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
)

// Package is a package interface (TBD)
// FIXME: Currently some of the methods are returning DefaultPackages due to JSON serialization of the package
type Package interface {
	Encode(PackageDatabase) (string, error)
	Related(definitiondb PackageDatabase) Packages

	GetFingerPrint() string
	GetPackageName() string
	ImageID() string
	Requires([]*DefaultPackage) Package
	HasRequires() bool
	Conflicts([]*DefaultPackage) Package
	Revdeps(PackageDatabase) Packages
	LabelDeps(PackageDatabase, string) Packages

	GetProvides() []*DefaultPackage
	SetProvides([]*DefaultPackage) Package
	HasProvides() bool
	GetProvidePackage(string) *DefaultPackage

	GetRequires() []*DefaultPackage
	GetConflicts() []*DefaultPackage
	Expand(PackageDatabase) (Packages, error)
	SetCategory(string)

	GetName() string
	SetName(string)
	GetCategory() string
	PackageName() string

	GetVersion() string
	SetVersion(string)
	RequiresContains(PackageDatabase, Package) (bool, error)
	Matches(m Package) bool
	AtomMatches(m Package) bool
	BumpBuildVersion() error

	AddUse(use string)
	RemoveUse(use string)
	GetUses() []string

	Yaml() ([]byte, error)
	Explain()

	SetPath(string)
	GetPath() string
	Rel(string) string

	GetDescription() string
	SetDescription(string)

	AddURI(string)
	GetURI() []string

	SetLicense(string)
	GetLicense() string

	AddLabel(string, string)
	GetLabels() map[string]string
	HasLabel(string) bool
	MatchLabel(*regexp.Regexp) bool

	AddAnnotation(string, interface{})
	GetAnnotations() map[string]interface{}
	HasAnnotation(string) bool
	MatchAnnotation(*regexp.Regexp) bool
	GetAnnotationByKey(k string) interface{}

	IsHidden() bool
	IsSelector() bool
	VersionMatchSelector(string, version.Versioner) (bool, error)
	SelectorMatchVersion(string, version.Versioner) (bool, error)

	String() string
	HumanReadableString() string
	HashFingerprint(string) string

	SetBuildTimestamp(s string)
	GetBuildTimestamp() string

	Clone() Package

	GetMetadataFilePath() string
	SetTreeDir(s string)
	GetTreeDir() string

	JSON() ([]byte, error)

	GetRepository() string
	SetRepository(string)
}

const (
	PackageMetaSuffix     = "metadata.yaml"
	PackageCollectionFile = "collection.yaml"
	PackageDefinitionFile = "definition.yaml"
)

type Packages []Package

type DefaultPackages []*DefaultPackage

type PackageMap map[string]Package

func (pm PackageMap) String() string {
	rr := []string{}
	for _, r := range pm {

		rr = append(rr, r.HumanReadableString())

	}
	return fmt.Sprint(rr)
}

func (d DefaultPackage) HasRequires() bool {
	return tools.Ternary(d.PackageRequires != nil, len(d.PackageRequires) > 0, false)
}

func NewDefaultPackageFromYaml(data []byte) (*DefaultPackage, error) {
	ans, err := DefaultPackageFromYaml(data)
	if err != nil {
		return nil, err
	}
	return &ans, nil
}

// >> Unmarshallers
// DefaultPackageFromYaml decodes a package from yaml bytes
func DefaultPackageFromYaml(yml []byte) (DefaultPackage, error) {

	var unescaped DefaultPackage
	source, err := yaml.YAMLToJSON(yml)
	if err != nil {
		return DefaultPackage{}, err
	}

	rawIn := json.RawMessage(source)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		return DefaultPackage{}, err
	}
	err = json.Unmarshal(bytes, &unescaped)
	if err != nil {
		return DefaultPackage{}, err
	}
	return unescaped, nil
}

type rawPackages []map[string]interface{}

func (r rawPackages) Find(name, category, version string) map[string]interface{} {
	for _, v := range r {
		if v["name"] == name && v["category"] == category && v["version"] == version {
			return v
		}
	}
	return map[string]interface{}{}
}

func GetRawPackages(yml []byte) (rawPackages, error) {
	var rawPackages struct {
		Packages []map[string]interface{} `yaml:"packages"`
	}
	source, err := yaml.YAMLToJSON(yml)
	if err != nil {
		return []map[string]interface{}{}, err
	}

	rawIn := json.RawMessage(source)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		return []map[string]interface{}{}, err
	}
	err = json.Unmarshal(bytes, &rawPackages)
	if err != nil {
		return []map[string]interface{}{}, err
	}
	return rawPackages.Packages, nil

}

type Collection struct {
	Packages []DefaultPackage `json:"packages" yaml:"packages"`
}

func DefaultPackagesFromYAML(yml []byte) ([]DefaultPackage, error) {

	var unescaped Collection
	source, err := yaml.YAMLToJSON(yml)
	if err != nil {
		return []DefaultPackage{}, err
	}

	rawIn := json.RawMessage(source)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		return []DefaultPackage{}, err
	}
	err = json.Unmarshal(bytes, &unescaped)
	if err != nil {
		return []DefaultPackage{}, err
	}
	return unescaped.Packages, nil
}

// Major and minor gets escaped when marshalling in JSON, making compiler fails recognizing selectors for expansion
func (t *DefaultPackage) JSON() ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}

// GetMetadataFilePath returns the canonical name of an artifact metadata file
func (d *DefaultPackage) GetMetadataFilePath() string {
	return fmt.Sprintf("%s.%s", d.GetFingerPrint(), PackageMetaSuffix)
}

// DefaultPackage represent a standard package definition
type DefaultPackage struct {
	ID               int               `storm:"id,increment" json:"id" yaml:"id,omitempty"`    // primary key with auto increment
	Name             string            `json:"name" yaml:"name"`                               // Affects YAML field names too.
	Version          string            `json:"version" yaml:"version,omitempty"`               // Affects YAML field names too.
	Category         string            `json:"category" yaml:"category"`                       // Affects YAML field names too.
	UseFlags         []string          `json:"use_flags,omitempty" yaml:"use_flags,omitempty"` // Affects YAML field names too.
	State            State             `json:"state,omitempty" yaml:"state,omitempty"`
	PackageRequires  []*DefaultPackage `json:"requires" yaml:"requires,omitempty"`           // Affects YAML field names too.
	PackageConflicts []*DefaultPackage `json:"conflicts" yaml:"conflicts,omitempty"`         // Affects YAML field names too.
	Provides         []*DefaultPackage `json:"provides,omitempty" yaml:"provides,omitempty"` // Affects YAML field names too.
	Hidden           bool              `json:"hidden,omitempty" yaml:"hidden,omitempty"`     // Affects YAML field names too.

	// Annotations are used for core features/options
	Annotations map[string]interface{} `json:"annotations,omitempty" yaml:"annotations,omitempty"` // Affects YAML field names too

	// Path is set only internally when tree is loaded from disk
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	Description    string   `json:"description,omitempty" yaml:"description,omitempty"`
	Uri            []string `json:"uri,omitempty" yaml:"uri,omitempty"`
	License        string   `json:"license,omitempty" yaml:"license,omitempty"`
	BuildTimestamp string   `json:"buildtimestamp,omitempty" yaml:"buildtimestamp,omitempty"`

	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"` // Affects YAML field names too.

	TreeDir string `json:"treedir,omitempty" yaml:"treedir,omitempty"`

	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`
}

// State represent the package state
type State string

// NewPackage returns a new package
func NewPackage(name, version string, requires []*DefaultPackage, conflicts []*DefaultPackage) *DefaultPackage {
	return &DefaultPackage{
		Name:             name,
		Version:          version,
		PackageRequires:  requires,
		PackageConflicts: conflicts,
		Labels:           nil,
	}
}

// TODO: Permit to create a new package also with category.
func NewPackageWithCat(category, name, version string,
	requires []*DefaultPackage, conflicts []*DefaultPackage) *DefaultPackage {
	return &DefaultPackage{
		Name:             name,
		Category:         category,
		Version:          version,
		PackageRequires:  requires,
		PackageConflicts: conflicts,
		Labels:           nil,
	}
}

func NewPackageWithCatThin(category, name, version string) *DefaultPackage {
	return &DefaultPackage{
		Name:             name,
		Category:         category,
		Version:          version,
		PackageRequires:  []*DefaultPackage{},
		PackageConflicts: []*DefaultPackage{},
		Labels:           nil,
	}
}

func (p *DefaultPackage) SetRepository(r string) { p.Repository = r }
func (p *DefaultPackage) GetRepository() string  { return p.Repository }

func (p *DefaultPackage) SetTreeDir(s string) {
	p.TreeDir = s
}
func (p *DefaultPackage) GetTreeDir() string {
	return p.TreeDir
}
func (p *DefaultPackage) String() string {
	b, err := p.JSON()
	if err != nil {
		return fmt.Sprintf("{ id: \"%d\", name: \"%s\", version: \"%s\", category: \"%s\"  }", p.ID, p.Name, p.Version, p.Category)
	}
	return fmt.Sprintf("%s", string(b))
}

// GetFingerPrint returns a UUID of the package.
// FIXME: this needs to be unique, now just name is generalized
func (p *DefaultPackage) GetFingerPrint() string {
	return fmt.Sprintf("%s-%s-%s", p.Name, p.Category, p.Version)
}

// This method return an hash based on package name, version
// and his requires, conflicts that is used to identify
// eventually changes of the requirements of a package
// that is not rebumped.
func (p *DefaultPackage) GetComparitionHash() string {
	pt := p.ToPackageThin()
	return pt.GenerateHash()
}

func (p *DefaultPackage) HashFingerprint(salt string) string {
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%s-%s", p.GetFingerPrint(), salt))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *DefaultPackage) HumanReadableString() string {
	return fmt.Sprintf("%s/%s-%s", p.Category, p.Name, p.Version)
}

func (p *DefaultPackage) PackageName() string {
	if p.Category != "" && p.Name != "" {
		return fmt.Sprintf("%s/%s", p.Category, p.Name)
	} else if p.Category != "" {
		return p.Category
	} else {
		return p.Name
	}
}

func FromString(s string) Package {
	var unescaped DefaultPackage

	err := json.Unmarshal([]byte(s), &unescaped)
	if err != nil {
		return &unescaped
	}
	return &unescaped
}

func (p *DefaultPackage) GetPackageName() string {
	return fmt.Sprintf("%s-%s", p.Name, p.Category)
}

func (p *DefaultPackage) ImageID() string {
	return docker.StripInvalidStringsFromImage(p.GetFingerPrint())
}

// GetBuildTimestamp returns the package build timestamp
func (p *DefaultPackage) GetBuildTimestamp() string {
	return p.BuildTimestamp
}

// SetBuildTimestamp sets the package Build timestamp
func (p *DefaultPackage) SetBuildTimestamp(s string) {
	p.BuildTimestamp = s
}

// GetPath returns the path where the definition file was found
func (p *DefaultPackage) GetPath() string {
	return p.Path
}

func (p *DefaultPackage) Rel(s string) string {
	return filepath.Join(p.GetPath(), s)
}

func (p *DefaultPackage) SetPath(s string) {
	p.Path = s
}

func (p *DefaultPackage) IsSelector() bool {
	return strings.ContainsAny(p.GetVersion(), "<>=")
}

func (p *DefaultPackage) IsHidden() bool {
	return p.Hidden
}

func (p *DefaultPackage) HasLabel(label string) bool {
	return match.MapHasKey(&p.Labels, label)
}

func (p *DefaultPackage) MatchLabel(r *regexp.Regexp) bool {
	return match.MapMatchRegex(&p.Labels, r)
}

func (p DefaultPackage) IsCollection() bool {
	return fileHelper.Exists(filepath.Join(p.Path, PackageCollectionFile))
}

func (p *DefaultPackage) HasAnnotation(label string) bool {
	return match.MapIHasKey(&p.Annotations, label)
}

func (p *DefaultPackage) MatchAnnotation(r *regexp.Regexp) bool {
	return match.MapIMatchRegex(&p.Annotations, r)
}

// AddUse adds a use to a package
func (p *DefaultPackage) AddUse(use string) {
	for _, v := range p.UseFlags {
		if v == use {
			return
		}
	}
	p.UseFlags = append(p.UseFlags, use)
}

// RemoveUse removes a use to a package
func (p *DefaultPackage) RemoveUse(use string) {

	for i := len(p.UseFlags) - 1; i >= 0; i-- {
		if p.UseFlags[i] == use {
			p.UseFlags = append(p.UseFlags[:i], p.UseFlags[i+1:]...)
		}
	}

}

// Encode encodes the package to string.
// It returns an ID which can be used to retrieve the package later on.
func (p *DefaultPackage) Encode(db PackageDatabase) (string, error) {
	return db.CreatePackage(p)
}

func (p *DefaultPackage) Yaml() ([]byte, error) {
	j, err := p.JSON()
	if err != nil {
		return []byte{}, err
	}
	y, err := yaml.JSONToYAML(j)
	if err != nil {

		return []byte{}, err
	}
	return y, nil
}

func (p *DefaultPackage) GetName() string {
	return p.Name
}

func (p *DefaultPackage) GetVersion() string {
	return p.Version
}
func (p *DefaultPackage) SetVersion(v string) {
	p.Version = v
}
func (p *DefaultPackage) GetDescription() string {
	return p.Description
}
func (p *DefaultPackage) SetDescription(s string) {
	p.Description = s
}
func (p *DefaultPackage) GetLicense() string {
	return p.License
}
func (p *DefaultPackage) SetLicense(s string) {
	p.License = s
}
func (p *DefaultPackage) AddURI(s string) {
	p.Uri = append(p.Uri, s)
}
func (p *DefaultPackage) GetURI() []string {
	return p.Uri
}
func (p *DefaultPackage) GetCategory() string {
	return p.Category
}
func (p *DefaultPackage) SetCategory(s string) {
	p.Category = s
}

func (p *DefaultPackage) SetName(s string) {
	p.Name = s
}

func (p *DefaultPackage) GetUses() []string {
	return p.UseFlags
}
func (p *DefaultPackage) AddLabel(k, v string) {
	if p.Labels == nil {
		p.Labels = make(map[string]string, 0)
	}
	p.Labels[k] = v
}
func (p *DefaultPackage) AddAnnotation(k string, v interface{}) {
	if p.Annotations == nil {
		p.Annotations = make(map[string]interface{}, 0)
	}
	p.Annotations[k] = v
}
func (p *DefaultPackage) GetLabels() map[string]string {
	return p.Labels
}
func (p *DefaultPackage) GetAnnotations() map[string]interface{} {
	return p.Annotations
}
func (p *DefaultPackage) GetAnnotationByKey(k string) (ans interface{}) {
	if p.HasAnnotation(k) {
		ans = p.Annotations[k]
	}

	return ans
}

func (p *DefaultPackage) GetProvidePackage(pname string) *DefaultPackage {
	var ans *DefaultPackage = nil

	if p.HasProvides() {
		for idx, prov := range p.Provides {
			if prov.PackageName() == pname {
				ans = p.Provides[idx]
				break
			}
		}
	}

	return ans
}
func (p *DefaultPackage) GetProvides() []*DefaultPackage {
	return p.Provides
}
func (p *DefaultPackage) SetProvides(req []*DefaultPackage) Package {
	p.Provides = req
	return p
}
func (p *DefaultPackage) HasProvides() bool {
	if len(p.Provides) > 0 {
		return true
	}
	return false
}
func (p *DefaultPackage) GetRequires() []*DefaultPackage {
	return p.PackageRequires
}
func (p *DefaultPackage) GetConflicts() []*DefaultPackage {
	return p.PackageConflicts
}
func (p *DefaultPackage) Requires(req []*DefaultPackage) Package {
	p.PackageRequires = req
	return p
}
func (p *DefaultPackage) Conflicts(req []*DefaultPackage) Package {
	p.PackageConflicts = req
	return p
}
func (p *DefaultPackage) Clone() Package {
	new := &DefaultPackage{}
	copier.Copy(&new, &p)
	return new
}
func (p *DefaultPackage) Matches(m Package) bool {
	if p.GetFingerPrint() == m.GetFingerPrint() {
		return true
	}
	return false
}

func (p *DefaultPackage) AtomMatches(m Package) bool {
	if p.GetName() == m.GetName() && p.GetCategory() == m.GetCategory() {
		return true
	}
	return false
}

func (p *DefaultPackage) Expand(definitiondb PackageDatabase) (Packages, error) {
	var versionsInWorld Packages

	all, err := definitiondb.FindPackages(p)
	if err != nil {
		return nil, err
	}
	for _, w := range all {
		match, err := p.SelectorMatchVersion(w.GetVersion(), nil)
		if err != nil {
			return nil, err
		}
		if match {
			versionsInWorld = append(versionsInWorld, w)
		}
	}

	return versionsInWorld, nil
}

func (p *DefaultPackage) Revdeps(definitiondb PackageDatabase) Packages {
	var versionsInWorld Packages
	for _, w := range definitiondb.World() {
		if w.Matches(p) {
			continue
		}
		for _, re := range w.GetRequires() {
			if re.Matches(p) {
				versionsInWorld = append(versionsInWorld, w)
				versionsInWorld = append(versionsInWorld, w.Revdeps(definitiondb)...)
			}
		}
	}

	return versionsInWorld
}

func (p *DefaultPackage) ToPackageThin() *PackageThin {
	ans := &PackageThin{
		Name:      p.Name,
		Category:  p.Category,
		Version:   p.Version,
		Requires:  []*PackageThin{},
		Conflicts: []*PackageThin{},
		Provides:  []*PackageThin{},
		UseFlags:  p.UseFlags,
	}

	if len(p.PackageRequires) > 0 {
		for idx, _ := range p.PackageRequires {
			ans.Requires = append(ans.Requires,
				p.PackageRequires[idx].ToPackageThin())
		}
	}

	if len(p.PackageConflicts) > 0 {
		for idx, _ := range p.PackageConflicts {
			ans.Conflicts = append(ans.Conflicts,
				p.PackageConflicts[idx].ToPackageThin())
		}
	}

	if len(p.Provides) > 0 {
		for idx, _ := range p.Provides {
			ans.Provides = append(ans.Provides,
				p.Provides[idx].ToPackageThin())
		}
	}

	return ans
}

func (p *DefaultPackage) ToGentooPackage() (*gentoo.GentooPackage, error) {

	var cond gentoo.PackageCond

	if strings.HasPrefix(p.Version, ">=") {
		cond = gentoo.PkgCondGreaterEqual
	} else if strings.HasPrefix(p.Version, "<=") {
		cond = gentoo.PkgCondLessEqual
	} else if strings.HasPrefix(p.Version, "!=") {
		cond = gentoo.PkgCondNot
	} else if strings.HasPrefix(p.Version, "=") {
		cond = gentoo.PkgCondEqual
	} else if strings.HasPrefix(p.Version, ">") {
		cond = gentoo.PkgCondGreater
	} else if strings.HasPrefix(p.Version, "<") {
		cond = gentoo.PkgCondLess
	}

	ans, err := gentoo.ParsePackageStr(
		fmt.Sprintf("%s-%s",
			p.PackageName(),
			strings.Trim(p.GetVersion(), "><=!"),
		),
	)

	if err != nil {
		return nil, err
	}

	ans.Condition = cond
	ans.Repository = p.Repository
	ans.License = p.License
	ans.UseFlags = p.UseFlags

	return ans, nil
}

func (p *DefaultPackage) Admit(a *DefaultPackage) (bool, error) {
	if len(p.PackageRequires) == 0 && len(p.PackageConflicts) == 0 {
		return true, nil
	}

	agentoo, err := a.ToGentooPackage()
	if err != nil {
		return false, err
	}

	if len(p.PackageRequires) > 0 {
		// PRE: the Requires are all selector
		for _, r := range p.PackageRequires {
			if r.AtomMatches(a) {
				greq, err := r.ToGentooPackage()
				if err != nil {
					return false, err
				}

				admitted, err := greq.Admit(agentoo)
				if err != nil {
					return false, err
				}

				if !admitted {
					return false, nil
				}
			} else if a.HasProvides() {

				for _, prov := range a.Provides {
					if r.AtomMatches(prov) {

						greq, err := r.ToGentooPackage()
						if err != nil {
							return false, err
						}
						gprov, err := prov.ToGentooPackage()
						if err != nil {
							return false, err
						}

						admitted, err := greq.Admit(gprov)
						if err != nil {
							return false, err
						}

						if !admitted {
							return false, nil
						}
					}
				}

			}
		}
	}

	if len(p.PackageConflicts) > 0 {
		for _, c := range p.PackageConflicts {
			if c.AtomMatches(a) {
				gconflict, err := c.ToGentooPackage()
				if err != nil {
					return false, err
				}

				admitted, err := gconflict.Admit(agentoo)
				if err != nil {
					return false, err
				}

				if admitted {
					// If the package is admitted means that
					// match the conflicts
					return false, nil
				}
			} else if a.HasProvides() {

				for _, prov := range a.Provides {
					if c.AtomMatches(prov) {

						gconflict, err := c.ToGentooPackage()
						if err != nil {
							return false, err
						}
						gprov, err := prov.ToGentooPackage()
						if err != nil {
							return false, err
						}

						admitted, err := gconflict.Admit(gprov)
						if err != nil {
							return false, err
						}

						if admitted {
							// If the package is admitted means that
							// match the conflicts
							return false, nil
						}
					}
				}

			}
		}
	}

	return true, nil
}

func walkPackage(p Package, definitiondb PackageDatabase, visited map[string]interface{}) Packages {
	var versionsInWorld Packages
	if _, ok := visited[p.HumanReadableString()]; ok {
		return versionsInWorld
	}
	visited[p.HumanReadableString()] = true

	revdeps, _ := definitiondb.GetRevdeps(p)
	for _, r := range revdeps {
		versionsInWorld = append(versionsInWorld, r)
	}

	if !p.IsSelector() {
		versionsInWorld = append(versionsInWorld, p)
	}

	for _, re := range p.GetRequires() {
		versions, _ := re.Expand(definitiondb)
		for _, r := range versions {

			versionsInWorld = append(versionsInWorld, r)
			versionsInWorld = append(versionsInWorld, walkPackage(r, definitiondb, visited)...)
		}

	}
	for _, re := range p.GetConflicts() {
		versions, _ := re.Expand(definitiondb)
		for _, r := range versions {

			versionsInWorld = append(versionsInWorld, r)
			versionsInWorld = append(versionsInWorld, walkPackage(r, definitiondb, visited)...)

		}
	}
	return versionsInWorld.Unique()
}

func (p *DefaultPackage) Related(definitiondb PackageDatabase) Packages {
	return walkPackage(p, definitiondb, map[string]interface{}{})
}

func (p *DefaultPackage) LabelDeps(definitiondb PackageDatabase, labelKey string) Packages {
	var pkgsWithLabelInWorld Packages
	// TODO: check if integrate some index to improve
	// research instead of iterate all list.
	for _, w := range definitiondb.World() {
		if w.HasLabel(labelKey) {
			pkgsWithLabelInWorld = append(pkgsWithLabelInWorld, w)
		}
	}

	return pkgsWithLabelInWorld
}

func DecodePackage(ID string, db PackageDatabase) (Package, error) {
	return db.GetPackage(ID)
}

func (pack *DefaultPackage) scanRequires(definitiondb PackageDatabase, s Package, visited map[string]interface{}) (bool, error) {
	if _, ok := visited[pack.HumanReadableString()]; ok {
		return false, nil
	}
	visited[pack.HumanReadableString()] = true
	p, err := definitiondb.FindPackage(pack)
	if err != nil {
		p = pack //relax things
		//return false, errors.Wrap(err, "Package not found in definition db")
	}

	for _, re := range p.GetRequires() {
		if re.Matches(s) {
			return true, nil
		}

		packages, _ := re.Expand(definitiondb)
		for _, pa := range packages {
			if pa.Matches(s) {
				return true, nil
			}
		}
		if contains, err := re.scanRequires(definitiondb, s, visited); err == nil && contains {
			return true, nil
		}
	}

	return false, nil
}

// RequiresContains recursively scans into the database packages dependencies to find a match with the given package
// It is used by the solver during uninstall.
func (pack *DefaultPackage) RequiresContains(definitiondb PackageDatabase, s Package) (bool, error) {
	return pack.scanRequires(definitiondb, s, make(map[string]interface{}))
}

// Best returns the best version of the package (the most bigger) from a list
// Accepts a versioner interface to change the ordering policy. If null is supplied
// It defaults to version.WrappedVersioner which supports both semver and debian versioning
func (set Packages) Best(v version.Versioner) Package {
	if v == nil {
		v = &version.WrappedVersioner{}
	}
	var versionsMap map[string]Package = make(map[string]Package)
	if len(set) == 0 {
		panic("Best needs a list with elements")
	}

	versionsRaw := []string{}
	for _, p := range set {
		versionsRaw = append(versionsRaw, p.GetVersion())
		versionsMap[p.GetVersion()] = p
	}
	sorted := v.Sort(versionsRaw)

	return versionsMap[sorted[len(sorted)-1]]
}

func (set Packages) Find(packageName string) (Package, error) {
	for _, p := range set {
		if p.GetPackageName() == packageName {
			return p, nil
		}
	}

	return &DefaultPackage{}, errors.New("package not found")
}

func (set Packages) Unique() Packages {
	var result Packages
	uniq := make(map[string]Package)
	for _, p := range set {
		uniq[p.GetFingerPrint()] = p
	}
	for _, p := range uniq {
		result = append(result, p)
	}
	return result
}

func (p *DefaultPackage) GetRuntimePackage() (*DefaultPackage, error) {
	var r *DefaultPackage
	if p.IsCollection() {
		collectionFile := filepath.Join(p.Path, PackageCollectionFile)
		dat, err := ioutil.ReadFile(collectionFile)
		if err != nil {
			return r, errors.Wrapf(err, "failed while reading '%s'", collectionFile)
		}
		coll, err := DefaultPackagesFromYAML(dat)
		if err != nil {
			return r, errors.Wrapf(err, "failed while parsing YAML '%s'", collectionFile)
		}
		for _, c := range coll {
			if c.Matches(p) {
				r = &c
				break
			}
		}
	} else {
		definitionFile := filepath.Join(p.Path, PackageDefinitionFile)
		dat, err := ioutil.ReadFile(definitionFile)
		if err != nil {
			return r, errors.Wrapf(err, "failed while reading '%s'", definitionFile)
		}
		d, err := DefaultPackageFromYaml(dat)
		if err != nil {
			return r, errors.Wrapf(err, "failed while parsing YAML '%s'", definitionFile)
		}
		r = &d
	}
	return r, nil
}

func (p *DefaultPackage) Explain() {

	fmt.Println("====================")
	fmt.Println("Name: ", p.GetName())
	fmt.Println("Category: ", p.GetCategory())
	fmt.Println("Version: ", p.GetVersion())

	for _, req := range p.GetRequires() {
		fmt.Println("\t-> ", req)
	}

	for _, con := range p.GetConflicts() {
		fmt.Println("\t!! ", con)
	}

	fmt.Println("====================")

}

func (p *DefaultPackage) BumpBuildVersion() error {
	cat := p.Category
	if cat == "" {
		// Use fake category for parse package
		cat = "app"
	}
	gp, err := gentoo.ParsePackageStr(
		fmt.Sprintf("%s/%s-%s", cat,
			p.Name, p.GetVersion()))
	if err != nil {
		return errors.Wrap(err, "Error on parser version")
	}

	buildPrefix := ""
	buildId := 0

	if gp.VersionBuild != "" {
		// Check if version build is a number
		buildId, err = strconv.Atoi(gp.VersionBuild)
		if err == nil {
			goto end
		}
		// POST: is not only a number

		// TODO: check if there is a better way to handle all use cases.

		r1 := regexp.MustCompile(`^r[0-9]*$`)
		if r1 == nil {
			return errors.New("Error on create regex for -r[0-9]")
		}
		if r1.MatchString(gp.VersionBuild) {
			buildId, err = strconv.Atoi(strings.ReplaceAll(gp.VersionBuild, "r", ""))
			if err == nil {
				buildPrefix = "r"
				goto end
			}
		}

		p1 := regexp.MustCompile(`^p[0-9]*$`)
		if p1 == nil {
			return errors.New("Error on create regex for -p[0-9]")
		}
		if p1.MatchString(gp.VersionBuild) {
			buildId, err = strconv.Atoi(strings.ReplaceAll(gp.VersionBuild, "p", ""))
			if err == nil {
				buildPrefix = "p"
				goto end
			}
		}

		rc1 := regexp.MustCompile(`^rc[0-9]*$`)
		if rc1 == nil {
			return errors.New("Error on create regex for -rc[0-9]")
		}
		if rc1.MatchString(gp.VersionBuild) {
			buildId, err = strconv.Atoi(strings.ReplaceAll(gp.VersionBuild, "rc", ""))
			if err == nil {
				buildPrefix = "rc"
				goto end
			}
		}

		// Check if version build contains a dot
		dotIdx := strings.LastIndex(gp.VersionBuild, ".")
		if dotIdx > 0 {
			buildPrefix = gp.VersionBuild[0 : dotIdx+1]
			bVersion := gp.VersionBuild[dotIdx+1:]
			buildId, err = strconv.Atoi(bVersion)
			if err == nil {
				goto end
			}
		}

		buildPrefix = gp.VersionBuild + "."
		buildId = 0
	}

end:

	buildId++
	p.Version = fmt.Sprintf("%s%s+%s%d",
		gp.Version, gp.VersionSuffix, buildPrefix, buildId)

	return nil
}

func (p *DefaultPackage) SelectorMatchVersion(ver string, v version.Versioner) (bool, error) {
	if !p.IsSelector() {
		return false, errors.New("Package is not a selector")
	}
	if v == nil {
		v = &version.WrappedVersioner{}
	}

	return v.ValidateSelector(ver, p.GetVersion()), nil
}

func (p *DefaultPackage) VersionMatchSelector(selector string, v version.Versioner) (bool, error) {
	if v == nil {
		v = &version.WrappedVersioner{}
	}

	return v.ValidateSelector(p.GetVersion(), selector), nil
}
