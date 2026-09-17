/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package artifact

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	gentoo "github.com/geaaru/pkgs-checker/pkg/gentoo"
	yaml "gopkg.in/yaml.v3"
)

type ArtifactsMap struct {
	Artifacts map[string][]*PackageArtifact `json:"artifacts" yaml:"artifacts"`
}

type ArtifactsPack struct {
	Artifacts []*PackageArtifact `json:"artifacts" yaml:"artifacts"`
}

func NewArtifactsMap() *ArtifactsMap {
	return &ArtifactsMap{
		Artifacts: make(map[string][]*PackageArtifact, 0),
	}
}

func NewArtifactsPack() *ArtifactsPack {
	return &ArtifactsPack{
		Artifacts: []*PackageArtifact{},
	}
}

func (ap *ArtifactsPack) ToMap() *ArtifactsMap {
	ans := &ArtifactsMap{
		Artifacts: make(map[string][]*PackageArtifact, 1),
	}

	for idx, _ := range ap.Artifacts {
		ans.Add(ap.Artifacts[idx])
	}

	return ans
}

func (ap *ArtifactsPack) AppendPack(p *ArtifactsPack) {
	if p != nil {
		for idx := range p.Artifacts {
			ap.Add(p.Artifacts[idx])
		}
	}
}

func (ap *ArtifactsPack) YAML() ([]byte, error) {
	return yaml.Marshal(ap)
}

func (ap *ArtifactsPack) JSON() ([]byte, error) {
	return json.Marshal(ap)
}

func (ap *ArtifactsPack) Add(p *PackageArtifact) {
	ap.Artifacts = append(ap.Artifacts, p)
}

func (ap *ArtifactsPack) IsPresent(p *PackageArtifact) bool {
	for _, a := range ap.Artifacts {
		if a.GetPackage().PackageName() == p.GetPackage().PackageName() {
			return true
		}
	}
	return false
}

func (am *ArtifactsMap) GetKeys() []string {
	ans := []string{}
	if len(am.Artifacts) > 0 {
		for k := range am.Artifacts {
			ans = append(ans, k)
		}
	}
	return ans
}

func (am *ArtifactsMap) MatchVersion(p *PackageArtifact) (*PackageArtifact, error) {
	var ans *PackageArtifact = nil
	var key string

	if p.Runtime != nil {
		key = p.Runtime.PackageName()
	} else {
		key = p.CompileSpec.Package.PackageName()
	}

	if val, ok := am.Artifacts[key]; ok {
		for idx, _ := range val {
			pp := val[idx]

			if pp.GetVersion() == p.GetVersion() {
				ans = pp
				break
			}
		}
	}

	if ans == nil {
		return ans, errors.New(fmt.Sprintf("Package %s-%s not found", key, p.GetVersion()))
	}

	return ans, nil
}

func (am *ArtifactsMap) Add(p *PackageArtifact) {
	var key string
	if p.Runtime != nil {
		key = p.Runtime.PackageName()
	} else {
		key = p.CompileSpec.Package.PackageName()
	}

	if val, ok := am.Artifacts[key]; ok {
		am.Artifacts[key] = append(val, p)
	} else {
		am.Artifacts[key] = []*PackageArtifact{p}
	}
}

func (am *ArtifactsMap) ToList() *[]*PackageArtifact {
	ans := []*PackageArtifact{}
	for _, v := range am.Artifacts {
		ans = append(ans, v...)
	}
	return &ans
}

func (am *ArtifactsMap) HasKey(k string) bool {
	_, present := am.Artifacts[k]
	return present
}

func (am *ArtifactsMap) GetKeyFromValue(p *PackageArtifact) string {
	ans := ""

	for k, val := range am.Artifacts {
		for _, a := range val {
			if a == p {
				ans = k
				break
			}
		}
	}

	return ans
}

func (am *ArtifactsMap) GetArtifactsByKey(k string) ([]*PackageArtifact, error) {
	val, ok := am.Artifacts[k]
	if !ok {
		return nil, fmt.Errorf("Package %s not found on map", k)
	}
	return val, nil
}

func (am *ArtifactsMap) GetProvides(p string) []*PackageArtifact {
	ans := []*PackageArtifact{}

	for k, arts := range am.Artifacts {
		for _, a := range arts {
			dp := a.GetPackage()
			if dp.GetProvidePackage(p) != nil {
				ans, _ = am.Artifacts[k]
				break
			}
		}
	}

	return ans
}

func (am *ArtifactsMap) GetSortedArtifactsByKey(k string) ([]*PackageArtifact, error) {
	ans := []*PackageArtifact{}

	val, ok := am.Artifacts[k]
	if !ok {
		return nil, fmt.Errorf("Package %s not found on map", k)
	}

	if len(val) == 1 {
		ans = val
	} else {

		// NOTE: In the near future the DefaultPackage will be based
		//       on GentooPackage implementation and I will reduce complexity
		//       here. For now, I convert the list on GentooPackageSorter

		// TODO: At the moment I don't respect the repository priority for packages
		//       with the same version but I have not repositories data here.
		glist := []gentoo.GentooPackage{}

		for _, p := range val {
			gpkg, err := gentoo.ParsePackageStr(p.GetPackage().HumanReadableString())
			if err != nil {
				return nil, err
			}
			glist = append(glist, *gpkg)
		}

		sort.Sort(sort.Reverse(gentoo.GentooPackageSorter(glist)))

		for idx, _ := range glist {
			pstr := glist[idx].String()
			if glist[idx].VersionBuild != "" {
				pstr = fmt.Sprintf("%s+%s", pstr, glist[idx].VersionBuild)
			}
			for _, p := range val {
				if p.GetPackage().HumanReadableString() == pstr {
					ans = append(ans, p)
					break
				}
			}
		}

	}

	return ans, nil
}
