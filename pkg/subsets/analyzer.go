/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package subsets

import (
	"regexp"

	. "github.com/macaroni-os/anise/pkg/config"
	"github.com/macaroni-os/anise/pkg/helpers"
)

type SubsetsAnalyzer struct {
	ignoreRegexes []*regexp.Regexp
}

func NewSubsetsAnalyzer(enableSubsets bool,
	def *AniseSubsetsDefinition) (*SubsetsAnalyzer, error) {

	ans := &SubsetsAnalyzer{
		ignoreRegexes: []*regexp.Regexp{},
	}

	if enableSubsets {
		for k, v := range def.Definitions {
			if !helpers.ContainsElem(&AniseCfg.Subsets.Enabled, k) {
				// POST: the selected subset is not enabled.
				//       I add the rules as IgnoreRegexes.

				for _, r := range v.Rules {
					re, err := regexp.Compile(r)
					if err != nil {
						return nil, err
					}
					ans.ignoreRegexes = append(ans.ignoreRegexes, re)
				}

			}
		}
	}

	return ans, nil
}

func (s *SubsetsAnalyzer) AddIgnoreRegexes(rules []string) error {
	for _, r := range rules {
		re, err := regexp.Compile(r)
		if err != nil {
			return err
		}
		s.ignoreRegexes = append(s.ignoreRegexes, re)
	}
	return nil
}

func (s *SubsetsAnalyzer) IsSelected(file string) bool {
	ans := true
	if len(s.ignoreRegexes) > 0 {
		for _, r := range s.ignoreRegexes {
			if r.MatchString(file) {
				ans = false
				break
			}
		}
	}
	return ans
}
