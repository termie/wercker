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
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	homePrefix = "~/"

	maxTries = 3
)

// ExpandHomePath will expand ~/ in p to home.
func ExpandHomePath(p string, home string) string {
	if strings.HasPrefix(p, homePrefix) {
		return path.Join(home, strings.TrimPrefix(p, homePrefix))
	}

	return p
}

// exists is like python's os.path.exists and too many lines in Go
func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Get tries to make a GET request to url. It will retry, upto 3 times, when
// the response is http statuscode 5xx.
func Get(url, authToken string) (*http.Response, error) {
	return get(url, authToken, 1)
}

func get(url, authToken string, try int) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if authToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 200 {
		return resp, nil
	}

	if shouldRetry(try, resp) {
		time.Sleep(time.Duration(try*200) * time.Millisecond)
		return get(url, authToken, try+1)
	}

	return resp, fmt.Errorf("Bad status code while fetching: %s (%d)", url, resp.StatusCode)
}

// shouldRetry checks whether try exceeds maxRetries, if it doesn't it will
// check if the status code is 5xx.
func shouldRetry(try int, resp *http.Response) bool {
	if try > maxTries {
		return false
	}

	return resp.StatusCode >= 500 && resp.StatusCode < 600
}

// UntarOne writes the contents up a single file to dst
func UntarOne(name string, dst io.Writer, src io.ReadCloser) error {
	// ungzipped, err := gzip.NewReader(src)
	// if err != nil {
	//   return err
	// }
	tarball := tar.NewReader(src)
	defer src.Close()
	// defer tarball.Close()

	for {
		hdr, err := tarball.Next()
		if err == io.EOF {
			// finished the tar
			break
		}
		if err != nil {
			return err
		}

		if hdr.Name != name {
			continue
		}

		// We found the file we care about
		_, err = io.Copy(dst, tarball)
		break
	}
	return nil
}

// Untargzip tries to untar-gzip stuff to a path
func Untargzip(path string, r io.Reader) error {
	ungzipped, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tarball := tar.NewReader(ungzipped)

	defer ungzipped.Close()

	// We have to treat things differently for git-archives
	isGitArchive := false

	// Alright, things seem in order, let's make the base directory
	os.MkdirAll(path, 0755)
	for {
		hdr, err := tarball.Next()
		if err == io.EOF {
			// finished the tar
			break
		}
		if err != nil {
			return err
		}
		// Skip the base dir
		if hdr.Name == "./" {
			continue
		}

		// If this was made with git-archive it will be in kinda an ugly
		// format, but we can identify it by the pax_global_header "file"
		name := hdr.Name
		if name == "pax_global_header" {
			isGitArchive = true
			continue
		}

		// It will also contain an extra subdir that we will automatically strip
		if isGitArchive {
			parts := strings.Split(name, "/")
			name = strings.Join(parts[1:], "/")
		}

		fpath := filepath.Join(path, name)
		if hdr.FileInfo().IsDir() {
			err = os.MkdirAll(fpath, 0755)
			if err != nil {
				return err
			}
			continue
		}
		file, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE, hdr.FileInfo().Mode())
		defer file.Close()
		if err != nil {
			return err
		}
		_, err = io.Copy(file, tarball)
		if err != nil {
			return err
		}
		file.Close()
	}
	return nil
}

// Untar takes a destination path and a reader; a tar reader loops over the tarfile
// creating the file structure at 'dst' along the way, and writing any files
func Untar(dst string, r io.Reader) error {
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

// TarPath makes a tarfile out of a directory
// The contents of the "root" directory will be placed at the top level in the tarfile.
// Directory names are not written to the tarfile.
func TarPath(writer io.Writer, root string) error {
	tw := tar.NewWriter(writer)
	defer tw.Close()
	walkFn := func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsDir() {
			return nil
		}
		newPath := path[len(root)+1:]
		if len(newPath) == 0 {
			return nil
		}

		fr, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()

		hdr, err := tar.FileInfoHeader(info, newPath)
		if err != nil {
			return err
		}
		hdr.Uid = 0
		hdr.Gid = 0
		hdr.Name = newPath
		err = tw.WriteHeader(hdr)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, fr)
		if err != nil {
			return err
		}
		return nil
	}

	err := filepath.Walk(root, walkFn)
	if err != nil {
		return err
	}
	return nil
}

