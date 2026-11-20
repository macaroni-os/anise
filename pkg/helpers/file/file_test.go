/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package file_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestResolvePathInRoot(t *testing.T) {
	g := NewWithT(t)
	root := t.TempDir()
	g.Expect(os.MkdirAll(filepath.Join(root, "usr", "lib64"), 0755)).To(Succeed())
	g.Expect(os.Symlink("lib64", filepath.Join(root, "usr", "lib"))).To(Succeed())
	g.Expect(os.Symlink("/usr/lib", filepath.Join(root, "lib"))).To(Succeed())
	g.Expect(os.Symlink("../../usr/lib", filepath.Join(root, "up"))).To(Succeed())
	g.Expect(os.Symlink("loop", filepath.Join(root, "loop"))).To(Succeed())

	for _, path := range []string{"usr/lib", "/lib", "/up", "../../lib"} {
		g.Expect(fileHelper.ResolvePathInRoot(root, path)).To(Equal(filepath.Join(root, "usr", "lib64")), "path %q", path)
	}
	g.Expect(fileHelper.ResolvePathInRoot(root, "/lib/new.so")).To(Equal(filepath.Join(root, "usr", "lib64", "new.so")))
	g.Expect(fileHelper.ResolvePathInRoot(root, "/lib/..")).To(Equal(filepath.Join(root, "usr")))
	_, err := fileHelper.ResolvePathInRoot(root, "/loop")
	g.Expect(err).To(MatchError(syscall.ELOOP))
}

var _ = Describe("Helpers", func() {
	Context("Exists", func() {
		It("Detect existing and not-existing files", func() {
			Expect(fileHelper.Exists("../../tests/fixtures/buildtree/app-admin/enman/1.4.0/build.yaml")).To(BeTrue())
			Expect(fileHelper.Exists("../../tests/fixtures/buildtree/app-admin/enman/1.4.0/build.yaml.not.exists")).To(BeFalse())
		})
	})

	Context("DirectoryIsEmpty", func() {
		It("Detects empty directory", func() {
			testDir, err := ioutil.TempDir(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)
			Expect(fileHelper.DirectoryIsEmpty(testDir)).To(BeTrue())
		})
		It("Detects directory with files", func() {
			testDir, err := ioutil.TempDir(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)
			err = fileHelper.Touch(filepath.Join(testDir, "foo"))
			Expect(err).ToNot(HaveOccurred())
			Expect(fileHelper.DirectoryIsEmpty(testDir)).To(BeFalse())
		})
	})

	Context("Orders dir and files correctly", func() {
		It("puts files first and folders at end", func() {
			testDir, err := ioutil.TempDir(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)

			err = ioutil.WriteFile(filepath.Join(testDir, "foo"), []byte("test\n"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = ioutil.WriteFile(filepath.Join(testDir, "baz"), []byte("test\n"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(filepath.Join(testDir, "bar"), 0755)
			Expect(err).ToNot(HaveOccurred())
			err = ioutil.WriteFile(filepath.Join(testDir, "bar", "foo"), []byte("test\n"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(filepath.Join(testDir, "baz2"), 0755)
			Expect(err).ToNot(HaveOccurred())
			err = ioutil.WriteFile(filepath.Join(testDir, "baz2", "foo"), []byte("test\n"), 0644)
			Expect(err).ToNot(HaveOccurred())

			ordered, dirs, notExisting := fileHelper.OrderFiles(testDir,
				[]string{
					"bar", "baz", "bar/foo", "baz2", "foo", "baz2/foo", "notexisting",
				})

			Expect(ordered).To(Equal([]string{"baz", "bar/foo", "foo", "baz2/foo"}))
			Expect(dirs).To(Equal([]string{"bar", "baz2"}))
			Expect(notExisting).To(Equal([]string{"notexisting"}))
		})

		It("orders correctly when there are folders with folders", func() {
			testDir, err := ioutil.TempDir(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)

			err = os.MkdirAll(filepath.Join(testDir, "bar"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())
			err = os.MkdirAll(filepath.Join(testDir, "foo"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(filepath.Join(testDir, "foo", "bar"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(filepath.Join(testDir, "foo", "baz"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())
			err = os.MkdirAll(filepath.Join(testDir, "foo", "baz", "fa"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			ordered, dirs, _ := fileHelper.OrderFiles(testDir,
				[]string{"foo", "foo/bar", "bar", "foo/baz/fa", "foo/baz"})
			Expect(ordered).To(Equal([]string{}))
			Expect(dirs).To(Equal([]string{"foo/baz/fa", "foo/bar", "foo/baz", "foo", "bar"}))
		})
	})
})
