/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package pkg

import (
	"fmt"

	gentoo "github.com/geaaru/pkgs-checker/pkg/gentoo"
)

func (c *Collection) GetPackage(pkgName, version string) (*DefaultPackage, error) {
	for idx, p := range c.Packages {
		if p.PackageName() == pkgName && p.GetVersion() == version {
			return &c.Packages[idx], nil
		}
	}

	return nil, fmt.Errorf("Package %s-%s not in collection", pkgName, version)
}

func (c *Collection) GetPackageFromString(pkgstr string) (*DefaultPackage, error) {
	gpkg, err := gentoo.ParsePackageStr(pkgstr)
	if err != nil {
		return nil, err
	}

	return c.GetPackageFromGentooPkg(gpkg)
}

func (c *Collection) GetPackageFromGentooPkg(g *gentoo.GentooPackage) (*DefaultPackage, error) {
	for idx := range c.Packages {
		p, _ := c.Packages[idx].ToGentooPackage()
		if matchedPkg, _ := p.Equal(g); matchedPkg {
			return &c.Packages[idx], nil
		}
	}

	return nil, fmt.Errorf("package %s not found in collection", g.GetPVR())
}
