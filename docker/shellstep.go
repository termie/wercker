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
	"io"

	"github.com/google/shlex"
	"github.com/pborman/uuid"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// ShellStep needs to implemenet IStep
type ShellStep struct {
	*core.BaseStep
	Code          string
	Cmd           []string
	data          map[string]string
	logger        *util.LogEntry
	env           *util.Environment
	options       *core.PipelineOptions
	dockerOptions *Options
}

// NewShellStep is a special step for doing docker pushes
func NewShellStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*ShellStep, error) {
	name := "shell"
	displayName := "shell"
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

	return &ShellStep{
		BaseStep:      baseStep,
		options:       options,
		dockerOptions: dockerOptions,
		data:          stepConfig.Data,
		logger:        util.RootLogger().WithField("Logger", "ShellStep"),
	}, nil
}

// InitEnv parses our data into our config
func (s *ShellStep) InitEnv(ctx context.Context, env *util.Environment) error {
	if code, ok := s.data["code"]; ok {
		s.Code = code
	}
	if cmd, ok := s.data["cmd"]; ok && cmd != "" {
		parts, err := shlex.Split(cmd)
		if err == nil {
			s.Cmd = parts
		} else {
			return fmt.Errorf("%s is an invalid value for cmd, parsing error: %s", cmd, err.Error())
		}
	} else {
		cmd, _ := shlex.Split(DefaultDockerCommand)
		s.Cmd = cmd
	}
	s.env = env
	return nil
}

// Fetch NOP
func (s *ShellStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// Execute a shell and give it to the user
func (s *ShellStep) Execute(ctx context.Context, sess *core.Session) (int, error) {
	// cheating to get containerID
	// TODO(termie): we should deal with this eventually
	dt := sess.Transport().(*DockerTransport)
	containerID := dt.containerID

	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		return -1, err
	}

	code := s.env.Export()
	code = append(code, "cd $WERCKER_SOURCE_DIR")
	code = append(code, s.Code)

	err = client.AttachInteractive(containerID, s.Cmd, code)
	if err != nil {
		return -1, err
	}
	return 0, nil
}

// CollectFile NOP
func (s *ShellStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact NOP
func (s *ShellStep) CollectArtifact(context.Context, string) (*core.Artifact, error) {
	return nil, nil
}

// ReportPath getter
func (s *ShellStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = TRUE
func (s *ShellStep) ShouldSyncEnv() bool {
	return true
}
