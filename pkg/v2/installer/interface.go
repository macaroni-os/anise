/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package installer

import (
	artifact "github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
)

type Client interface {
	DownloadArtifact(*artifact.PackageArtifact) error
	DownloadFile(string) (string, error)
}
