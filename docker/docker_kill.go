/* Copyright © 2018, Oracle and/or its affiliates. All rights reserved. */
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
	"io"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/pborman/uuid"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// DockerKillStep needs to implemenet IStep
type DockerKillStep struct {
	*core.BaseStep
	logger        *util.LogEntry
	options       *core.PipelineOptions
	dockerOptions *Options
	data          map[string]string
	containerName string
}

// NewDockerKillStep is a special step for killing and removing container.
func NewDockerKillStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*DockerKillStep, error) {
	name := "docker-kill"
	displayName := "docker kill"
	if stepConfig.Name == "" {
		err := fmt.Errorf("\"name\" is a required field")
		return nil, err
	}
	// Add a random number to the name to prevent collisions on disk
	stepSafeID := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())
	baseStep := core.NewBaseStep(core.BaseStepOptions{
		DisplayName: displayName,
		Env:         &util.Environment{},
		ID:          name,
		Name:        name,
		Owner:       "wercker",
		SafeID:      stepSafeID,
		Version:     util.Version(),
	})
	return &DockerKillStep{
		BaseStep:      baseStep,
		data:          stepConfig.Data,
		logger:        util.RootLogger().WithField("Logger", "DockerKillStep"),
		options:       options,
		dockerOptions: dockerOptions,
		containerName: stepConfig.Name,
	}, nil
}

// InitEnv parses our data into our config
func (s *DockerKillStep) InitEnv(ctx context.Context, env *util.Environment) error {
	return nil
}

// Fetch NOP
func (s *DockerKillStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// Execute kills container
func (s *DockerKillStep) Execute(ctx context.Context, sess *core.Session) (int, error) {
	// TODO(termie): could probably re-use the tansport's client
	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		return 1, err
	}
	containerToKill := s.options.RunID + s.containerName
	killOpts := docker.KillContainerOptions{
		ID: containerToKill,
	}
	s.logger.Debugln("Kill container:", containerToKill)
	err = client.KillContainer(killOpts)
	if err != nil {
		s.logger.Errorln("Failed to kill container", err)
		return -1, err
	}
	removeContainerOpts := docker.RemoveContainerOptions{
		ID: containerToKill,
	}
	s.logger.Debugln("Remove container:", containerToKill)
	err = client.RemoveContainer(removeContainerOpts)
	if err != nil {
		s.logger.Errorln("Failed to remove container", err)
		return -1, err
	}
	s.logger.WithField("containerName", s.containerName).Debug("Docker-kill completed")
	return 0, nil
}

// CollectFile NOP
func (s *DockerKillStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact NOP
func (s *DockerKillStep) CollectArtifact(context.Context, string) (*core.Artifact, error) {
	return nil, nil
}

// ReportPath NOP
func (s *DockerKillStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = FALSE
func (s *DockerKillStep) ShouldSyncEnv() bool {
	return false
}
