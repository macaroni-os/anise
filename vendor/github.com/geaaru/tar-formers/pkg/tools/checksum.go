/*
Copyright © 2021-2026 Daniele Rondina <geaaru@macaronios.org>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package tools

import (
	"crypto/md5"
	"crypto/sha512"
	"fmt"
	"hash"

	"golang.org/x/crypto/blake2b"
)

type FileHashesWriter struct {
	sha512  hash.Hash
	blake2b hash.Hash
	md5     hash.Hash
	size    int64
}

func NewFileHashesWriter() *FileHashesWriter {
	bhash, _ := blake2b.New512([]byte{})
	return &FileHashesWriter{
		md5:     md5.New(),
		sha512:  sha512.New(),
		blake2b: bhash,
		size:    int64(0),
	}
}

func (f *FileHashesWriter) Write(b []byte) (int, error) {
	n := len(b)

	if n > 0 {
		// Increment byte counter
		f.size += int64(n)

		// Update md5
		if _, err := f.md5.Write(b); err != nil {
			return n, err
		}

		// Update sha512
		if _, err := f.sha512.Write(b); err != nil {
			return n, err
		}

		// Update blake2b
		if _, err := f.blake2b.Write(b); err != nil {
			return n, err
		}
	}

	return n, nil
}

func (f *FileHashesWriter) Size() int64 {
	return f.size
}

func (f *FileHashesWriter) MD5() string {
	return fmt.Sprintf("%x", f.md5.Sum(nil))
}

func (f *FileHashesWriter) Sha512() string {
	return fmt.Sprintf("%x", f.sha512.Sum(nil))
}

func (f *FileHashesWriter) Blake2b() string {
	return fmt.Sprintf("%x", f.blake2b.Sum(nil))
}
