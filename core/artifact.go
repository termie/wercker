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

package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wercker/wercker/util"
)

// Artifact holds the information required to extract a folder
// from a container and eventually upload it to S3.
type Artifact struct {
	ContainerID   string
	GuestPath     string
	HostTarPath   string
	HostPath      string
	ApplicationID string
	RunID         string
	RunStepID     string
	Bucket        string
	Key           string
	ContentType   string
	Meta          map[string]*string
}

// URL returns the artifact's S3 url
func (art *Artifact) URL() string {
	return fmt.Sprintf("https://s3.amazonaws.com/%s/%s", art.Bucket, art.RemotePath())
}

// RemotePath returns the S3 path for an artifact
func (art *Artifact) RemotePath() string {
	if art.Key != "" {
		return art.Key
	}

	path := fmt.Sprintf("project-artifacts/%s/%s", art.ApplicationID, art.RunID)
	if art.RunStepID != "" {
		path = fmt.Sprintf("%s/step/%s", path, art.RunStepID)
	}
	path = fmt.Sprintf("%s/%s", path, filepath.Base(art.HostTarPath))
	return path
}

// Cleanup removes files from the host
func (art *Artifact) Cleanup() error {
	return os.Remove(art.HostPath)
}

// FileCollector gets files out of containers
type FileCollector interface {
	Collect(path string) (*util.Archive, chan error)
}
