/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package repository

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/macaroni-os/anise/pkg/config"
	fhelpers "github.com/macaroni-os/anise/pkg/helpers/file"
	artifact "github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"

	tarf_tools "github.com/geaaru/tar-formers/pkg/tools"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type WagonDocument struct {
	FileName        string                     `json:"filename" yaml:"filename"`
	CompressionType tarf_tools.CompressionMode `json:"compressiontype,omitempty" yaml:"compressiontype,omitempty"`
	Checksums       artifact.Checksums         `json:"checksums,omitempty" yaml:"checksums,omitempty"`
}

type WagonIdentity struct {
	*config.AniseRepository `yaml:",inline" json:",inline"`

	IdentityFile    string                    `yaml:"-" json:"-"`
	RepositoryFiles map[string]*WagonDocument `yaml:"repo_files,omitempty" json:"repo_files,omitempty"`
}

func NewWagonIdentify(l *config.AniseRepository) *WagonIdentity {
	return &WagonIdentity{
		AniseRepository: l,
		IdentityFile:    "",
		RepositoryFiles: make(map[string]*WagonDocument, 0),
	}
}

func (w *WagonIdentity) PurgeFiles() {
	w.RepositoryFiles = make(map[string]*WagonDocument, 0)
}

func (w *WagonIdentity) Valid() bool {
	_, hasTreev2 := w.RepositoryFiles[REPOFILE_TREEV2_KEY]
	if !hasTreev2 {
		_, hasMeta := w.RepositoryFiles[REPOFILE_META_KEY]
		if !hasMeta {
			return false
		}

		_, hasTree := w.RepositoryFiles[REPOFILE_TREE_KEY]
		if !hasTree {
			return false
		}
	}

	return true
}

func (w *WagonIdentity) HasDocument(s string) bool {
	_, hasDoc := w.RepositoryFiles[s]
	return hasDoc
}

func (w *WagonIdentity) GetTreePath() string   { return w.AniseRepository.TreePath }
func (w *WagonIdentity) GetMetaPath() string   { return w.AniseRepository.MetaPath }
func (w *WagonIdentity) GetName() string       { return w.AniseRepository.Name }
func (w *WagonIdentity) GetLastUpdate() string { return w.AniseRepository.LastUpdate }
func (w *WagonIdentity) GetRevision() int      { return w.AniseRepository.Revision }
func (w *WagonIdentity) GetType() string       { return w.AniseRepository.Type }
func (w *WagonIdentity) GetVerify() bool       { return w.AniseRepository.Verify }
func (w *WagonIdentity) GetUrls() []string     { return w.AniseRepository.Urls }

func (w *WagonIdentity) SetLastUpdate(u string) { w.AniseRepository.LastUpdate = u }
func (w *WagonIdentity) SetType(p string)       { w.AniseRepository.Type = p }
func (w *WagonIdentity) SetVerify(p bool)       { w.AniseRepository.Verify = p }

func (w *WagonIdentity) GetAuthentication() map[string]string {
	return w.AniseRepository.Authentication
}

func (w *WagonIdentity) IncrementRevision() {
	w.AniseRepository.Revision++
}

func (w *WagonIdentity) BumpRevision() {
	w.IncrementRevision()
	w.AniseRepository.LastUpdate = strconv.FormatInt(time.Now().Unix(), 10)
}

func (w *WagonIdentity) Yaml() ([]byte, error) {
	return yaml.Marshal(w)
}

func (w *WagonIdentity) Write(f string) error {
	data, err := w.Yaml()
	if err != nil {
		return err
	}

	err = os.WriteFile(f, data, os.ModePerm)
	if err != nil {
		return fmt.Errorf("Error on write identify file %s: %s",
			f, err.Error())
	}

	return nil
}

func (w *WagonIdentity) Load(f string) error {
	//previousName := w.AniseRepository.Name

	if !fhelpers.Exists(f) {
		return fmt.Errorf("The repository %s must be synced.",
			w.AniseRepository.Name)
	}

	data, err := os.ReadFile(f)
	if err != nil {
		return errors.Wrap(err, "Error on reading file "+f)
	}

	err = yaml.Unmarshal(data, w)
	if err != nil {
		return err
	}

	w.IdentityFile = f

	return nil
}

func (w *WagonIdentity) Is2Update(newIdentity *WagonIdentity) bool {
	ans := true

	if w.GetRevision() == newIdentity.GetRevision() &&
		w.GetLastUpdate() == newIdentity.GetLastUpdate() {
		ans = false
	}

	return ans
}

func (w *WagonIdentity) DownloadDocument(c Client, key string) (*artifact.PackageArtifact, error) {
	docFile, ok := w.RepositoryFiles[key]
	if !ok {
		return nil, errors.New(fmt.Sprintf("key %s not present in the repository", key))
	}

	downloadedFile, err := c.DownloadFile(docFile.GetFileName())
	if err != nil {
		return nil, errors.Wrap(err, "While downloading "+docFile.GetFileName())
	}

	docArtifact := artifact.NewPackageArtifact(downloadedFile)
	docArtifact.CachePath = downloadedFile
	docArtifact.Checksums = docFile.GetChecksums()
	docArtifact.CompressionType = docFile.GetCompressionType()

	err = docArtifact.Verify()
	if err != nil {
		return nil, errors.Wrap(err, "file integrity check failure for repository "+w.GetName())
	}

	return docArtifact, nil
}
