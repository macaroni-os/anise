/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"time"

	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"
	pkg "github.com/macaroni-os/anise/pkg/package"

	"github.com/pkg/errors"
	v "github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	AniseVersion     = "0.42.0"
	AniseEnvPrefix   = "ANISE"
	AniseForkVersion = ""
)

var (
	BuildTime      string
	BuildCommit    string
	BuildGoVersion string
)

var AniseCfg *AniseConfig = NewAniseConfig(nil)

type AniseLoggingConfig struct {
	// Path of the logfile
	Path string `yaml:"path,omitempty" json:"path,omitempty" mapstructure:"path"`
	// Enable/Disable logging to file
	EnableLogFile bool `yaml:"enable_logfile,omitempty" json:"enable_logfile,omitempty" mapstructure:"enable_logfile"`
	// Enable JSON format logging in file
	JsonFormat bool `yaml:"json_format,omitempty" json:"json_format,omitempty" mapstructure:"json_format"`

	// Log level
	Level string `yaml:"level,omitempty" json:"level,omitempty" mapstructure:"level"`
	// Enable extra debug logging
	Paranoid bool `yaml:"paranoid,omitempty" json:"paranoid,omitempty" mapstructure:"paranoid"`

	// Enable emoji
	EnableEmoji bool `yaml:"enable_emoji,omitempty" json:"enable_emoji,omitempty" mapstructure:"enable_emoji"`
	// Enable/Disable color in logging
	Color bool `yaml:"color,omitempty" json:"color,omitempty" mapstructure:"color"`
}

type AniseGeneralConfig struct {
	SameOwner       bool `yaml:"same_owner,omitempty" json:"same_owner,omitempty" mapstructure:"same_owner"`
	Concurrency     int  `yaml:"concurrency,omitempty" json:"concurrency,omitempty" mapstructure:"concurrency"`
	Debug           bool `yaml:"debug,omitempty" json:"debug,omitempty" mapstructure:"debug"`
	ShowBuildOutput bool `yaml:"show_build_output,omitempty" json:"show_build_output,omitempty" mapstructure:"show_build_output"`
	SpinnerMs       int  `yaml:"spinner_ms,omitempty" json:"spinner_ms,omitempty" mapstructure:"spinner_ms"`
	SpinnerCharset  int  `yaml:"spinner_charset,omitempty" json:"spinner_charset,omitempty" mapstructure:"spinner_charset"`
	FatalWarns      bool `yaml:"fatal_warnings,omitempty" json:"fatal_warnings,omitempty" mapstructure:"fatal_warnings"`

	ClientTimeout    int  `yaml:"client_timeout,omitempty" json:"client_timeout,omitempty" mapstructure:"client_timeout,omitempty"`
	ClientMultiFetch int  `yaml:"client_multifetch,omitempty" json:"client_multifetch,omitempty" mapstructure:"client_multifetch,omitempty"`
	ClientEncodeURL  bool `yaml:"client_encodeurl,omitempty" json:"client_encodeurl,omitempty" mapstructure:"client_encodeurl,omitempty"`

	OverwriteDirPerms bool `yaml:"overwrite_dir_perms,omitempty" json:"overwrite_dir_perms,omitempty" mapstructure:"overwrite_dir_perms,omitempty"`
}

type AniseBoxConfig struct {
	// Box Backend (possible values: pivot, fchroot)
	Backend     string       `yaml:"backend,omitempty" json:"backend,omitempty" mapstructure:"backend,omitempty"`
	FchrootOpts *FchrootOpts `yaml:"fchroot_opts,omitempty" json:"fchroot_opts,omitempty" mapstructure:"fchroot_opts,omitempty"`
}

type AniseSolverOptions struct {
	Type           string `yaml:"type,omitempty" json:"type,omitempty" mapstructure:"type"`
	Implementation string `yaml:"implementation,omitempty" json:"implementation,omitempty" mapstructure:"implementation"`
}

type AniseSystemConfig struct {
	DatabaseEngine string `yaml:"database_engine" json:"database_engine,omitempty" mapstructure:"database_engine"`
	DatabasePath   string `yaml:"database_path" json:"database_path" mapstructure:"database_path"`
	Rootfs         string `yaml:"rootfs" json:"rootfs" mapstructure:"rootfs"`
	PkgsCachePath  string `yaml:"pkgs_cache_path" json:"pkgs_cache_path" mapstructure:"pkgs_cache_path"`
	TmpDirBase     string `yaml:"tmpdir_base" json:"tmpdir_base" mapstructure:"tmpdir_base"`
}

