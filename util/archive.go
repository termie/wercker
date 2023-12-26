//   Copyright © 2016,2018, Oracle and/or its affiliates.  All rights reserved.
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
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

var (
	// ErrEmptyTarball is returned when the tarball has no files in it
	ErrEmptyTarball = errors.New("empty tarball")
)

// ArchiveProcessor is a stream processor for the archive tarballs
type ArchiveProcessor interface {
	Process(*tar.Header, io.Reader) (*tar.Header, io.Reader, error)
}

// Archive holds the tarball stream and provides methods to manipulate it
type Archive struct {
	stream io.Reader
	closer func()
}

// NewArchive constructor
// If the reader needs to be closed after use then closer must be set to a func which will close the reader
func NewArchive(stream io.Reader, closer func()) *Archive {
	return &Archive{stream: stream, closer: closer}
}

// Close the archive
func (a *Archive) Close() {
	a.closer()
}

// Tee the tar stream to your own writer
func (a *Archive) Tee(w io.Writer) {
	newReader := io.TeeReader(a.stream, w)
	a.stream = newReader
}

// Stream is the low-level interface to the archive stream processor
func (a *Archive) Stream(processors ...ArchiveProcessor) error {
	tarball := tar.NewReader(a.stream)

	// If we don't eat the rest of the stream Docker 1.9+ seems to choke
	defer func() {
		io.Copy(ioutil.Discard, a.stream)
	}()

	var tarfile io.Reader
EntryLoop:
	for {
		hdr, err := tarball.Next()
		if err == io.EOF {
			// finished the tar
			break
		}
		// basic filter, we never care about this entry
		if hdr.Name == "./" {
			continue EntryLoop
		}

		tarfile = tarball
		for _, processor := range processors {
			hdr, tarfile, err = processor.Process(hdr, tarfile)
			if err != nil {
				return err
			}
			// if hdr is nil, skip this file
			if hdr == nil {
				continue EntryLoop
			}
		}
	}
	return nil
}

// Single file extraction with max size and empty check
func (a *Archive) Single(source, target string, maxSize int64) (errs chan error) {
	errs = make(chan error)
	single := &ArchiveSingle{Name: source}
	empty := &ArchiveCheckEmpty{}
	max := &ArchiveMaxSize{MaxSize: maxSize}
	extract := NewArchiveExtract("")

	go func() {
		defer close(errs)
		defer extract.Clean()
		err := a.Stream(
			single,
			empty,
			max,
			extract,
		)
		if err != nil {
			errs <- err
			return
		}
		if empty.IsEmpty() {
			errs <- ErrEmptyTarball
			return
		}
		errs <- nil
	}()
	return errs
}

// Multi file extraction with max size and empty check
func (a *Archive) Multi(source, target string, maxSize int64) (errs chan error) {
	errs = make(chan error)
	empty := &ArchiveCheckEmpty{}
	max := &ArchiveMaxSize{MaxSize: maxSize}
	extract := NewArchiveExtract(filepath.Dir(target))

	go func() {
		defer close(errs)
		defer extract.Clean()
		err := a.Stream(
			empty,
			max,
			extract,
		)
		if err != nil {
			errs <- err
			return
		}
		if empty.IsEmpty() {
			errs <- ErrEmptyTarball
			return
		}
		errs <- extract.Rename(source, target)
	}()
	return errs
}

// SingleBytes gives you the bytes of a single file, with empty check
func (a *Archive) SingleBytes(source string, dst *bytes.Buffer) chan error {
	single := &ArchiveSingle{Name: source}
	empty := &ArchiveCheckEmpty{}
	buffer := &ArchiveBytes{dst}

	errs := make(chan error)
	go func() {
		defer close(errs)
		err := a.Stream(
			single,
			empty,
			buffer,
		)
		if err != nil {
			errs <- err
			return
		}
		if empty.IsEmpty() {
			errs <- ErrEmptyTarball
			return
		}
		errs <- nil
	}()
	return errs
}

