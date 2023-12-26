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
	"fmt"
	"os"

	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

type DockerBuild struct {
	*DockerPipeline
}

func NewDockerBuild(name string, config *core.Config, options *core.PipelineOptions, dockerOptions *Options, builder Builder) (*DockerBuild, error) {
	base, err := NewDockerPipeline(name, config, options, dockerOptions, builder)
	if err != nil {
		return nil, err
	}
	return &DockerBuild{base}, nil
}

// LocalSymlink makes an easy to use symlink to find the latest run
func (b *DockerBuild) LocalSymlink() {
	_ = os.RemoveAll(b.options.WorkingPath("latest"))
	_ = os.Symlink(b.options.HostPath(), b.options.WorkingPath("latest"))
}

// InitEnv sets up the internal state of the environment for the build
func (b *DockerBuild) InitEnv(ctx context.Context, hostEnv *util.Environment) {
	env := b.Env()

	a := [][]string{
		[]string{"BUILD", "true"},
		[]string{"CI", "true"},
		[]string{"WERCKER_RUN_ID", b.options.RunID},
		[]string{"WERCKER_RUN_URL", b.options.WorkflowURL()},
		[]string{"WERCKER_GIT_DOMAIN", b.options.GitDomain},
		[]string{"WERCKER_GIT_OWNER", b.options.GitOwner},
		[]string{"WERCKER_GIT_REPOSITORY", b.options.GitRepository},
		[]string{"WERCKER_GIT_BRANCH", b.options.GitBranch},
		[]string{"WERCKER_GIT_TAG", b.options.GitTag},
		[]string{"WERCKER_GIT_COMMIT", b.options.GitCommit},

		// Legacy env vars
		[]string{"WERCKER_BUILD_ID", b.options.RunID},
		[]string{"WERCKER_BUILD_URL", b.options.WorkflowURL()},
	}

	env.Update(b.CommonEnv())
	env.Update(a)
	env.Update(hostEnv.GetMirror())
	env.Update(hostEnv.GetPassthru().Ordered())
	env.Hidden.Update(hostEnv.GetHiddenPassthru().Ordered())
}

// DockerRepo calculates our repo name
func (b *DockerBuild) DockerRepo() string {
	if b.options.Repository != "" {
		return b.options.Repository
	}
	return fmt.Sprintf("run-%s", b.options.RunID)
}

// DockerTag calculates our tag
func (b *DockerBuild) DockerTag() string {
	if b.options.Tag != "" {
		return b.options.Tag
	}
	return "latest"
}

// DockerMessage calculates our message
func (b *DockerBuild) DockerMessage() string {
	message := b.options.Message
	if message == "" {
		message = fmt.Sprintf("Run %s", b.options.RunID)
	}
	return message
}

// CollectArtifact copies the artifacts associated with the Build.
func (b *DockerBuild) CollectArtifact(ctx context.Context, containerID string) (*core.Artifact, error) {
	artificer := NewArtificer(b.options, b.dockerOptions)

	// Ensure we have the host directory

	artifact := &core.Artifact{
		ContainerID:   containerID,
		GuestPath:     b.options.GuestPath("output"),
		HostPath:      b.options.HostPath("output"),
		HostTarPath:   b.options.HostPath("output.tar"),
		ApplicationID: b.options.ApplicationID,
		RunID:         b.options.RunID,
		Bucket:        b.options.S3Bucket,
		ContentType:   "application/x-tar",
	}

	sourceArtifact := &core.Artifact{
		ContainerID:   containerID,
		GuestPath:     b.options.BasePath(),
		HostPath:      b.options.HostPath("output"),
		HostTarPath:   b.options.HostPath("output.tar"),
		ApplicationID: b.options.ApplicationID,
		RunID:         b.options.RunID,
		Bucket:        b.options.S3Bucket,
		ContentType:   "application/x-tar",
	}

	// Get the output dir, if it is empty grab the source dir.
	fullArtifact, err := artificer.Collect(ctx, artifact)
	if err != nil {
		if err == util.ErrEmptyTarball {
			fullArtifact, err = artificer.Collect(ctx, sourceArtifact)
			if err != nil {
				return nil, err
			}
			return fullArtifact, nil
		}
		return nil, err
	}

	return fullArtifact, nil
}
