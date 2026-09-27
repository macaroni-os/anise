/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package docker

import (
	"context"
	"fmt"
	"os"
	"strings"

	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"

	continerdarchive "github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/images"
	tregistry "github.com/docker/docker/api/types/registry"
	tarf "github.com/geaaru/tar-formers/pkg/executor"
	tarf_specs "github.com/geaaru/tar-formers/pkg/specs"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type staticAuth struct {
	auth *tregistry.AuthConfig
}

func (s staticAuth) Authorization() (*authn.AuthConfig, error) {
	if s.auth == nil {
		return nil, nil
	}
	return &authn.AuthConfig{
		Username:      s.auth.Username,
		Password:      s.auth.Password,
		Auth:          s.auth.Auth,
		IdentityToken: s.auth.IdentityToken,
		RegistryToken: s.auth.RegistryToken,
	}, nil
}

// UnarchiveLayers extract layers with archive.Untar from docker instead of containerd
func UnarchiveLayers(temp string, img v1.Image, image, dest string, auth *tregistry.AuthConfig, verify bool) (int64, error) {
	layers, err := img.Layers()
	if err != nil {
		return 0, fmt.Errorf("reading layers from '%s' image failed: %v", image, err)
	}

	var size int64
	for _, l := range layers {
		s, err := l.Size()
		if err != nil {
			return 0, fmt.Errorf("reading layer size from '%s' image failed: %v", image, err)
		}
		size += s

		layerReader, err := l.Uncompressed()
		if err != nil {
			return 0, fmt.Errorf("reading uncompressed layer from '%s' image failed: %v", image, err)
		}
		defer layerReader.Close()

		spec := tarf_specs.NewSpecFile()
		spec.SameOwner = true
		spec.EnableMutex = true
		spec.OverwritePerms = true
		spec.IgnoreRegexes = []string{
			// prevent 'operation not permitted'
			//"^/dev/",
		}
		spec.IgnoreFiles = []string{}

		tarformers := tarf.NewTarFormers(tarf.GetOptimusPrime().Config)
		tarformers.SetReader(layerReader)

		if err := tarformers.RunTask(spec, dest); err != nil {
			return 0, fmt.Errorf("extracting '%s' image to directory %s failed: %v", image, dest, err)
		}
	}

	return size, nil
}

// DownloadAndExtractDockerImage extracts a container image natively. It supports privileged/unprivileged mode
func DownloadAndExtractDockerImage(temp, image, dest string, auth *tregistry.AuthConfig) (*images.Image, error) {

	if !fileHelper.Exists(dest) {
		if err := os.MkdirAll(dest, os.ModePerm); err != nil {
			return nil, errors.Wrapf(err, "cannot create destination directory")
		}
	}

	ref, err := name.ParseReference(image)
	if err != nil {
		return nil, err
	}

	img, err := remote.Image(ref, remote.WithAuth(staticAuth{auth}))
	if err != nil {
		return nil, err
	}

	m, err := img.Manifest()
	if err != nil {
		return nil, err
	}

	mt, err := img.MediaType()
	if err != nil {
		return nil, err
	}

	d, err := img.Digest()
	if err != nil {
		return nil, err
	}

	reader := mutate.Extract(img)
	defer reader.Close()
	defer os.RemoveAll(temp)

	c, err := continerdarchive.Apply(context.TODO(), dest, reader)
	if err != nil {
		return nil, err
	}

	return &images.Image{
		Name:   image,
		Labels: m.Annotations,
		Target: specs.Descriptor{
			MediaType: string(mt),
			Digest:    digest.Digest(d.String()),
			Size:      c,
		},
	}, nil
}

func StripInvalidStringsFromImage(s string) string {
	return strings.ReplaceAll(s, "+", "-")
}