func (s *AniseSystemConfig) SetRootFS(path string) error {
	p, err := fileHelper.Rel2Abs(path)
	if err != nil {
		return err
	}

	s.Rootfs = p
	return nil
}

func (sc *AniseSystemConfig) GetRepoDatabaseDirPath(name string) string {
	dbpath := filepath.Join(sc.Rootfs, sc.DatabasePath)
	dbpath = filepath.Join(dbpath, "repos/"+name)
	if _, err := os.Stat(dbpath); err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(dbpath, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}
	}
	return dbpath
}

func (c *AniseConfig) GetLockFilePath(lockfile string) string {
	// NOTE: Also when config_from_host is true I prefer
	//       using the rootfs directory for locks.
	return filepath.Join(c.System.Rootfs, "/var/lock/", lockfile)
}

func (sc *AniseSystemConfig) GetSystemRepoDatabaseDirPath() string {
	dbpath := filepath.Join(sc.Rootfs, sc.DatabasePath)
	if _, err := os.Stat(dbpath); err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(dbpath, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}
	}
	return dbpath
}

func (sc *AniseSystemConfig) GetSystemReposDirPath() string {
	ans := filepath.Join(sc.Rootfs, sc.DatabasePath, "repos")
	return ans
}

func (sc *AniseSystemConfig) GetSystemPkgsCacheDirPath() (ans string) {
	var cachepath string
	if sc.PkgsCachePath != "" {
		cachepath = sc.PkgsCachePath
	} else {
		// Create dynamic cache for test suites
		cachepath, _ = ioutil.TempDir(os.TempDir(), "cachepkgs")
	}

	if filepath.IsAbs(cachepath) {
		ans = cachepath
	} else {
		ans = filepath.Join(sc.GetSystemRepoDatabaseDirPath(), cachepath)
	}

	return
}

func (sc *AniseSystemConfig) GetRootFsAbs() (string, error) {
	return filepath.Abs(sc.Rootfs)
}

type AniseRepository struct {
	Name           string            `json:"name" yaml:"name" mapstructure:"name"`
	Description    string            `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description"`
	Urls           []string          `json:"urls" yaml:"urls" mapstructure:"urls"`
	Type           string            `json:"type" yaml:"type" mapstructure:"type"`
	Mode           string            `json:"mode,omitempty" yaml:"mode,omitempty" mapstructure:"mode,omitempty"`
	Priority       int               `json:"priority,omitempty" yaml:"priority,omitempty" mapstructure:"priority"`
	Enable         bool              `json:"enable" yaml:"enable" mapstructure:"enable"`
	Cached         bool              `json:"cached,omitempty" yaml:"cached,omitempty" mapstructure:"cached,omitempty"`
	Authentication map[string]string `json:"auth,omitempty" yaml:"auth,omitempty" mapstructure:"auth,omitempty"`
	TreePath       string            `json:"treepath,omitempty" yaml:"treepath,omitempty" mapstructure:"treepath"`
	MetaPath       string            `json:"metapath,omitempty" yaml:"metapath,omitempty" mapstructure:"metapath"`
	Verify         bool              `json:"verify,omitempty" yaml:"verify,omitempty" mapstructure:"verify"`

	// Serialized options not used in repository configuration

	// Incremented value that identify revision of the repository in a user-friendly way.
	Revision int `json:"revision,omitempty" yaml:"revision,omitempty" mapstructure:"-"`
	// Epoch time in seconds
	LastUpdate string `json:"last_update,omitempty" yaml:"last_update,omitempty" mapstructure:"-"`

	File string `json:"-" yaml:"-" mapstructure:"-"`
}

func NewAniseRepository(name, t, descr string, urls []string, priority int, enable, cached bool) *AniseRepository {
	return &AniseRepository{
		Name:        name,
		Description: descr,
		Urls:        urls,
		Type:        t,
		// Used in cached repositories
		Mode:           "",
		Priority:       priority,
		Enable:         enable,
		Cached:         cached,
		Authentication: make(map[string]string, 0),
		TreePath:       "",
		MetaPath:       "",
	}
}

func NewEmptyAniseRepository() *AniseRepository {
	return &AniseRepository{
		Name:           "",
		Description:    "",
		Urls:           []string{},
		Type:           "",
		Priority:       9999,
		TreePath:       "",
		MetaPath:       "",
		Enable:         false,
		Cached:         true,
		Authentication: make(map[string]string, 0),
	}
}

