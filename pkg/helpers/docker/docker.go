/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package docker

import (
	"context"
	"os"
	"strings"

	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"

	continerdarchive "github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/images"
	tregistry "github.com/docker/docker/api/types/registry"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
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
