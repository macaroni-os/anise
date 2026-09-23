/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package tree

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"
	. "github.com/macaroni-os/anise/pkg/logger"
	pkg "github.com/macaroni-os/anise/pkg/package"

	"github.com/geaaru/pkgs-checker/pkg/gentoo"
	zstd "github.com/klauspost/compress/zstd"
	"gopkg.in/yaml.v3"
)

const (
	IDX_FILE = ".anise-idx.json"
)

type TreeIdx struct {
	Map      map[string][]*TreeIdxPkg     `json:"packages,omitempty" yaml:"packages,omitempty"`
	Provides map[string][]*TreeIdxProvide `json:"provides,omitempty" yaml:"provides,omitempty"`
	BaseDir  string                       `json:"basedir,omitempty" yaml:"basedir,omitempty"`

	Compress bool   `json:"-" yaml:"-"`
	TreePath string `json:"-" yaml:"-"`
}

type GenOpts struct {
	DryRun   bool
	OnlyMain bool
}

type TreeIdxPkg struct {
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Path    string `json:"path,omitempty" yaml:"path,omitempty"`
}

type TreeIdxProvide struct {
	PkgName    string `json:"package,omitempty" yaml:"package,omitempty"`
	PkgVersion string `json:"package_version,omitempty" yaml:"package_version,omitempty"`
	Version    string `json:"version,omitempty" yaml:"version,omitempty"`
}

func (tp *TreeIdxPkg) IsCollection() bool {
	file := filepath.Base(tp.Path)
	if file == pkg.PackageDefinitionFile {
		return false
	}
	return true
}

func NewTreeIdx(tpath string, compress bool) *TreeIdx {
	return &TreeIdx{
		Map:      make(map[string][]*TreeIdxPkg, 0),
		Provides: make(map[string][]*TreeIdxProvide, 0),
		TreePath: tpath,
		Compress: compress,
	}
}

func (t *TreeIdx) DetectMode() *TreeIdx {
	f := filepath.Join(t.TreePath, IDX_FILE)
	if fileHelper.Exists(f) {
		t.Compress = false
	} else if fileHelper.Exists(f + ".zstd") {
		t.Compress = true
	}

	return t
}

func (t *TreeIdx) HasIndex() bool {
	ans := false
	f := filepath.Join(t.TreePath, IDX_FILE)
	if fileHelper.Exists(f) {
		ans = true
	} else if fileHelper.Exists(f + ".zstd") {
		ans = true
	}

	return ans
}

func (t *TreeIdx) AddPackage(name string, p *TreeIdxPkg) {
	if v, ok := t.Map[name]; ok {
		t.Map[name] = append(v, p)
	} else {
		t.Map[name] = []*TreeIdxPkg{p}
	}
}

func (t *TreeIdx) AddProvide(name string, p *TreeIdxProvide) {
	if v, ok := t.Provides[name]; ok {
		t.Provides[name] = append(v, p)
	} else {
		t.Provides[name] = []*TreeIdxProvide{p}
	}
}

func (t *TreeIdx) GetPackageVersions(name string) ([]*TreeIdxPkg, bool) {
	val, ok := t.Map[name]
	return val, ok
}

func (t *TreeIdx) GetPackageProvides(name string) ([]*TreeIdxProvide, bool) {
	val, ok := t.Provides[name]
	return val, ok
}

func (t *TreeIdx) GetPackageVersion(name, version string) (*TreeIdxPkg, bool) {
	val, ok := t.Map[name]
	for _, v := range val {
		if v.Version == version {
			return v, ok
		}
	}
	return nil, false
}