func (r *AniseRepository) Clone() *AniseRepository {
	ans := NewAniseRepository(r.Name, r.Type, r.Description, r.Urls, r.Priority, r.Enable, r.Cached)
	ans.Verify = r.Verify
	ans.Revision = r.Revision
	ans.LastUpdate = r.LastUpdate
	ans.Authentication = r.Authentication

	return ans
}

func (r *AniseRepository) String() string {
	return fmt.Sprintf("[%s] prio: %d, type: %s, enable: %t, cached: %t",
		r.Name, r.Priority, r.Type, r.Enable, r.Cached)
}

func (r *AniseRepository) YAML() ([]byte, error) {
	return yaml.Marshal(r)
}

type AniseKV struct {
	Key   string `json:"key" yaml:"key" mapstructure:"key"`
	Value string `json:"value" yaml:"value" mapstructure:"value"`
}

type AniseConfig struct {
	Viper *v.Viper `yaml:"-"`

	Logging  AniseLoggingConfig  `yaml:"logging,omitempty" mapstructure:"logging"`
	General  AniseGeneralConfig  `yaml:"general,omitempty" mapstructure:"general"`
	System   AniseSystemConfig   `yaml:"system" mapstructure:"system"`
	Solver   AniseSolverOptions  `yaml:"solver,omitempty" mapstructure:"solver"`
	TarFlows AniseTarflowsConfig `yaml:"tar_flows,omitempty" mapstructure:"tar_flows,omitempty"`
	Box      AniseBoxConfig      `yaml:"box,omitempty" mapstructure:"box,omitempty"`

	RepositoriesConfDir  []string          `yaml:"repos_confdir,omitempty" mapstructure:"repos_confdir"`
	ConfigProtectConfDir []string          `yaml:"config_protect_confdir,omitempty" mapstructure:"config_protect_confdir"`
	PackagesMaskDir      []string          `yaml:"packages_maskdir,omitempty" mapstructure:"packages_maskdir,omitempty"`
	ConfigProtectSkip    bool              `yaml:"config_protect_skip,omitempty" mapstructure:"config_protect_skip"`
	ConfigFromHost       bool              `yaml:"config_from_host,omitempty" mapstructure:"config_from_host"`
	CacheRepositories    []AniseRepository `yaml:"repetitors,omitempty" mapstructure:"repetitors"`
	SystemRepositories   []AniseRepository `yaml:"repositories,omitempty" mapstructure:"repositories"`

	FinalizerEnvs []AniseKV `json:"finalizer_envs,omitempty" yaml:"finalizer_envs,omitempty" mapstructure:"finalizer_envs,omitempty"`

	ConfigProtectConfFiles []ConfigProtectConfFile `yaml:"-" mapstructure:"-"`

	// Subsets config directories for users override.
	SubsetsConfDir []string           `yaml:"subsets_confdir,omitempty" mapstructure:"subsets_confdir"`
	SubsetsDefDir  []string           `yaml:"subsets_defdir,omitempty" mapstructure:"subsets_defdir"`
	Subsets        AniseSubsetsConfig `yaml:"subsets,omitempty" mapstructure:"subsets"`

	SubsetsDefinitions *AniseSubsetsDefinition            `yaml:"-" mapstructure:"-"`
	SubsetsPkgsDefMap  map[string]*AniseSubsetsDefinition `yaml:"-" mapstructure:"-"`
	SubsetsCatDefMap   map[string]*AniseSubsetsDefinition `yaml:"-" mapstructure:"-"`
}

type AniseTarflowsConfig struct {
	CopyBufferSize int   `yaml:"copy_buffer_size,omitempty" mapstructure:"copy_buffer_size,omitempty"`
	MaxOpenFiles   int64 `yaml:"max_openfiles,omitempty" mapstructure:"max_openfiles,omitempty"`
	Mutex4Dirs     bool  `yaml:"mutex4dir,omitempty" mapstructure:"mutex4dir,omitempty"`
	Validate       bool  `yaml:"validate,omitempty" mapstructure:"validate,omitempty"`
}

type AniseSubsetsConfig struct {
	Enabled []string `yaml:"enabled,omitempty" mapstructure:"enabled"`
}

type AniseSubsetsDefinition struct {
	Definitions map[string]*AniseSubsetDefinition `yaml:"subsets_def,omitempty" mapstructure:"subsets_def,omitempty" json:"subsets_def,omitempty"`
}

