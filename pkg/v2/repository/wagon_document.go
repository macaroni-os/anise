/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package repository

import (
	artifact "github.com/macaroni-os/anise/pkg/v2/compiler/types/artifact"

	tarf_tools "github.com/geaaru/tar-formers/pkg/tools"
)

func NewWagonDocument(f string) *WagonDocument {
	return &WagonDocument{
		FileName:        f,
		CompressionType: tarf_tools.None,
		Checksums:       make(map[string]string, 0),
	}
}

// SetFileName sets the name of the repository file.
// Each repository can ship arbitrary file that will be downloaded by the client
// in case of need, this set the filename that the client will pull
func (f *WagonDocument) SetFileName(n string) {
	f.FileName = n
}

// GetFileName returns the name of the repository file.
// Each repository can ship arbitrary file that will be downloaded by the client
// in case of need, this gets the filename that the client will pull
func (f *WagonDocument) GetFileName() string {
	return f.FileName
}

// SetCompressionType sets the compression type of the repository file.
// Each repository can ship arbitrary file that will be downloaded by the client
// in case of need, this sets the compression type that the client will use to uncompress the artifact
func (f *WagonDocument) SetCompressionType(c tarf_tools.CompressionMode) {
	f.CompressionType = c
}

// GetCompressionType gets the compression type of the repository file.
// Each repository can ship arbitrary file that will be downloaded by the client
// in case of need, this gets the compression type that the client will use to uncompress the artifact
func (f *WagonDocument) GetCompressionType() tarf_tools.CompressionMode {
	return f.CompressionType
}

// SetChecksums sets the checksum of the repository file.
// Each repository can ship arbitrary file that will be downloaded by the client
// in case of need, this sets the checksums that the client will use to verify the artifact
func (f *WagonDocument) SetChecksums(c artifact.Checksums) {
	f.Checksums = c
}

// GetChecksums gets the checksum of the repository file.
// Each repository can ship arbitrary file that will be downloaded by the client
// in case of need, this gets the checksums that the client will use to verify the artifact
func (f *WagonDocument) GetChecksums() artifact.Checksums {
	return f.Checksums
}
