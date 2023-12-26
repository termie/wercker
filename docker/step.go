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

package dockerlocal

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

func NewStep(config *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (core.Step, error) {
	// NOTE(termie) Special case steps are special
	if config.ID == "internal/docker-push" {
		return NewDockerPushStep(config, options, dockerOptions)
	}
	if config.ID == "internal/docker-scratch-push" {
		return NewDockerScratchPushStep(config, options, dockerOptions)
	}
	if config.ID == "internal/docker-build" {
		return NewDockerBuildStep(config, options, dockerOptions)
	}
	if config.ID == "internal/store-container" {
		return NewStoreContainerStep(config, options, dockerOptions)
	}
	if config.ID == "internal/publish-step" {
		return NewPublishStep(config, options, dockerOptions)
	}
	if config.ID == "internal/docker-run" {
		return NewDockerRunStep(config, options, dockerOptions)
	}
	if config.ID == "internal/docker-kill" {
		return NewDockerKillStep(config, options, dockerOptions)
	}

	if strings.HasPrefix(config.ID, "internal/") {
		if !options.EnableDevSteps {
			util.RootLogger().Warnln("Ignoring dev step:", config.ID)
			return nil, nil
		}
	}
	if options.EnableDevSteps {
		if config.ID == "internal/watch" {
			return NewWatchStep(config, options, dockerOptions)
		}
		if config.ID == "internal/shell" {
			return NewShellStep(config, options, dockerOptions)
		}
	}
	return NewDockerStep(config, options, dockerOptions)
}

// DockerStep is an external step that knows how to fetch artifacts
type DockerStep struct {
	*core.ExternalStep
	options       *core.PipelineOptions
	dockerOptions *Options
	logger        *util.LogEntry
}

// NewDockerStep ctor
func NewDockerStep(config *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*DockerStep, error) {
	base, err := core.NewStep(config, options)
	if err != nil {
		return nil, err
	}
	logger := util.RootLogger().WithFields(util.LogFields{
		"Logger": "DockerStep",
		"SafeID": base.SafeID(),
	})

	return &DockerStep{
		ExternalStep:  base,
		options:       options,
		dockerOptions: dockerOptions,
		logger:        logger,
	}, nil
}

// CollectFile gets an individual file from the container
func (s *DockerStep) CollectFile(containerID, path, name string, dst io.Writer) error {
	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		return err
	}

	pipeReader, pipeWriter := io.Pipe()

	opts := docker.DownloadFromContainerOptions{
		OutputStream: pipeWriter,
		Path:         filepath.Join(path, name),
	}

	errs := make(chan error)
	go func() {
		defer close(errs)
		errs <- util.UntarOne(name, dst, pipeReader)
	}()

	if err = client.DownloadFromContainer(containerID, opts); err != nil {
		s.logger.Debug("Probably expected error:", err)
		return util.ErrEmptyTarball
	}

	return <-errs
}

// CollectArtifact copies the artifacts associated with the Step.
func (s *DockerStep) CollectArtifact(ctx context.Context, containerID string) (*core.Artifact, error) {
	artificer := NewArtificer(s.options, s.dockerOptions)

	// Ensure we have the host directory

	artifact := &core.Artifact{
		ContainerID:   containerID,
		GuestPath:     s.ReportPath("artifacts"),
		HostTarPath:   s.options.HostPath(s.SafeID(), "output.tar"),
		HostPath:      s.options.HostPath(s.SafeID(), "output"),
		ApplicationID: s.options.ApplicationID,
		RunID:         s.options.RunID,
		RunStepID:     s.SafeID(),
		Bucket:        s.options.S3Bucket,
		ContentType:   "application/x-tar",
	}

	fullArtifact, err := artificer.Collect(ctx, artifact)
	if err != nil {
		if err == util.ErrEmptyTarball {
			return nil, nil
		}
		return nil, err
	}

	return fullArtifact, nil
}
