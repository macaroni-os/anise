/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package file

// Code from macaroni-os/mark-devkit project

import (
	"crypto/md5"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"

	"golang.org/x/crypto/blake2b"
)

type FileHashesReader struct {
	fd      *os.File
	sha512  hash.Hash
	blake2b hash.Hash
	md5     hash.Hash
	size    int64
}

func NewFileHashesReader(file string) (*FileHashesReader, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("error on open file %s: %s",
			file, err.Error())
	}
	bhash, _ := blake2b.New512([]byte{})
	return &FileHashesReader{
		fd:      fd,
		md5:     md5.New(),
		sha512:  sha512.New(),
		blake2b: bhash,
		size:    int64(0),
	}, nil
}

func (f *FileHashesReader) Read(b []byte) (int, error) {
	n, err := f.fd.Read(b)
	if err != nil {
		return n, err
	}

	if n > 0 {

		// Increment byte counter
		f.size += int64(n)

		// Update md5
		_, err = f.md5.Write(b[:n])
		if err != nil {
			return n, err
		}

		// Update sha512
		_, err = f.sha512.Write(b[:n])
		if err != nil {
			return n, err
		}

		// Update blake2b
		_, err = f.blake2b.Write(b[:n])
	}

	return n, err
}

func (f *FileHashesReader) Close() error {
	return f.fd.Close()
}

func (f *FileHashesReader) Size() int64 {
	return f.size
}

func (f *FileHashesReader) MD5() string {
	return fmt.Sprintf("%x", f.md5.Sum(nil))
}

func (f *FileHashesReader) Sha512() string {
	return fmt.Sprintf("%x", f.sha512.Sum(nil))
}

func (f *FileHashesReader) Blake2b() string {
	return fmt.Sprintf("%x", f.blake2b.Sum(nil))
}

func GetFileHashes(f string) (*FileHashesReader, error) {
	reader, err := NewFileHashesReader(f)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	buffer := make([]byte, 1024)
	for {
		_, err = reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}

	return reader, nil
}

func GetFileMd5(f string) (string, error) {
	reader, err := GetFileHashes(f)
	if err != nil {
		return "", err
	}
	return reader.MD5(), nil
}

func GetFileSha512(f string) (string, error) {
	reader, err := GetFileHashes(f)
	if err != nil {
		return "", err
	}
	return reader.Sha512(), nil
}

func GetFileBlake2b(f string) (string, error) {
	reader, err := GetFileHashes(f)
	if err != nil {
		return "", err
	}
	return reader.Blake2b(), nil
}