type AniseSubsetDefinition struct {
	Description string   `yaml:"descr,omitempty" mapstructure:"descr"`
	Name        string   `yaml:"name,omitempty" mapstructure:"name"`
	Rules       []string `yaml:"rules,omitempty" mapstructure:"rules"`

	Packages   []string `yaml:"packages,omitempty" mapstructure:"packages"`
	Categories []string `yaml:"categories,omitempty" mapstructure:"categories"`
}

func NewAniseConfig(viper *v.Viper) *AniseConfig {
	if viper == nil {
		viper = v.New()
	}

	GenDefault(viper)
	return &AniseConfig{
		Viper:                  viper,
		ConfigProtectConfFiles: nil,
		SubsetsDefinitions:     nil,
		SubsetsCatDefMap:       make(map[string]*AniseSubsetsDefinition, 0),
		SubsetsPkgsDefMap:      make(map[string]*AniseSubsetsDefinition, 0),
	}
}

func GenDefault(viper *v.Viper) {
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.enable_logfile", false)
	viper.SetDefault("logging.path", "/var/log/anise.log")
	viper.SetDefault("logging.json_format", false)
	viper.SetDefault("logging.enable_emoji", true)
	viper.SetDefault("logging.color", true)
	viper.SetDefault("logging.paranoid", false)

	viper.SetDefault("general.concurrency", runtime.NumCPU())
	viper.SetDefault("general.debug", false)
	viper.SetDefault("general.show_build_output", false)
	viper.SetDefault("general.spinner_ms", 100)
	viper.SetDefault("general.spinner_charset", 22)
	viper.SetDefault("general.fatal_warnings", false)
	viper.SetDefault("general.overwrite_dir_perms", false)
	viper.SetDefault("general.client_timeout", 3600)
	viper.SetDefault("general.client_multifetch", 2)
	viper.SetDefault("general.client_encodeurl", true)

	u, err := user.Current()
	// os/user doesn't work in from scratch environments
	if err != nil || (u != nil && u.Uid == "0") {
		viper.SetDefault("general.same_owner", true)
	} else {
		viper.SetDefault("general.same_owner", false)
	}

	viper.SetDefault("system.database_engine", "boltdb")
	viper.SetDefault("system.database_path", "/var/cache/anise")
	viper.SetDefault("system.rootfs", "/")
	viper.SetDefault("system.tmpdir_base", filepath.Join(os.TempDir(), "tmpanise"))
	viper.SetDefault("system.pkgs_cache_path", "packages")

	viper.SetDefault("repos_confdir", []string{"/etc/anise/repos.conf.d"})
	viper.SetDefault("config_protect_confdir", []string{"/etc/anise/config.protect.d"})
	viper.SetDefault("packages_maskdir", []string{"/etc/anise/mask.d"})
	viper.SetDefault("subsets_confdir", []string{"/etc/anise/subsets.conf.d"})
	viper.SetDefault("subsets_defdir", []string{"/etc/anise/subsets.def.d"})
	viper.SetDefault("config_protect_skip", false)
	// TODO: Set default to false when we are ready for migration.
	viper.SetDefault("config_from_host", true)
	viper.SetDefault("cache_repositories", []string{})
	viper.SetDefault("system_repositories", []string{})
	viper.SetDefault("finalizer_envs", make(map[string]string, 0))

	viper.SetDefault("solver.type", "")
	viper.SetDefault("solver.rate", 0.7)
	viper.SetDefault("solver.discount", 1.0)
	viper.SetDefault("solver.max_attempts", 9000)

	viper.SetDefault("tar_flows.mutex4dir", true)
	viper.SetDefault("tar_flows.max_openfiles", 100)
	viper.SetDefault("tar_flows.copy_buffer_size", 32)
	viper.SetDefault("tar_flows.validate", false)
}

func (c *AniseConfig) GetSystemDB() pkg.PackageDatabase {
	switch AniseCfg.GetSystem().DatabaseEngine {
	case "boltdb":
		return pkg.NewBoltDatabase(
			filepath.Join(AniseCfg.GetSystem().GetSystemRepoDatabaseDirPath(), "anise.db"))
	default:
		return pkg.NewInMemoryDatabase(true)
	}
}

func (c *AniseConfig) AddSystemRepository(r *AniseRepository) {
	c.SystemRepositories = append(c.SystemRepositories, *r)
}