func (t *TreeIdx) Write() error {
	fpath := filepath.Join(t.TreePath, IDX_FILE)
	if t.Compress {
		fpath += ".zstd"
	}

	data, err := json.Marshal(t)
	if err != nil {
		return err
	}

	buffer := bytes.NewBuffer(data)

	dst, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if t.Compress {
		enc, err := zstd.NewWriter(dst)
		if err != nil {
			return err
		}

		_, err = io.Copy(enc, buffer)
		if err != nil {
			enc.Close()
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}
	} else {
		_, err = io.Copy(dst, buffer)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *TreeIdx) ToYAML() ([]byte, error) {
	return yaml.Marshal(t)
}

func (t *TreeIdx) ToJSON() ([]byte, error) {
	return json.Marshal(t)
}

func (t *TreeIdx) Merge(tI *TreeIdx) {
	for k, v := range tI.Map {
		for _, tp := range v {
			t.AddPackage(k, tp)
		}
	}
	for k, v := range tI.Provides {
		for _, prov := range v {
			t.AddProvide(k, prov)
		}
	}
}

func (t *TreeIdx) HasPackages() bool {
	return len(t.Map) > 0
}

func (t *TreeIdx) HasProvides() bool {
	return len(t.Provides) > 0
}

func (t *TreeIdx) Read(treeDir string) error {
	// I consider that the tree will be
	// with the index file of the upper directory
	// that contains the packages of all sub-directories.

	idxfile := filepath.Join(treeDir, IDX_FILE)
	if t.Compress {
		idxfile += ".zstd"
	}

	if !fileHelper.Exists(idxfile) {
		return errors.New(fmt.Sprintf("File %s doesn't exists",
			idxfile))
	}

	idxf, err := os.Open(idxfile)
	if err != nil {
		return err
	}
	defer idxf.Close()

	var reader io.Reader
	if t.Compress {
		d, err := zstd.NewReader(idxf)
		if err != nil {
			return err
		}
		defer d.Close()

		reader = d
	} else {
		reader = idxf
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, t)
	if err != nil {
		return err
	}

	t.TreePath = treeDir

	return nil
}

func (t *TreeIdx) Generate(treeDir string, opts *GenOpts) error {
	var err error

	if treeDir == "" {
		return errors.New("Invalid tree directory")
	}

	if treeDir[len(treeDir)-1:len(treeDir)] == "/" {
		treeDir = treeDir[0 : len(treeDir)-1]
	}

	if !filepath.IsAbs(treeDir) {
		treeDir, err = filepath.Abs(treeDir)
		if err != nil {
			return err
		}
	}
	base := filepath.Dir(treeDir)

	Debug(fmt.Sprintf("Generating tree %s (base %s)...", treeDir, base))

	tm, err := t.generateIdxDir(treeDir, base, opts)
	if err != nil {
		return err
	}

	t.Merge(tm)

	t.BaseDir, _ = filepath.Rel(treeDir, base)
	return nil
}

func (t *TreeIdx) generateIdxDir(dir, base string, opts *GenOpts) (*TreeIdx, error) {
	ans := NewTreeIdx(dir, t.Compress)

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.New("Error on readdir " + dir + ":" + err.Error())
	}

	for _, file := range dirEntries {
		f := filepath.Join(dir, file.Name())
		if file.IsDir() {
			tChildren, err := t.generateIdxDir(f, base, opts)
			if err != nil {
				return nil, err
			}

			ans.Merge(tChildren)

		} else if file.Name() == pkg.PackageDefinitionFile {

			dp, err := ReadDefinitionFile(f)
			if err != nil {
				return nil, fmt.Errorf("Error on parse file %s: %s",
					f, err.Error())
			}

			relf, err := filepath.Rel(base, f)
			if err != nil {
				return nil, err
			}

			ans.AddPackage(dp.PackageName(), &TreeIdxPkg{
				Version: dp.GetVersion(),
				Path:    relf,
			})

			if dp.HasProvides() {

				for _, prov := range dp.GetProvides() {

					ans.AddProvide(prov.PackageName(),
						&TreeIdxProvide{
							Version:    prov.GetVersion(),
							PkgName:    dp.PackageName(),
							PkgVersion: dp.GetVersion(),
						})

				}
			}

		} else if file.Name() == pkg.PackageCollectionFile {

			c, err := ReadCollectionFile(f)
			if err != nil {
				return nil, fmt.Errorf("Error on parse file %s: %s",
					f, err.Error())
			}

			relf, _ := filepath.Rel(base, f)

			for _, p := range c.Packages {
				ans.AddPackage(p.PackageName(), &TreeIdxPkg{
					Version: p.GetVersion(),
					Path:    relf,
				})

				if p.HasProvides() {

					for _, prov := range p.GetProvides() {

						ans.AddProvide(p.PackageName(),
							&TreeIdxProvide{
								Version:    prov.GetVersion(),
								PkgName:    p.PackageName(),
								PkgVersion: p.GetVersion(),
							})

					}
				}

			}
		}

	}

	if !opts.DryRun {
		ans.BaseDir, _ = filepath.Rel(dir, base)

		if !opts.OnlyMain || (opts.OnlyMain && ans.BaseDir == "..") {
			// Write index file.
			err = ans.Write()
			if err != nil {
				return nil, err
			}
		}
	}

	return ans, nil
}

// Split every TreeIdx with multiple versions in multiple
// TreeIdx with a single version. In addition, removed duplicated
// version. This helps sorting and elaboration.
// The returned array is only with version of the specified
// package name.
func FragmentTrees(indexes *[]*TreeIdx, pkgname string, reverse bool) *[]*TreeIdx {
	tidxs := *indexes
	ans := []*TreeIdx{}

	versions := make(map[string]bool, 0)

	for i := range tidxs {
		for k, v := range tidxs[i].Map {

			if k != pkgname {
				continue
			}

			for tip := range v {

				if _, present := versions[v[tip].Version]; present {
					continue
				}
				versions[v[tip].Version] = true
				nt := NewTreeIdx(tidxs[i].TreePath, tidxs[i].Compress)
				nt.BaseDir = tidxs[i].BaseDir
				nt.AddPackage(k, v[tip])

				ans = append(ans, nt)
			}
		}
	}

	sort.Slice(ans[:], func(i, j int) bool {

		tpi, _ := ans[i].Map[pkgname]
		tpj, _ := ans[j].Map[pkgname]

		ti := tpi[0].Version
		tj := tpj[0].Version

		gpi, _ := gentoo.ParsePackageStr(fmt.Sprintf("%s-%s", pkgname, ti))
		gpj, _ := gentoo.ParsePackageStr(fmt.Sprintf("%s-%s", pkgname, tj))

		if reverse {
			c, _ := gpi.GreaterThanOrEqual(gpj)
			return c
		} else {
			c, _ := gpi.LessThanOrEqual(gpj)
			return c
		}

	})

	return &ans
}
