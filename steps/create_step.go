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
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// CreateTarball will create a step tarball based on path.
func CreateTarball(path string, w io.Writer) (string, error) {
	if !strings.HasSuffix(path, "/") {
		path = fmt.Sprintf("%s/", path)
	}

	h := sha256.New()

	err := createTarball(path, io.MultiWriter(h, w))
	if err != nil {
		return "", err
	}

	checksum := hex.EncodeToString(h.Sum(nil))

	return checksum, nil
}

func createTarball(path string, w io.Writer) error {
	gw := gzip.NewWriter(w)

	tw := tar.NewWriter(gw)

	err := filepath.Walk(path, createWalkFunc(tw, path))
	if err != nil {
		return err
	}

	// Make sure we close it, since it will output two zero blocks.
	err = tw.Close()
	if err != nil {
		return errors.Wrap(err, "Unable to close tar writer")
	}

	// Also make sure that the gzip writer is closed, as this contains buffered
	// contents.
	err = gw.Close()
	if err != nil {
		return errors.Wrap(err, "Unable to close gzip writer")
	}

	return nil
}

// createTarWalk will return a func which should be used with filepath.Walk. It
// will add all files found and writes them to w. Optionally skipping over
// directories or files using the shouldIgnore func.
func createWalkFunc(w *tar.Writer, rootPath string) func(string, os.FileInfo, error) error {
	return func(p string, info os.FileInfo, err error) error {
		logger := log.WithField("path", p)

		if err != nil {
			logger.WithError(err).Warnf("Unable to access file or directory: %s", p)
			return err
		}

		isDir := info.IsDir()
		isSymlink := !isDir && !info.Mode().IsRegular()

		if shouldIgnore(p, info) {
			logger.Debugf("Skipping %s", p)
			return filepath.SkipDir
		}

		var link string
		if isSymlink {
			// We need to get the target of the symlink and pass that to
			// tar.FileInfoHeader.
			link, err = os.Readlink(p)
			if err != nil {
				return errors.Wrap(err, "Unable to read symlink")
			}
		}

		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return errors.Wrap(err, "Unable to create tar header from file info")
		}

		// Update the name of the tar header to include the entire path (from rootPath)
		hdr.Name = strings.TrimPrefix(p, rootPath)
		if hdr.Name == "" {
			return nil
		}

		w.WriteHeader(hdr)

		// When it is a directory or a symlink, we only have to create the header
		if isDir || isSymlink {
			return nil
		}

		// Read the source file and copy it into the tarball
		f, err := os.Open(p)
		if err != nil {
			return errors.Wrap(err, "Unable to access file")
		}
		defer f.Close()

		_, err = io.Copy(w, f)
		if err != nil {
			logger.Error("Unable to copy")
			return errors.Wrap(err, "Unable to copy source file to tar")
		}

		return nil
	}
}

func shouldIgnore(path string, info os.FileInfo) bool {
	if info.Name() == ".git" && info.IsDir() {
		return true
	}

	return false
}