// TarPathWithRoot makes a tarfile from the files under "root".
// A directory named "tarRoot" will be created at the top level
// in the tarball, with the files under "root" placed under it.
// Directory names are written to the tarfile (including empty directories).
// Symbolic links are ignored (walk doesn't follow symbolic links anyway).
func TarPathWithRoot(writer io.Writer, root string, tarRoot string) error {
	tw := tar.NewWriter(writer)
	defer tw.Close()

	walkFn := func(currentPath string, info os.FileInfo, err error) error {

		var newPath string
		if len(currentPath) > len(root) {
			newPath = currentPath[len(root)+1:]
		}
		if tarRoot != "" {
			newPath = path.Join(tarRoot, newPath)
		}
		if len(newPath) == 0 {
			return nil
		}

		var link = ""
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			link, err = os.Readlink(currentPath)
			if err != nil {
				return err
			}
		}

		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Uid = 0
		hdr.Gid = 0
		hdr.Name = newPath
		err = tw.WriteHeader(hdr)
		if err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		// File
		fr, err := os.Open(currentPath)
		if err != nil {
			return err
		}
		defer fr.Close()

		_, err = io.Copy(tw, fr)
		if err != nil {
			return err
		}
		return nil
	}

	err := filepath.Walk(root, walkFn)
	if err != nil {
		return err
	}
	return nil
}

// Finisher is a helper class for running something either right away or
// at `defer` time.
type Finisher struct {
	callback   func(interface{})
	isFinished bool
}

// NewFinisher returns a new Finisher with a callback.
func NewFinisher(callback func(interface{})) *Finisher {
	return &Finisher{callback: callback, isFinished: false}
}

// Finish executes the callback if it hasn't been run yet.
func (f *Finisher) Finish(result interface{}) {
	if f.isFinished {
		return
	}
	f.isFinished = true
	f.callback(result)
}

// Counter is a simple struct
type Counter struct {
	Current int
	l       sync.Mutex
}

// Increment will return current and than increment c.Current.
func (c *Counter) Increment() int {
	c.l.Lock()
	defer c.l.Unlock()

	current := c.Current
	c.Current = current + 1

	return current
}

// ContainsString checks if the array items contains the string target.
// TODO(bvdberg): write units tests
func ContainsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// QueryString converts a struct to a map. It looks for items with a qs tag.
// This code was taken from the fsouza/go-dockerclient, and then slightly
// modified. See: https://github.com/fsouza/go-dockerclient/blob/5fa67ac8b52afe9430a490391a639085e9357e1e/client.go#L535
func QueryString(opts interface{}) map[string]interface{} {
	items := map[string]interface{}{}
	if opts == nil {
		return items
	}
	value := reflect.ValueOf(opts)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return items
	}
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		key := field.Tag.Get("qs")
		if key == "" {
			key = strings.ToLower(field.Name)
		} else if key == "-" {
			continue
		}
		v := value.Field(i)
		switch v.Kind() {
		case reflect.Bool:
			if v.Bool() {
				items[key] = "1"
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if v.Int() > 0 {
				items[key] = strconv.FormatInt(v.Int(), 10)
			}
		case reflect.Float32, reflect.Float64:
			if v.Float() > 0 {
				items[key] = strconv.FormatFloat(v.Float(), 'f', -1, 64)
			}
		case reflect.String:
			if v.String() != "" {
				items[key] = v.String()
			}
		case reflect.Ptr:
			if !v.IsNil() {
				if b, err := json.Marshal(v.Interface()); err == nil {
					items[key] = string(b)
				}
			}
		case reflect.Map:
			if len(v.MapKeys()) > 0 {
				if b, err := json.Marshal(v.Interface()); err == nil {
					items[key] = string(b)
				}
			}
		}
	}
	return items
}

