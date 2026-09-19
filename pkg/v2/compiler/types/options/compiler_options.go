/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package options

import (
	"runtime"

	"github.com/macaroni-os/anise/pkg/v2/compiler/types/compression"
)

type Compiler struct {
	PushImageRepository string                     `json:"push_image_repository,omitempty" yaml:"push_image_repository,omitempty"`
	PullImageRepository []string                   `json:"pull_image_repository,omitempty" yaml:"pull_image_repository,omitempty"`
	PullFirst           bool                       `json:"pull_first,omitempty" yaml:"pull_first,omitempty"`
	KeepImg             bool                       `json:"keepimg,omitempty" yaml:"keepimg,omitempty"`
	Push                bool                       `json:"push,omitempty" yaml:"push,omitempty"`
	Privileged          bool                       `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	Concurrency         int                        `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	CompressionType     compression.Implementation `json:"compression_type,omitempty" yaml:"compression_type,omitempty"`

	PackageTargetOnly bool `json:"package_targetonly,omitempty" yaml:"package_targetonly,omitempty"`
	Rebuild           bool `json:"rebuild,omitempty" yaml:"rebuild,omitempty"`

	BackendArgs []string `json:"backend_args,omitempty" yaml:"backend_args,omitempty"`
	BackendType string   `json:"backend_type,omitempty" yaml:"backend_type,omitempty"`
}

func NewDefaultCompiler() *Compiler {
	return &Compiler{
		PushImageRepository: "anise/cache",
		PullFirst:           false,
		Push:                false,
		CompressionType:     compression.None,
		KeepImg:             true,
		Concurrency:         runtime.NumCPU(),
		BackendType:         "dockerv3",
		BackendArgs:         []string{},
		Rebuild:             false,
	}
}

type Option func(cfg *Compiler) error

func (cfg *Compiler) Apply(opts ...Option) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return err
		}
	}
	return nil
}

func WithOptions(opt *Compiler) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg = opt
		return nil
	}
}

func WithBackendType(r string) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.BackendType = r
		return nil
	}
}

func WithPullRepositories(r []string) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.PullImageRepository = r
		return nil
	}
}

func WithPushRepository(r string) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		if len(cfg.PullImageRepository) == 0 {
			cfg.PullImageRepository = []string{cfg.PushImageRepository}
		}
		cfg.PushImageRepository = r
		return nil
	}
}

func BackendArgs(r []string) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.BackendArgs = r
		return nil
	}
}

func PullFirst(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.PullFirst = b
		return nil
	}
}

func KeepImg(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.KeepImg = b
		return nil
	}
}

func Rebuild(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.Rebuild = b
		return nil
	}
}

func Privileged(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.Privileged = b
		return nil
	}
}

func PushImages(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.Push = b
		return nil
	}
}

func OnlyTarget(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.PackageTargetOnly = b
		return nil
	}
}

func Concurrency(i int) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		if i == 0 {
			i = runtime.NumCPU()
		}
		cfg.Concurrency = i
		return nil
	}
}

func WithCompressionType(t compression.Implementation) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.CompressionType = t
		return nil
	}
}
