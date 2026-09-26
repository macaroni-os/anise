/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package helpers

import (
	"errors"
	"regexp"

	. "github.com/macaroni-os/anise/pkg/config"

	"github.com/spf13/cobra"
)

func BindValuesFlags(cmd *cobra.Command) {
	AniseCfg.Viper.BindPFlag("values", cmd.Flags().Lookup("values"))
}

func ValuesFlags() []string {
	return AniseCfg.Viper.GetStringSlice("values")
}

func CreateRegexArray(rgx []string) ([]*regexp.Regexp, error) {
	ans := make([]*regexp.Regexp, len(rgx))
	if len(rgx) > 0 {
		for idx, reg := range rgx {
			re := regexp.MustCompile(reg)
			if re == nil {
				return nil, errors.New("Invalid regex " + reg + "!")
			}
			ans[idx] = re
		}
	}

	return ans, nil
}

func BindSolverFlags(cmd *cobra.Command) {
	AniseCfg.Viper.BindPFlag("solver.implementation", cmd.Flags().Lookup("solver-implementation"))
}

func SetSolverConfig() (c *AniseSolverOptions) {
	stype := AniseCfg.Viper.GetString("solver.type")
	implementation := AniseCfg.Viper.GetString("solver.implementation")

	AniseCfg.GetSolverOptions().Type = stype
	AniseCfg.GetSolverOptions().Implementation = implementation

	if implementation == "" {
		// Using solver.type until i will drop solver.implementation option.
		AniseCfg.GetSolverOptions().Implementation = stype
		implementation = stype
	}

	return &AniseSolverOptions{
		Type:           stype,
		Implementation: implementation,
	}
}
