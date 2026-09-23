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
	AniseCfg.Viper.BindPFlag("solver.type", cmd.Flags().Lookup("solver-type"))
	AniseCfg.Viper.BindPFlag("solver.discount", cmd.Flags().Lookup("solver-discount"))
	AniseCfg.Viper.BindPFlag("solver.rate", cmd.Flags().Lookup("solver-rate"))
	AniseCfg.Viper.BindPFlag("solver.max_attempts", cmd.Flags().Lookup("solver-attempts"))
	AniseCfg.Viper.BindPFlag("solver.implementation", cmd.Flags().Lookup("solver-implementation"))
}

func SetSolverConfig() (c *AniseSolverOptions) {
	stype := AniseCfg.Viper.GetString("solver.type")
	discount := AniseCfg.Viper.GetFloat64("solver.discount")
	rate := AniseCfg.Viper.GetFloat64("solver.rate")
	attempts := AniseCfg.Viper.GetInt("solver.max_attempts")
	implementation := AniseCfg.Viper.GetString("solver.implementation")

	AniseCfg.GetSolverOptions().Type = stype
	AniseCfg.GetSolverOptions().LearnRate = float32(rate)
	AniseCfg.GetSolverOptions().Discount = float32(discount)
	AniseCfg.GetSolverOptions().MaxAttempts = attempts
	AniseCfg.GetSolverOptions().Implementation = implementation

	if implementation == "" {
		// Using solver.type until i will drop solver.implementation option.
		AniseCfg.GetSolverOptions().Implementation = stype
		implementation = stype
	}

	return &AniseSolverOptions{
		Type:           stype,
		LearnRate:      float32(rate),
		Discount:       float32(discount),
		MaxAttempts:    attempts,
		Implementation: implementation,
	}
}