func SplitSpaceOrComma(str string) []string {
	return strings.FieldsFunc(str, func(c rune) bool {
		return unicode.IsSpace(c) || c == ','
	})
}

// CounterReader is a io.Reader which wraps a other io.Reader and stores the
// bytes reader from it.
type CounterReader struct {
	r io.Reader
	c int64
}

// NewCounterReader creates a new CounterReader.
func NewCounterReader(r io.Reader) *CounterReader {
	return &CounterReader{r: r}
}

// Read proxy's the request to r, and stores the bytes read as reported by r.
func (c *CounterReader) Read(p []byte) (int, error) {
	read, err := c.r.Read(p)

	c.c += int64(read)

	return read, err
}

// Count returns the bytes read from r.
func (c *CounterReader) Count() int64 {
	return c.c
}

// MinInt finds the smallest int in input and return that value. If no input is
// given, it will return 0.
func MinInt(input ...int) int {
	if len(input) == 0 {
		return 0
	}

	min := input[0]
	for _, in := range input {
		if in < min {
			min = in
		}
	}

	return min
}

// MaxInt finds the biggest int in input and return that value. If no input is
// given, it will return 0.
func MaxInt(input ...int) int {
	if len(input) == 0 {
		return 0
	}

	max := input[0]
	for _, in := range input {
		if in > max {
			max = in
		}
	}
	return max
}

// Timer so we can dump step timings
type Timer struct {
	begin time.Time
}

// NewTimer ctor
func NewTimer() *Timer {
	return &Timer{begin: time.Now()}
}

// Reset that timer
func (t *Timer) Reset() {
	t.begin = time.Now()
}

// Elapsed duration
func (t *Timer) Elapsed() time.Duration {
	return time.Now().Sub(t.begin)
}

// String repr for time
func (t *Timer) String() string {
	return fmt.Sprintf("%.2fs", t.Elapsed().Seconds())
}

// ByModifiedTime is used for sorting files/folders by mod time
type ByModifiedTime []os.FileInfo

// Len, Swap and Less are used for sorting

// Len returns the length of items in the slice
func (s ByModifiedTime) Len() int {
	return len(s)
}

// Swap swaps two items when sorting
func (s ByModifiedTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns true if the first value shoudl appear first in the
// sorted results
func (s ByModifiedTime) Less(i, j int) bool {
	return s[i].ModTime().After(s[j].ModTime())
}

// SortByModDate sorts the files or folders descending by modification date
func SortByModDate(dirs []os.FileInfo) {
	sort.Sort(ByModifiedTime(dirs))
}

var units = []string{
	"B",
	"KiB",
	"MiB",
	"GiB",
}

// ConvertUnit takes the number of bytes and converts this to the largest unit
// possible, where the result is still > 1. Uses default golang int rounding.
func ConvertUnit(size int64) (int64, string) {
	unit := ""

	for i, u := range units {
		unit = u

		// No need to continue when it is smaller than 1024
		if size < 1024 {
			break
		}

		// Do not divide on the last item
		if i+1 < len(units) {
			size = size / 1024
		}
	}

	return size, unit
}

// SqaushErrors takes multiple error objects and returns a single error which
// will enumerate all Error() from the wrapped error objects. If errors
// contains no error objects, then it will return nil.
func SqaushErrors(errors []error) error {
	if len(errors) > 0 {
		return multiError(errors)
	}

	return nil
}

type multiError []error

func (e multiError) Error() string {
	if len(e) == 0 {
		return ""
	}

	buf := make([]string, len(e))
	for i, err := range []error(e) {
		buf[i] = err.Error()
	}

	return strings.Join(buf, "; ")
}
