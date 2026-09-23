/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package tree

import (
	"fmt"
	"strings"

	cfg "github.com/macaroni-os/anise/pkg/config"
	"github.com/macaroni-os/anise/pkg/helpers"
	pkg "github.com/macaroni-os/anise/pkg/package"

	"github.com/geaaru/pkgs-checker/pkg/gentoo"
	_gentoo "github.com/geaaru/pkgs-checker/pkg/gentoo"
)

// The ForestGuard object is used to
// analize and iterate over TreeIdx objects.
type ForestGuard struct {
	Config *cfg.AniseConfig
	Trees  []*TreeIdx
}

func NewForestGuard(config *cfg.AniseConfig) *ForestGuard {
	return &ForestGuard{
		Config: config,
		Trees:  []*TreeIdx{},
	}
}

func (fg *ForestGuard) GetTrees() []*TreeIdx        { return fg.Trees }
func (fg *ForestGuard) GetConfig() *cfg.AniseConfig { return fg.Config }

func (fg *ForestGuard) LoadTrees(tpaths []string) error {
	for _, t := range tpaths {
		tIdx := NewTreeIdx(t, true).DetectMode()
		err := tIdx.Read(t)
		if err != nil {
			return err
		}

		fg.Trees = append(fg.Trees, tIdx)
	}

	return nil
}

func (fg *ForestGuard) resolveCategory(name string) (string, error) {
	matches := []string{}
	ans := ""

	for _, ti := range fg.Trees {
		for k, _ := range ti.Map {
			words := strings.Split(k, "/")
			if words[1] == name {
				ans = words[0]
				if len(matches) == 0 {
					matches = append(matches, k)
				} else if len(matches) > 1 && matches[0] != k {
					return "", fmt.Errorf("multiple matches for %s:\n%s",
						name, strings.Join(matches, ","))
				}
			}
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no matching for package with name %s",
			name)
	}

	return ans, nil
}

func (fg *ForestGuard) SearchPackage(p *pkg.DefaultPackage) ([]*TreeIdx, error) {
	ans := []*TreeIdx{}

	gps, _ := p.ToGentooPackage()

	for _, ti := range fg.Trees {

		versions, present := ti.GetPackageVersions(p.PackageName())
		if !present {
			continue
		}

		tProcessed := NewTreeIdx(ti.TreePath, ti.Compress)
		tProcessed.BaseDir = ti.BaseDir

		for _, ver := range versions {
			pkg2check := &pkg.DefaultPackage{
				Name:     p.Name,
				Category: p.Category,
				Version:  ver.Version,
			}

			gp, _ := pkg2check.ToGentooPackage()
			admitted, err := gps.Admit(gp)
			if err != nil {
				return ans, err
			}

			if admitted {
				tProcessed.AddPackage(pkg2check.PackageName(), ver)
			}
		}

		if tProcessed.HasPackages() {
			ans = append(ans, tProcessed)
		}
	}

	return ans, nil
}

func (fg *ForestGuard) SearchProvides(p *pkg.DefaultPackage) ([]*TreeIdx, error) {
	ans := []*TreeIdx{}

	for _, ti := range fg.Trees {

		provs, present := ti.GetPackageProvides(p.PackageName())
		if !present {
			continue
		}

		tProcessed := NewTreeIdx(ti.TreePath, ti.Compress)
		tProcessed.BaseDir = ti.BaseDir

		for _, prov := range provs {

			gprov, _ := gentoo.ParsePackageStr(fmt.Sprintf("%s-%s",
				prov.PkgName, prov.PkgVersion))

			pkg2check := &pkg.DefaultPackage{
				Name:     gprov.GetPN(),
				Category: gprov.Category,
				Version:  prov.PkgVersion,
			}

			admitted, err := p.Admit(pkg2check)
			if err != nil {
				continue
			}

			if admitted {
				tProcessed.AddProvide(p.PackageName(), prov)
			}
		}

		if tProcessed.HasProvides() {
			ans = append(ans, tProcessed)
		}
	}

	return ans, nil
}

func (fg *ForestGuard) ResolveSelector(p string) (*pkg.DefaultPackage, error) {
	var err error
	ver := ">=0"
	cat := ""
	name := ""
	cond := _gentoo.PackageCond(_gentoo.PkgCondInvalid)

	if strings.Contains(p, "@") || !strings.Contains(p, "/") {

		pinfo := strings.Split(p, "@")
		if len(pinfo) > 1 {
			ver = pinfo[1]
		}
		cat, name, cond = helpers.PackageResolveSplit(p)

		if cond != _gentoo.PkgCondInvalid && ver != "" {
			ver = cond.String() + ver
		}

		if cat == "" {
			// POST: searching between all keys of the indexes a match of
			//       the name.
			cat, err = fg.resolveCategory(name)
			if err != nil {
				return nil, err
			}
		}

	} else {
		// POST: package string contains / but not @

		gp, err := _gentoo.ParsePackageStr(p)
		if err != nil {
			return nil, err
		}

		if gp.Version == "" {
			gp.Version = "0"
			gp.Condition = _gentoo.PkgCondGreaterEqual
		}

		ver = helpers.GentooVersion(gp)
		cat = gp.Category
		name = gp.Name
	}

	ans := &pkg.DefaultPackage{
		Name:     name,
		Category: cat,
		Version:  ver,
		Uri:      make([]string, 0),
	}

	return ans, nil
}

func (fg *ForestGuard) Search(p string) ([]*TreeIdx, error) {
	ans := []*TreeIdx{}

	pkg2search, err := fg.ResolveSelector(p)
	if err != nil {
		return ans, err
	}

	return fg.SearchPackage(pkg2search)
}
