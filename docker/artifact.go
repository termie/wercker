//   Copyright © 2016, 2018, Oracle and/or its affiliates.  All rights reserved.
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

package dockerlocal

import (
	"os"
	"path/filepath"

	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// Set upper limit that we can store
const maxArtifactSize = 5000 * 1024 * 1024 // in bytes

// Artificer collects artifacts from containers and uploads them.
type Artificer struct {
	options       *core.PipelineOptions
	dockerOptions *Options
	logger        *util.LogEntry
	store         core.Store
}

// NewArtificer returns an Artificer
func NewArtificer(options *core.PipelineOptions, dockerOptions *Options) *Artificer {
	logger := util.RootLogger().WithField("Logger", "Artificer")

	var store core.Store
	if options.ShouldStoreS3 {
		if options.GlobalOptions.LocalFileStore != "" {
			store = core.NewFileStore(options, options.GlobalOptions.LocalFileStore)
			logger.Debug("Activating local-file-store")
		} else {
			logger.Debug("Activating s3-store")
			store = core.NewS3Store(options.AWSOptions)
		}
	} else if options.ShouldStoreOCI {
		logger.Debug("Activating oci-store")
		store = core.NewObjectStore(options.OCIOptions)
	}

	return &Artificer{
		options:       options,
		dockerOptions: dockerOptions,
		logger:        logger,
		store:         store,
	}
}

// Collect an artifact from the container, if it doesn't have any files in
// the tarball return util.ErrEmptyTarball
func (a *Artificer) Collect(ctx context.Context, artifact *core.Artifact) (*core.Artifact, error) {
	client, _ := NewOfficialDockerClient(a.dockerOptions)

	if err := os.MkdirAll(filepath.Dir(artifact.HostPath), 0755); err != nil {
		return nil, err
	}

	outputFile, err := os.Create(artifact.HostTarPath)
	defer outputFile.Close()
	if err != nil {
		return nil, err
	}

	dfc := NewDockerFileCollector(client, artifact.ContainerID)
	archive, err := dfc.Collect(ctx, artifact.GuestPath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()

	// all reads from the archive are matched with correspnding writes to outputFile
	archive.Tee(outputFile)

	err = <-archive.Multi(filepath.Base(artifact.GuestPath), artifact.HostPath, maxArtifactSize)
	if err != nil {
		return nil, err
	}
	return artifact, nil
}

// Upload an artifact
func (a *Artificer) Upload(artifact *core.Artifact) error {
	return a.store.StoreFromFile(&core.StoreFromFileArgs{
		Path:        artifact.HostTarPath,
		Key:         artifact.RemotePath(),
		ContentType: artifact.ContentType,
		MaxTries:    3,
		Meta:        artifact.Meta,
	})
}

// DockerFileCollector impl of FileCollector
type DockerFileCollector struct {
	client      *OfficialDockerClient
	containerID string
	logger      *util.LogEntry
}

// NewDockerFileCollector constructor
func NewDockerFileCollector(client *OfficialDockerClient, containerID string) *DockerFileCollector {
	return &DockerFileCollector{
		client:      client,
		containerID: containerID,
		logger:      util.RootLogger().WithField("Logger", "DockerFileCollector"),
	}
}

// Collect grabs a path and returns an Archive containing the stream.
// The caller must call Close() on the returned Archive after it has finished with it.
func (fc *DockerFileCollector) Collect(ctx context.Context, path string) (*util.Archive, error) {
	reader, _, err := fc.client.CopyFromContainer(ctx, fc.containerID, path)
	if err != nil {
		// Ideally we would return an ErrEmptyTarball error if the API call failed with "Could not find the file",
		// which means the path being downloaded does not exist, and return err otherwise.
		// This is because some callers want to ignore ErrEmptyTarball errors.
		// However CopyFromContainer throws away the underlying error so we can't do this.
		// We therefore convert all errors into an ErrEmptyTarball error.
		return nil, util.ErrEmptyTarball
	}
	return util.NewArchive(reader, func() { reader.Close() }), nil
}
