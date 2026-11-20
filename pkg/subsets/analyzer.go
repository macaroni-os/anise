/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package subsets

import (
	"regexp"
	"sort"

	. "github.com/macaroni-os/anise/pkg/config"
	"github.com/macaroni-os/anise/pkg/helpers"
)

type SubsetsAnalyzer struct {
	ignoreRegexes []*regexp.Regexp
	enabledRules  map[string][]*regexp.Regexp
}

func NewSubsetsAnalyzer(enableSubsets bool,
	def *AniseSubsetsDefinition) (*SubsetsAnalyzer, error) {
	return NewSubsetsAnalyzerWithMatches(enableSubsets, false, def)
}

// NewSubsetsAnalyzerWithMatches also tracks rules for enabled subsets.
func NewSubsetsAnalyzerWithMatches(enableSubsets, trackMatches bool,
	def *AniseSubsetsDefinition) (*SubsetsAnalyzer, error) {

	ans := &SubsetsAnalyzer{
		ignoreRegexes: []*regexp.Regexp{},
	}

	if trackMatches {
		ans.enabledRules = make(map[string][]*regexp.Regexp)
	}

	if enableSubsets {
		for k, v := range def.Definitions {
			disabled := !helpers.ContainsElem(&AniseCfg.Subsets.Enabled, k)
			if disabled || trackMatches {
				for _, r := range v.Rules {
					re, err := regexp.Compile(r)
					if err != nil {
						return nil, err
					}
					if disabled {
						ans.ignoreRegexes = append(ans.ignoreRegexes, re)
					} else {
						ans.enabledRules[k] = append(ans.enabledRules[k], re)
					}
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

// MatchingSubsets returns the names of enabled subsets that match the file.
func (s *SubsetsAnalyzer) MatchingSubsets(file string) []string {
	matches := []string{}
	for name, rules := range s.enabledRules {
		for _, rule := range rules {
			if rule.MatchString(file) {
				matches = append(matches, name)
				break
			}
		}
	}
	sort.Strings(matches)
	return matches
}
