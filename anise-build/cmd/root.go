/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	config "github.com/macaroni-os/anise/pkg/config"
	helpers "github.com/macaroni-os/anise/pkg/helpers"
	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"
	. "github.com/macaroni-os/anise/pkg/logger"
	repo "github.com/macaroni-os/anise/pkg/repository"

	tarf "github.com/geaaru/tar-formers/pkg/executor"
	tarf_specs "github.com/geaaru/tar-formers/pkg/specs"
	"github.com/spf13/cobra"
)

var cfgFile string
var Verbose bool

func version() string {
	if config.BuildGoVersion != "" {
		return fmt.Sprintf("%s-%s-g%s %s - %s",
			config.AniseVersion, config.AniseForkVersion,
			config.BuildCommit, config.BuildTime, config.BuildGoVersion)
	} else {
		return fmt.Sprintf("%s-%s-g%s %s", config.AniseVersion,
			config.AniseForkVersion, config.BuildCommit, config.BuildTime)
	}
}

func LoadConfig(c *config.AniseConfig) error {
	// If a config file is found, read it in.
	err := c.Viper.ReadInConfig()
	if err != nil {
		Debug(fmt.Sprintf("Error on reading file %s: %s",
			c.Viper.ConfigFileUsed(), err.Error()))
	}

	err = c.Viper.Unmarshal(&c)
	if err != nil {
		return err
	}

	noSpinner := c.Viper.GetBool("no_spinner")

	InitAurora()
	if !noSpinner {
		NewSpinner()
	}

	Debug("Using config file:", c.Viper.ConfigFileUsed())

	if c.GetLogging().EnableLogFile && c.GetLogging().Path != "" {
		// Init zap logger
		err = ZapLogger()
		if err != nil {
			return err
		}
	}

	// Load repositories
	err = repo.LoadRepositories(c)
	if err != nil {
		return err
	}

	// Initialize default tarformers instance
	// to use the config object used by the library.
	cfg := tarf_specs.NewConfig(c.Viper)
	if c.GetLogging().Paranoid {
		cfg.GetGeneral().Debug = true
		cfg.GetLogging().Level = c.GetLogging().Level
	} else {
		cfg.GetGeneral().Debug = false
		cfg.GetLogging().Level = "warning"
	}

	t := tarf.NewTarFormersWithLog(cfg, true)
	tarf.SetDefaultTarFormers(t)

	return nil
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	var cfg *config.AniseConfig = config.AniseCfg

	initConfig(cfg)

	// RootCmd represents the base command when called without any subcommands
	var RootCmd = &cobra.Command{
		Use:   "anise-build",
		Short: "Container based package manager",
		Long: `Anise build is the build module of the luset package manager based on containers to build packages.
		
	To build a package, from a tree definition:

		$ anise-build build --tree tree/path package
		
	`,
		Version: version(),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {

			cfg.Viper.SetConfigType("yaml")

			if cfgFile != "" { // enable ability to specify config file via flag
				cfg.Viper.SetConfigFile(cfgFile)
			} else {
				// Retrieve pwd directory
				pwdDir, err := os.Getwd()
				if err != nil {
					Error(err)
					os.Exit(1)
				}
				homeDir := helpers.GetHomeDir()

				if fileHelper.Exists(filepath.Join(pwdDir, ".anise.yaml")) || (homeDir != "" && fileHelper.Exists(filepath.Join(homeDir, ".anise.yaml"))) {
					cfg.Viper.AddConfigPath(".")
					if homeDir != "" {
						cfg.Viper.AddConfigPath(homeDir)
					}
					cfg.Viper.SetConfigName(".anise")
				} else {
					cfg.Viper.SetConfigName("anise")
					cfg.Viper.AddConfigPath("/etc/anise")
				}
			}

			err := LoadConfig(cfg)
			if err != nil {
				Fatal("failed to load configuration:", err.Error())
			}
			// Initialize tmpdir prefix. TODO: Move this with LoadConfig
			// directly on sub command to ensure the creation only when it's
			// needed.
			err = cfg.GetSystem().InitTmpDir()
			if err != nil {
				Fatal("failed on init tmp basedir:", err.Error())
			}

		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			avoidCleanupTmpdir, _ := cmd.Parent().PersistentFlags().GetBool("avoid-cleanup-tmpdir")

			// Cleanup all tmp directories used by anise
			if !avoidCleanupTmpdir {
				err := cfg.GetSystem().CleanupTmpDir()
				if err != nil {
					Warning("failed on cleanup tmpdir:", err.Error())
				}
			}

			systemDB := cfg.GetSystemDB()
			err := systemDB.Close()
			if err != nil {
				Warning("failed on close database:", err.Error())
			}

		},
		SilenceErrors: true,
	}

	initCommand(RootCmd, cfg)

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func initCommand(rootCmd *cobra.Command, cfg *config.AniseConfig) {

	pflags := rootCmd.PersistentFlags()
	pflags.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.anise.yaml)")
	pflags.BoolP("debug", "d", false, "verbose output")
	pflags.Bool("fatal", false, "Enables Warnings to exit")
	pflags.Bool("enable-logfile", false, "Enable log to file")
	pflags.Bool("avoid-cleanup-tmpdir", false, "For debugging could help to see the content of temporary dirs.")
	pflags.Bool("no-spinner", false, "Disable spinner.")
	pflags.Bool("color", cfg.Viper.GetBool("logging.color"), "Enable/Disable color.")
	pflags.Bool("emoji", cfg.Viper.GetBool("logging.enable_emoji"), "Enable/Disable emoji.")
	pflags.Bool("skip-config-protect", cfg.Viper.GetBool("config_protect_skip"),
		"Disable config protect analysis.")
	pflags.StringP("logfile", "l", cfg.Viper.GetString("logging.path"),
		"Logfile path. Empty value disable log to file.")

	pflags.String("system-dbpath", "", "System db path")
	pflags.String("system-target", "", "System rootpath")
	pflags.String("system-engine", "", "System DB engine")

	// os/user doesn't work in from scratch environments.
	// Check if i can retrieve user informations.
	_, err := user.Current()
	if err != nil {
		Warning("failed to retrieve user identity:", err.Error())
	}
	pflags.Bool("same-owner", cfg.Viper.GetBool("general.same_owner"),
		"Maintain same owner on uncompress.")
	pflags.Int("concurrency", runtime.NumCPU(), "Concurrency")

	cfg.Viper.BindPFlag("logging.color", pflags.Lookup("color"))
	cfg.Viper.BindPFlag("logging.enable_emoji", pflags.Lookup("emoji"))
	cfg.Viper.BindPFlag("logging.enable_logfile", pflags.Lookup("enable-logfile"))
	cfg.Viper.BindPFlag("logging.path", pflags.Lookup("logfile"))

	cfg.Viper.BindPFlag("general.concurrency", pflags.Lookup("concurrency"))
	cfg.Viper.BindPFlag("general.debug", pflags.Lookup("debug"))
	cfg.Viper.BindPFlag("general.fatal_warnings", pflags.Lookup("fatal"))
	cfg.Viper.BindPFlag("general.same_owner", pflags.Lookup("same-owner"))
	cfg.Viper.BindPFlag("plugin", pflags.Lookup("plugin"))

	// Currently I maintain this only from cli.
	cfg.Viper.BindPFlag("no_spinner", pflags.Lookup("no-spinner"))
	cfg.Viper.BindPFlag("config_protect_skip", pflags.Lookup("skip-config-protect"))

	cfg.Viper.BindPFlag("system.database_path", pflags.Lookup("system-dbpath"))
	cfg.Viper.BindPFlag("system.rootfs", pflags.Lookup("system-target"))
	cfg.Viper.BindPFlag("system.database_engine", pflags.Lookup("system-engine"))

	// Add main commands
	rootCmd.AddCommand(
		newCreateRepoCommand(cfg),
		newConfigCommand(cfg),
		newPackCommand(cfg),
		newServerRepoCommand(cfg),
		newTreeCommand(cfg),
		newBuildCommand(cfg),
	)
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cfg *config.AniseConfig) {
	// Anise support these priorities on read configuration file:
	// - command line option (if available)
	// - $PWD/.anise.yaml
	// - $HOME/.anise.yaml
	// - /etc/anise/anise.yaml
	//
	// Note: currently a single viper instance support only one config name.

	cfg.Viper.SetEnvPrefix(config.AniseEnvPrefix)

	cfg.Viper.BindEnv("config")
	cfg.Viper.SetDefault("config", "")
	cfg.Viper.SetDefault("etcd-config", false)

	cfg.Viper.AutomaticEnv() // read in environment variables that match

	// Create EnvKey Replacer for handle complex structure
	replacer := strings.NewReplacer(".", "__")
	cfg.Viper.SetEnvKeyReplacer(replacer)

	cfg.Viper.SetTypeByDefaultValue(true)
}
