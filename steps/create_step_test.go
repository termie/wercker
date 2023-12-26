//   Copyright 2017 Wercker Holding BV
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

package steps

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_CreateTarBall(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err, "Unable to get current working directory")

	// See testdata/README.md for the details of the test cases.
	tests := []struct {
		Name string
		Path string
	}{
		{"invalid_symlink_abs", path.Join(cwd, "testdata/invalid_symlink")},
		{"invalid_symlink", "testdata/invalid_symlink"},
		{"plain_step_abs", path.Join(cwd, "testdata/plain_step")},
		{"plain_step", "testdata/plain_step"},
		{"subdir_abs", path.Join(cwd, "testdata/subdir")},
		{"subdir", "testdata/subdir"},
		{"valid_symlink_abs", path.Join(cwd, "testdata/valid_symlink")},
		{"valid_symlink", "testdata/valid_symlink"},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var buf bytes.Buffer

			_, err := CreateTarball(test.Path, &buf)
			require.NoError(t, err)

			// Make sure that we're able to read the entire tarball
			r := bytes.NewReader(buf.Bytes())

			readEntireTarArchive(t, r, true)
		})
	}
}

func Test_CreateTarBall_Invalid(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err, "Unable to get current working directory")

	// See testdata/README.md for the details of the test cases.
	tests := []struct {
		Name string
		Path string
	}{
		{"non_existing_dir_abs", path.Join(cwd, "testdata/non_existing_dir")},
		{"non_existing_dir", "testdata/non_existing_dir"},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var buf bytes.Buffer

			_, err := CreateTarball(test.Path, &buf)
			require.Error(t, err)
		})
	}
}

// readEntireTarArchive will try to read r as a tarball. It will use
// require.*(t,...) when a error occurs.
func readEntireTarArchive(t *testing.T, r io.Reader, useGzip bool) {
	if useGzip {
		var err error
		r, err = gzip.NewReader(r)
		require.NoError(t, err)
	}

	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// This is a valid error. It means the end of the tarball was reached.
			break
		}
		require.NoError(t, err, "Unable to iterate to next entry")

		// Read every file, but discard the contents
		// NOTE(bvdberg): test this with symlinks
		_, err = io.Copy(ioutil.Discard, tr)
		require.NoError(t, err, "Unable to read file from tarball: %s", hdr.Name)
	}
}