func (c *AniseConfig) GetFinalizerEnvsMap() map[string]string {
	ans := make(map[string]string, 0)

	for _, kv := range c.FinalizerEnvs {
		ans[kv.Key] = kv.Value
	}
	return ans
}

func (c *AniseConfig) SetFinalizerEnv(k, v string) {
	keyPresent := false
	envs := []AniseKV{}

	for _, kv := range c.FinalizerEnvs {
		if kv.Key == k {
			keyPresent = true
			envs = append(envs, AniseKV{Key: kv.Key, Value: v})
		} else {
			envs = append(envs, kv)
		}
	}
	if !keyPresent {
		envs = append(envs, AniseKV{Key: k, Value: v})
	}

	c.FinalizerEnvs = envs
}

func (c *AniseConfig) GetFinalizerEnvs() []string {
	ans := []string{}
	for _, kv := range c.FinalizerEnvs {
		ans = append(ans, fmt.Sprintf("%s=%s", kv.Key, kv.Value))
	}
	return ans
}

func (c *AniseConfig) GetFinalizerEnv(k string) (string, error) {
	keyNotPresent := true
	ans := ""
	for _, kv := range c.FinalizerEnvs {
		if kv.Key == k {
			keyNotPresent = false
			ans = kv.Value
		}
	}

	if keyNotPresent {
		return "", errors.New("Finalizer key " + k + " not found")
	}
	return ans, nil
}

func (c *AniseConfig) GetLogging() *AniseLoggingConfig {
	return &c.Logging
}

func (c *AniseConfig) GetGeneral() *AniseGeneralConfig {
	return &c.General
}

func (c *AniseConfig) GetSystem() *AniseSystemConfig {
	return &c.System
}

func (c *AniseConfig) GetTarFlows() *AniseTarflowsConfig {
	return &c.TarFlows
}

func (c *AniseConfig) GetSolverOptions() *AniseSolverOptions {
	return &c.Solver
}

func (c *AniseConfig) GetBox() *AniseBoxConfig {
	return &c.Box
}

func (c *AniseConfig) YAML() ([]byte, error) {
	return yaml.Marshal(c)
}

func (c *AniseConfig) GetConfigProtectConfFiles() []ConfigProtectConfFile {
	return c.ConfigProtectConfFiles
}

func (c *AniseConfig) AddConfigProtectConfFile(file *ConfigProtectConfFile) {
	if c.ConfigProtectConfFiles == nil {
		c.ConfigProtectConfFiles = []ConfigProtectConfFile{*file}
	} else {
		c.ConfigProtectConfFiles = append(c.ConfigProtectConfFiles, *file)
	}
}

func (c *AniseConfig) GetSystemRepository(name string) (*AniseRepository, error) {
	var ans *AniseRepository = nil

	for idx, repo := range c.SystemRepositories {
		if repo.Name == name {
			ans = &c.SystemRepositories[idx]
			break
		}
	}
	if ans == nil {
		return nil, errors.New("Repository " + name + " not found")
	}

	return ans, nil
}

func (c *AniseGeneralConfig) GetSpinnerMs() time.Duration {
	duration, err := time.ParseDuration(fmt.Sprintf("%dms", c.SpinnerMs))
	if err != nil {
		return 100 * time.Millisecond
	}
	return duration
}

func (c *AniseLoggingConfig) SetLogLevel(s string) {
	c.Level = s
}

func (c *AniseSystemConfig) InitTmpDir() error {
	if !filepath.IsAbs(c.TmpDirBase) {
		abs, err := fileHelper.Rel2Abs(c.TmpDirBase)
		if err != nil {
			return errors.Wrap(err, "while converting relative path to absolute path")
		}
		c.TmpDirBase = abs
	}

	if _, err := os.Stat(c.TmpDirBase); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(c.TmpDirBase, os.ModePerm)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *AniseSystemConfig) CleanupTmpDir() error {
	return os.RemoveAll(c.TmpDirBase)
}

func (c *AniseSystemConfig) TempDir(pattern string) (string, error) {
	err := c.InitTmpDir()
	if err != nil {
		return "", err
	}
	return ioutil.TempDir(c.TmpDirBase, pattern)
}

func (c *AniseSystemConfig) TempFile(pattern string) (*os.File, error) {
	err := c.InitTmpDir()
	if err != nil {
		return nil, err
	}
	return ioutil.TempFile(c.TmpDirBase, pattern)
}
