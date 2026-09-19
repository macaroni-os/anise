/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package artifact

import (
	"encoding/json"

	tarf_specs "github.com/geaaru/tar-formers/pkg/specs"
	yaml "gopkg.in/yaml.v3"
)

const (
	ExtraMetadataSuffix = ".extra-metadata.json"
)

type ExtraMetadata struct {
	Files []*tarf_specs.FileIdentity `json:"files,omitempty" yaml:"files,omitempty"`
}

func NewExtraMetadata() *ExtraMetadata {
	return &ExtraMetadata{}
}

func (m *ExtraMetadata) JSON() ([]byte, error) {
	return json.Marshal(m)
}

func (m *ExtraMetadata) YAML() ([]byte, error) {
	return yaml.Marshal(m)
}