// ArchiveCheckEmpty is an ArchiveProcessor to check whether a stream is empty
type ArchiveCheckEmpty struct {
	hasFiles bool
}

// Process impl
func (p *ArchiveCheckEmpty) Process(hdr *tar.Header, r io.Reader) (*tar.Header, io.Reader, error) {
	if p.hasFiles {
		return hdr, r, nil
	}
	if !hdr.FileInfo().IsDir() {
		p.hasFiles = true
	}
	return hdr, r, nil
}

// IsEmpty will represent whether the tarball was empty after processing
func (p *ArchiveCheckEmpty) IsEmpty() bool {
	return !p.hasFiles
}

// ArchiveMaxSize throws an error and stop stream if MaxSize reached
type ArchiveMaxSize struct {
	MaxSize     int64 // in bytes
	currentSize int64 // in bytes
}

// Process impl
func (p *ArchiveMaxSize) Process(hdr *tar.Header, r io.Reader) (*tar.Header, io.Reader, error) {
	// Check max size
	p.currentSize += hdr.Size
	if p.currentSize >= p.MaxSize {
		err := fmt.Errorf("Size exceeds maximum size of %dMB", p.MaxSize/(1024*1024))
		return hdr, r, err
	}
	return hdr, r, nil
}

// ArchiveExtract everything to a tempdir, methods for Rename and Cleanup
type ArchiveExtract struct {
	// Target  string // target path
	// Source  string // path within the tarball
	workingDir string // path where temporary extraction occurs
	tempDir    string // base path for temp extraction
}

// NewArchiveExtract creates a new ArchiveExtract.
// tempDir is directory to perform work. Leave empty string to use default
func NewArchiveExtract(tempDir string) *ArchiveExtract {
	return &ArchiveExtract{tempDir: tempDir}
}

// Process impl
func (p *ArchiveExtract) Process(hdr *tar.Header, r io.Reader) (*tar.Header, io.Reader, error) {
	// make sure we have our tempdir
	if p.workingDir == "" {
		t, err := ioutil.TempDir(p.tempDir, "tar-")
		if err != nil {
			return hdr, r, err
		}
		p.workingDir = t
	}

	// If a directory make it and continue
	fpath := filepath.Join(p.workingDir, hdr.Name)

	switch hdr.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(fpath, 0755); err != nil {
			return hdr, r, err
		}
	case tar.TypeReg:
		file, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode))
		if err != nil {
			return hdr, r, err
		}
		defer file.Close()

		_, err = io.Copy(file, r)
		if err != nil {
			return hdr, r, err
		}
	case tar.TypeSymlink:
		err := os.Symlink(hdr.Linkname, fpath)
		if err != nil {
			return hdr, r, err
		}
	}

	return hdr, r, nil
}

// Rename one of the extracted paths to the target path
func (p *ArchiveExtract) Rename(source, target string) error {
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return os.Rename(filepath.Join(p.workingDir, source), target)
}

// Clean should be called to clean up the workingDir
func (p *ArchiveExtract) Clean() {
	if p.workingDir != "" {
		os.RemoveAll(p.workingDir)
	}
}

// ArchiveSingle filters all but a single item out of the string
type ArchiveSingle struct {
	Name string
}

// Process impl
func (p *ArchiveSingle) Process(hdr *tar.Header, r io.Reader) (*tar.Header, io.Reader, error) {
	if hdr.Name == p.Name {
		return hdr, r, nil
	}
	return nil, r, nil
}

// ArchiveBytes is expected to be used with an ArchiveSingle filter so that it
// only gets one file, if not the buffer will be pretty silly
type ArchiveBytes struct {
	*bytes.Buffer
}

// Process writes the bytes for a file to ourselves (a bytes.Buffer)
func (p *ArchiveBytes) Process(hdr *tar.Header, r io.Reader) (*tar.Header, io.Reader, error) {
	_, err := io.Copy(p, r)
	return hdr, r, err
}
