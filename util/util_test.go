//   Copyright 2016 Wercker Holding BV
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package util

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type UtilSuite struct {
	*TestSuite
}

func TestUtilSuite(t *testing.T) {
	suiteTester := &UtilSuite{&TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *UtilSuite) TestCounterIncrement() {
	counter := &Counter{}
	s.Equal(0, counter.Current, "expected counter to intialize with 0")

	n1 := counter.Increment()
	s.Equal(0, n1, "expected first increment to be 0")

	n2 := counter.Increment()
	s.Equal(1, n2, "expected second increment to be 0")
}

func (s *UtilSuite) TestCounterIncrement2() {
	counter := &Counter{Current: 3}
	s.Equal(3, counter.Current, "expected counter to intialize with 3")

	n1 := counter.Increment()
	s.Equal(3, n1, "expected first increment to be 3")

	n2 := counter.Increment()
	s.Equal(4, n2, "expected second increment to be 4")
}

func (s *UtilSuite) TestMinInt() {
	testSteps := []struct {
		input    []int
		expected int
	}{
		{[]int{}, 0},
		{[]int{1}, 1},
		{[]int{1, 2}, 1},
		{[]int{2, 1}, 1},
		{[]int{1, 1}, 1},
		{[]int{5, 4, 3, 5, 7, 4}, 3},
	}

	for _, test := range testSteps {
		actual := MinInt(test.input...)

		s.Equal(test.expected, actual)
	}
}

func (s *UtilSuite) TestMaxInt() {
	testSteps := []struct {
		input    []int
		expected int
	}{
		{[]int{}, 0},
		{[]int{1}, 1},
		{[]int{1, 2}, 2},
		{[]int{2, 1}, 2},
		{[]int{1, 1}, 1},
		{[]int{5, 4, 3, 5, 7, 4}, 7},
	}

	for _, test := range testSteps {
		actual := MaxInt(test.input...)

		s.Equal(test.expected, actual)
	}
}

func (s *UtilSuite) TestSplitFunc() {
	testCases := []struct {
		input  string
		output []string
	}{
		{"hello, world", []string{"hello", "world"}},
		{"hello world", []string{"hello", "world"}},
		{"hello,              world", []string{"hello", "world"}},
		{"hello,world", []string{"hello", "world"}},
		{"hello                     world", []string{"hello", "world"}},
	}

	for _, test := range testCases {
		actual := SplitSpaceOrComma(test.input)
		s.Equal(test.output, actual)
		s.Equal(len(test.output), 2)
	}
}

// TestFileInfo is a dummy os.FileInfo for testing
type TestFileInfo struct {
	modTime time.Time
	name    string
}

func (t TestFileInfo) ModTime() time.Time {
	return t.modTime
}

func (t TestFileInfo) Name() string {
	return t.name
}

func (t TestFileInfo) Size() int64 {
	return 0
}

func (t TestFileInfo) Mode() os.FileMode {
	return 0
}

func (t TestFileInfo) IsDir() bool {
	return true
}

func (t TestFileInfo) Sys() interface{} {
	return nil
}

func (s *UtilSuite) TestSortByModDate() {
	// create 5 fake file infos, the first one being the oldest
	dirs := []os.FileInfo{}
	for day := 1; day <= 5; day++ {
		dirs = append(dirs, TestFileInfo{
			// offset modified time so it's jan 1st, 2nd, etc
			modTime: time.Date(2016, 1, day, 12, 0, 0, 0, time.UTC),
			name:    fmt.Sprintf("jan-%v", day),
		})
	}

	// before sort the first item is the one we added first,
	// ignoring the modtime
	s.Equal("jan-1", dirs[0].Name())

	SortByModDate(dirs)

	// after sort the one with the most recent mod time is first
	s.Equal("jan-5", dirs[0].Name())
	s.Equal("jan-4", dirs[1].Name())
	s.Equal("jan-3", dirs[2].Name())
	s.Equal("jan-2", dirs[3].Name())
	s.Equal("jan-1", dirs[4].Name())
}

func (s *UtilSuite) TestConvertUnit() {
	tests := []struct {
		input         int64
		expectedValue int64
		expectedUnit  string
	}{
		{1, 1, "B"},
		{1024, 1, "KiB"},
		{2048, 2, "KiB"},
		{1047552, 1023, "KiB"},
		{1048576, 1, "MiB"},
		{1073741824, 1, "GiB"},
		{1099511627776, 1024, "GiB"},
		{1100585369600, 1025, "GiB"}, // GiB is the last unit
	}

	for _, test := range tests {
		actualValue, actualUnit := ConvertUnit(test.input)

		s.Equal(test.expectedValue, actualValue)
		s.Equal(test.expectedUnit, actualUnit)

	}

}

func (s *UtilSuite) TestTarPathWithRoot() {
	testName := "TestTarPathWithRoot"
	sourcePath := path.Join("testdata", "testTarPath")
	symlinkTargetPath := path.Join("testdata", "testTarPath2")

	// first of all create an empty directory to use in the test
	// we need to do this because we can't store empty directories in git
	dir3 := path.Join(sourcePath, "dir3")
	os.Mkdir(dir3, 0777)
	defer os.RemoveAll(dir3)

	// now create a symbolic link to a regular file
	symlinkTargetPathAbs, err := filepath.Abs(symlinkTargetPath)
	s.NoError(err)
	symFile1 := path.Join(sourcePath, "symFile1")
	targetFile := path.Join(symlinkTargetPathAbs, "file1")
	os.Symlink(targetFile, symFile1)
	defer os.Remove(symFile1)

	// verify we've created the symbolic link to a regular file correctly
	assertSymbolicLinkToFile(s, symFile1)

	// now create a symbolic link to a directory
	symDir1 := path.Join(sourcePath, "symDir1")
	targetDir := symlinkTargetPathAbs
	os.Symlink(targetDir, symDir1)
	defer os.Remove(symDir1)

	// verify we've created the symbolic link to a directory correctly
	assertSymbolicLinkToDirectory(s, symDir1)

	// create a tarball which contains the contents of the test directory under a specified parent directory
	tarfileName := testName + ".tar"
	parent := "parent"

	// create a file that we will write the tarball to
	fw, err := os.Create(tarfileName)
	s.Require().NoError(err)
	defer fw.Close()
	defer os.Remove(tarfileName)

	// now create the tarball
	err = TarPathWithRoot(fw, sourcePath, parent)
	s.Require().NoError(err)

	// now unpack the tarball
	untarDir := path.Join("testdata", testName+"Out")
	defer os.RemoveAll(untarDir)

	reader, err := os.Open(tarfileName)
	s.Require().NoError(err)
	defer reader.Close()
	err = Untar(untarDir, reader)

	// finally check what we have
	assertSymbolicLinkToDirectory(s, path.Join(untarDir, parent, "symDir1"))
	assertSymbolicLinkToFile(s, path.Join(untarDir, parent, "symFile1"))

	assertRegularFile(s, path.Join(untarDir, parent, "file1"))
	assertRegularFile(s, path.Join(untarDir, parent, "file2"))
	assertRegularFile(s, path.Join(untarDir, parent, "dir1", "dir1File1"))

	assertRegularFile(s, path.Join(untarDir, parent, "dir1", "dir1File2"))
	assertRegularFile(s, path.Join(untarDir, parent, "dir2", "dir2File1"))

	assertDirectory(s, path.Join(untarDir, parent, "dir1"))

	assertDirectory(s, path.Join(untarDir, parent, "dir2"))
	assertDirectory(s, path.Join(untarDir, parent, "dir3"))

	// count the number of files under the parent to verify that the synlinks were ignored
	files, err := ioutil.ReadDir(path.Join(untarDir, parent))
	s.Require().Equal(7, len(files), "Unexpected number of files in directtory")
}

func assertSymbolicLinkToFile(s *UtilSuite, filename string) {
	fi, err := os.Lstat(filename)
	s.Require().NoError(err)
	s.Require().NotNil(fi)
	s.Require().True(fi.Mode()&os.ModeSymlink == os.ModeSymlink, filename+" is not a symbolic link")

	// now follow the link and check it
	// the target file should contain its own name (just the plain name, omitting the path)
	resolvedFilename, err := filepath.EvalSymlinks(filename)
	s.Require().NoError(err)
	assertRegularFile(s, resolvedFilename)
}

func assertSymbolicLinkToDirectory(s *UtilSuite, filename string) {
	fi, err := os.Lstat(filename)
	s.Require().NoError(err)
	s.Require().NotNil(fi)
	s.Require().True(fi.Mode()&os.ModeSymlink == os.ModeSymlink, filename+" is not a symbolic link")

	// now follow the link and check it
	resolvedFilename, err := filepath.EvalSymlinks(filename)
	s.Require().NoError(err)
	assertDirectory(s, resolvedFilename)
}

func assertRegularFile(s *UtilSuite, filename string) {
	fi, err := os.Lstat(filename)
	s.Require().NoError(err)
	s.Require().NotNil(fi)
	s.True(fi.Mode().IsRegular(), filename+" is not a regular file")

	// each file should contain its own name (just the plain name, omitting the path)
	content, err := ioutil.ReadFile(filename)
	s.Require().NoError(err)
	expectedContent := filepath.Base(filename)
	s.Require().Equal(expectedContent, string(content), "unexpected file content")
}

func assertDirectory(s *UtilSuite, filename string) {
	fi, err := os.Lstat(filename)
	s.Require().NoError(err)
	s.Require().NotNil(fi)
	s.Require().True(fi.Mode().IsDir(), filename+" is not a directory")
}

// Untar takes a destination path and a reader; a tar reader loops over the tarfile
// creating the file structure at 'dst' along the way, and writing any files
func UntarTestUtil(dst string, r io.Reader) error {
	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		target := filepath.Join(dst, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}
		case tar.TypeSymlink:
			err = os.Symlink(header.Linkname, target)
			if err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer f.Close()

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
		}
	}
}
