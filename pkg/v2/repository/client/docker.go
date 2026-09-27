/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/docker/docker/api/types/registry"
	"github.com/docker/go-units"
	"github.com/pkg/errors"

	"github.com/macaroni-os/anise/pkg/config"
	"github.com/macaroni-os/anise/pkg/helpers/docker"
	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"
	. "github.com/macaroni-os/anise/pkg/logger"
	"github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"
)

const (
	errImageDownloadMsg = "failed downloading image %s: %s"
)

type DockerClient struct {
	Repository *config.AniseRepository
	auth       *registry.AuthConfig
}

func NewDockerClient(r *config.AniseRepository) *DockerClient {
	auth := &registry.AuthConfig{}

	dat, _ := json.Marshal(r.Authentication)
	json.Unmarshal(dat, auth)

	return &DockerClient{Repository: r, auth: auth}
}

func (c *DockerClient) DownloadArtifact(a *artifact.PackageArtifact, msg string) error {
	//var u *url.URL = nil
	var err error
	var temp string

	Spinner(22)
	defer SpinnerStop()

	var resultingArtifact *artifact.PackageArtifact
	artifactName := path.Base(a.Path)
	cacheFile := filepath.Join(config.AniseCfg.GetSystem().GetSystemPkgsCacheDirPath(), artifactName)
	Debug("Cache file", cacheFile)
	if err := fileHelper.EnsureDir(cacheFile); err != nil {
		return errors.Wrapf(err, "could not create cache folder %s for %s", config.AniseCfg.GetSystem().GetSystemPkgsCacheDirPath(), cacheFile)
	}
	ok := false

	// TODO:
	// Files are in URI/packagename:version (GetPackageImageName() method)
	// use downloadAndExtract .. and egenrate an archive to consume. Checksum should be already checked while downloading the image
	// with the above functions, because Docker images already contain such metadata
	// - Check how verification is done when calling DownloadArtifact outside, similarly we need to check DownloadFile, and how verification
	// is done in such cases (see repository.go)

	// Check if file is already in cache
	if fileHelper.Exists(cacheFile) {
		Debug("Cache hit for artifact", artifactName)
		resultingArtifact = a
		resultingArtifact.Path = cacheFile
		resultingArtifact.Checksums = artifact.Checksums{}
	} else {

		temp, err = config.AniseCfg.GetSystem().TempDir("tree")
		if err != nil {
			return err
		}
		defer os.RemoveAll(temp)

		for _, uri := range c.Repository.Urls {

			imageName := fmt.Sprintf("%s:%s", uri, a.CompileSpec.GetPackage().ImageID())
			Info("Downloading image", imageName)

			contentstore, err := config.AniseCfg.GetSystem().TempDir("contentstore")
			if err != nil {
				Warning("Cannot create contentstore", err.Error())
				continue
			}

			// imageName := fmt.Sprintf("%s/%s", uri, artifact.GetCompileSpec().GetPackage().GetPackageImageName())
			info, err := docker.DownloadAndExtractDockerImage(contentstore, imageName, temp, c.auth)
			if err != nil {
				Warning(fmt.Sprintf(errImageDownloadMsg, imageName, err.Error()))
				continue
			}

			Info(fmt.Sprintf("Pulled: %s", info.Target.Digest))
			Info(fmt.Sprintf("Size: %s", units.BytesSize(float64(info.Target.Size))))
			Debug("\nCompressing result ", filepath.Join(temp), "to", cacheFile)

			a.CachePath = cacheFile
			// We discard checksum, that are checked while during pull and unpack
			newart := a
			a.Checksums = artifact.Checksums{}
			newart.Checksums = artifact.Checksums{}
			newart.Path = cacheFile                    // First set to cache file
			newart.Path = newart.GetUncompressedName() // Calculate the real path from cacheFile
			err = newart.Compress(temp, 1)
			if err != nil {
				Error(fmt.Sprintf("Failed compressing package %s: %s", imageName, err.Error()))
				continue
			}
			//resultingArtifact = newart

			ok = true
			break
		}

		if !ok {
			return err
		}
	}

	return nil
}

func (c *DockerClient) DownloadFile(name string) (string, error) {
	var file *os.File = nil
	var err error
	var temp, contentstore string
	// Files should be in URI/repository:<file>
	ok := false

	temp, err = config.AniseCfg.GetSystem().TempDir("tree")
	if err != nil {
		return "", err
	}

	for _, uri := range c.Repository.Urls {
		file, err = config.AniseCfg.GetSystem().TempFile("DockerClient")
		if err != nil {
			continue
		}

		contentstore, err = config.AniseCfg.GetSystem().TempDir("contentstore")
		if err != nil {
			Warning("Cannot create contentstore", err.Error())
			continue
		}

		imageName := fmt.Sprintf("%s:%s", uri, docker.StripInvalidStringsFromImage(name))
		Info("Downloading", imageName)

		info, err := docker.DownloadAndExtractDockerImage(contentstore, imageName, temp, c.auth)
		if err != nil {
			Warning(fmt.Sprintf(errImageDownloadMsg, imageName, err.Error()))
			continue
		}

		Info(fmt.Sprintf("Pulled: %s", info.Target.Digest))
		Info(fmt.Sprintf("Size: %s", units.BytesSize(float64(info.Target.Size))))

		Debug("\nCopying file ", filepath.Join(temp, name), "to", file.Name())
		err = fileHelper.CopyFile(filepath.Join(temp, name), file.Name())
		if err != nil {
			continue
		}
		ok = true
		break
	}

	if !ok {
		return "", err
	}

	return file.Name(), err
}
