//   Copyright © 2017,2018, Oracle and/or its affiliates.  All rights reserved.
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
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/fsouza/go-dockerclient"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/steps/cmd"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// PublishStep needs to implemenet IStep
type PublishStep struct {
	*core.BaseStep
	user            string
	private         bool
	endpoint        string
	authToken       string
	pathInContainer string
	data            map[string]string
	logger          *util.LogEntry
	options         *core.PipelineOptions
	dockerOptions   *Options
}

// NewPublishStep is a special step for doing docker pushes
func NewPublishStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*PublishStep, error) {
	name := "publish-step"
	displayName := "publish-step"
	if stepConfig.Name != "" {
		displayName = stepConfig.Name
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

	return &PublishStep{
		BaseStep:        baseStep,
		options:         options,
		dockerOptions:   dockerOptions,
		data:            stepConfig.Data,
		authToken:       options.AuthToken,
		endpoint:        options.StepRegistryURL,
		pathInContainer: pathInContainer(""),
		logger:          util.RootLogger().WithField("Logger", "PublishStep"),
	}, nil
}

// InitEnv parses our data into our config
func (s *PublishStep) InitEnv(ctx context.Context, env *util.Environment) error {
	if owner, ok := s.data["owner"]; ok {
		s.user = env.Interpolate(owner)
	}
	if endpoint, ok := s.data["endpoint"]; ok {
		s.endpoint = env.Interpolate(endpoint)
	}
	if authToken, ok := s.data["auth-token"]; ok {
		s.authToken = env.Interpolate(authToken)
	}
	if path, ok := s.data["path"]; ok {
		s.pathInContainer = pathInContainer(path)
	}
	if privateProp, ok := s.data["private"]; ok {
		private, err := strconv.ParseBool(privateProp)
		if err == nil {
			s.private = private
		}
	}
	return nil
}

// Fetch NOP
func (s *PublishStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// Execute a shell and give it to the user
func (s *PublishStep) Execute(ctx context.Context, sess *core.Session) (int, error) {
	dt := sess.Transport().(*DockerTransport)
	containerID := dt.containerID

	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		s.logger.Error("Failed to create docker client", err)
		return -1, err
	}

	runDir := filepath.Join("/var/lib/wercker/", s.options.RunID)

	stepDir, err := ioutil.TempDir(runDir, "step-")
	if err != nil {
		s.logger.Error("Failed to create temp dir for the step", err)
		return -1, err
	}

	err = getFilesFromContainer(client, containerID, runDir, stepDir, s.pathInContainer)
	if err != nil {
		s.logger.Error("Failed to retrieve step files from container", err)
		return -1, err
	}

	opts := &cmd.PublishStepOptions{
		Endpoint:  s.endpoint,
		AuthToken: s.authToken,
		Owner:     s.user,
		Private:   s.private,
		StepDir:   stepDir,
		TempDir:   runDir,
	}
	if err = cmd.PublishStep(opts); err != nil {
		s.logger.Error("Unable to publish step to the registry", err)
		return -1, err
	}

	return 0, nil
}

func getFilesFromContainer(client *DockerClient, containerID, runDir, dst, src string) error {
	sourceTar, err := ioutil.TempFile(runDir, "step-")
	if err != nil {
		return errors.Wrap(err, "failed to create tmp file for the archive")
	}
	defer func() {
		sourceTar.Close()
		os.Remove(sourceTar.Name())
	}()

	downloadOpts := docker.DownloadFromContainerOptions{
		OutputStream: sourceTar,
		Path:         src,
	}
	err = client.DownloadFromContainer(containerID, downloadOpts)
	if err != nil {
		return errors.Wrap(err, "failed to download files from container")
	}

	sourceTar.Seek(0, io.SeekStart)

	err = util.Untar(dst, sourceTar)
	if err != nil {
		return errors.Wrap(err, "failed to untar files from container")
	}

	return nil
}

func pathInContainer(path string) string {
	return filepath.Join("/pipeline/source", path) + "/."
}

// CollectFile NOP
func (s *PublishStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact NOP
func (s *PublishStep) CollectArtifact(context.Context, string) (*core.Artifact, error) {
	return nil, nil
}

// ReportPath getter
func (s *PublishStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = TRUE
func (s *PublishStep) ShouldSyncEnv() bool {
	return true
}
