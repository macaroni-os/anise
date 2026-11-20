/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package file

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/continuity/sysx"
	"github.com/google/renameio"
	copy "github.com/otiai10/copy"
	"github.com/pkg/errors"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func Move(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	t, err := renameio.TempFile("", dst)
	if err != nil {
		return err
	}
	defer t.Cleanup()

	_, err = io.Copy(t, f)
	if err != nil {
		return err
	}
	return t.CloseAtomicallyReplace()
}

// ResolvePathInRoot resolves symlinks relative to root, treating root as /.
// The returned path describes the lookup at that moment. It can change if the filesystem is modified.
func ResolvePathInRoot(root, path string) (string, error) {
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	defer rootFS.Close()

	const maxSymlinks = 40
	const separator = string(filepath.Separator)
	resolved := "."
	for links := 0; path != ""; {
		var component string
		component, path, _ = strings.Cut(path, separator)
		switch component {
		case "", ".":
			continue
		case "..":
			resolved = filepath.Dir(resolved)
			continue
		}

		next := filepath.Join(resolved, component)
		info, err := rootFS.Lstat(next)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if info == nil || info.Mode()&os.ModeSymlink == 0 {
			resolved = next
			continue
		}
		if links == maxSymlinks {
			return "", &os.PathError{Op: "resolve", Path: filepath.Join(root, next), Err: syscall.ELOOP}
		}
		links++

		destination, err := rootFS.Readlink(next)
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(destination) {
			resolved = "."
		}
		path = destination + separator + path
	}
	return filepath.Join(root, resolved), nil
}

func OrderFiles(target string, files []string) ([]string, []string, []string) {
	var newFiles []string
	var notPresent []string
	dirs := []string{}

	for _, f := range files {
		completePath := filepath.Join(target, f)
		fi, err := os.Lstat(completePath)
		if err != nil {
			notPresent = append(notPresent, f)
			continue
		}

		if fi.IsDir() && (fi.Mode()&fs.ModeSymlink == 0) {
			// POST: found a directory that is not a link to another directory.
			dirs = append(dirs, f)
		} else {
			newFiles = append(newFiles, f)
		}
	}

	// Compare how many sub paths there are, and push at the end the ones
	// that have less subpaths
	sort.Slice(dirs, func(i, j int) bool {
		return len(strings.Split(dirs[i],
			string(os.PathSeparator))) > len(strings.Split(dirs[j],
			string(os.PathSeparator)))
	})

	return newFiles, dirs, notPresent
}

// DirectoryIsEmpty Checks wether the directory is empty or not
func DirectoryIsEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()

	if _, err = f.Readdirnames(1); err == io.EOF {
		return true, nil
	}
	return false, nil
}

// Touch creates an empty file
func Touch(f string) error {
	_, err := os.Stat(f)
	if os.IsNotExist(err) {
		file, err := os.Create(f)
		if err != nil {
			return err
		}
		defer file.Close()
	} else {
		currentTime := time.Now().Local()
		err = os.Chtimes(f, currentTime, currentTime)
		if err != nil {
			return err
		}
	}
	return nil
}

// Exists reports whether the named file or directory exists.
func Exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func ExistsLink(name string) bool {
	if _, err := os.Lstat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func Read(file string) (string, error) {
	dat, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(dat), nil
}

func EnsureDirPerm(src, dst string) {
	if info, err := os.Lstat(filepath.Dir(src)); err == nil {
		if _, err := os.Lstat(filepath.Dir(dst)); os.IsNotExist(err) {
			err := os.MkdirAll(filepath.Dir(dst), info.Mode().Perm())
			if err != nil {
				fmt.Println("warning: failed creating", filepath.Dir(dst), err.Error())
			}
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				if err := os.Lchown(filepath.Dir(dst), int(stat.Uid), int(stat.Gid)); err != nil {
					fmt.Println("warning: failed chowning", filepath.Dir(dst), err.Error())
				}
			}
		}
	} else {
		EnsureDir(dst)
	}
}

func EnsureDir(fileName string) error {
	dirName := filepath.Dir(fileName)
	if _, serr := os.Stat(dirName); os.IsNotExist(serr) {
		merr := os.MkdirAll(dirName, os.ModePerm) // FIXME: It should preserve permissions from src to dst instead
		if merr != nil {
			return merr
		}
	}
	return nil
}

func CopyFile(src, dst string) (err error) {
	return copy.Copy(src, dst, copy.Options{
		Sync:      true,
		OnSymlink: func(string) copy.SymlinkAction { return copy.Shallow }})
}

func copyXattr(srcPath, dstPath, attr string) error {
	data, err := sysx.LGetxattr(srcPath, attr)
	if errors.Is(err, sysx.ENODATA) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read extended attribute %q on %q: %w", attr, srcPath, err)
	}
	if data != nil {
		if err := sysx.LSetxattr(dstPath, attr, data, 0); err != nil {
			return fmt.Errorf("set extended attribute %q on %q: %w", attr, dstPath, err)
		}
	}
	return nil
}

func doCopyXattrs(srcPath, dstPath string) error {
	if err := copyXattr(srcPath, dstPath, "security.capability"); err != nil {
		return err
	}

	return copyXattr(srcPath, dstPath, "trusted.overlay.opaque")
}

// DeepCopyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file. The file mode will be copied from the source and
// the copied data is synced/flushed to stable storage.
func DeepCopyFile(src, dst string) (err error) {
	// Workaround for https://github.com/otiai10/copy/issues/47
	fi, err := os.Lstat(src)
	if err != nil {
		return errors.Wrap(err, "error reading file info")
	}

	fm := fi.Mode()
	switch {
	case fm&os.ModeNamedPipe != 0:
		EnsureDirPerm(src, dst)
		if err := syscall.Mkfifo(dst, uint32(fi.Mode())); err != nil {
			return errors.Wrap(err, "failed creating pipe")
		}
		if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
			if err := os.Chown(dst, int(stat.Uid), int(stat.Gid)); err != nil {
				return errors.Wrap(err, "failed chowning file")
			}
		}
		return nil
	}

	//filepath.Dir(src)
	EnsureDirPerm(src, dst)

	err = copy.Copy(src, dst, copy.Options{
		Sync:      true,
		OnSymlink: func(string) copy.SymlinkAction { return copy.Shallow }})
	if err != nil {
		return err
	}
	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		if err := os.Lchown(dst, int(stat.Uid), int(stat.Gid)); err != nil {
			fmt.Println("warning: failed chowning", dst, err.Error())
		}
	}

	return doCopyXattrs(src, dst)
}

func IsDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), err
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
// Symlinks are ignored and skipped.
func CopyDir(src string, dst string) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	return copy.Copy(src, dst, copy.Options{
		Sync:      true,
		OnSymlink: func(string) copy.SymlinkAction { return copy.Shallow }})
}

func Rel2Abs(s string) (string, error) {
	pathToSet := s
	if !filepath.IsAbs(s) {
		abs, err := filepath.Abs(s)
		if err != nil {
			return "", err
		}
		pathToSet = abs
	}
	return pathToSet, nil
}

func Sha256Sum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Try to resolve the abs path of the passed param
// Return itself if the binary is not present in the
// default paths (/sbin, /bin, /usr/sbin, /usr/bin)
func TryResolveBinaryAbsPath(b string) string {
	ans := b
	possiblePaths := []string{
		"/sbin",
		"/bin",
		"/usr/sbin",
		"/usr/bin",
	}

	for _, s := range possiblePaths {
		abs := filepath.Join(s, b)
		if Exists(abs) {
			ans = filepath.Join(abs)
			break
		}
	}

	return ans
}
