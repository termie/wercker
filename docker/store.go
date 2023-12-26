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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/fsouza/go-dockerclient"
	"github.com/mreiferson/go-snappystream"
	"github.com/pborman/uuid"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// StoreContainerStep stores the container that was built
type StoreContainerStep struct {
	*core.BaseStep
	options       *core.PipelineOptions
	dockerOptions *Options
	data          map[string]string
	logger        *util.LogEntry
	artifact      *core.Artifact
}

// NewStoreContainerStep constructor
func NewStoreContainerStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*StoreContainerStep, error) {
	name := "store-container"
	displayName := "store container"
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

	return &StoreContainerStep{
		BaseStep:      baseStep,
		options:       options,
		dockerOptions: dockerOptions,
		data:          stepConfig.Data,
		logger:        util.RootLogger().WithField("Logger", "StoreContainerStep"),
	}, nil

}

// InitEnv preps our env
func (s *StoreContainerStep) InitEnv(ctx context.Context, env *util.Environment) error {
	// NOP
	return nil
}

// Fetch NOP
func (s *StoreContainerStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// DockerRepo calculates our repo name
func (s *StoreContainerStep) DockerRepo() string {
	if s.options.Repository != "" {
		return s.options.Repository
	}
	return fmt.Sprintf("run-%s", s.options.RunID)
}

// DockerTag calculates our tag
func (s *StoreContainerStep) DockerTag() string {
	if s.options.Tag != "" {
		return s.options.Tag
	}
	return "latest"
}

// DockerMessage calculates our message
func (s *StoreContainerStep) DockerMessage() string {
	message := s.options.Message
	if message == "" {
		message = fmt.Sprintf("Run %s", s.options.RunID)
	}
	return message
}

// Execute does the actual export and upload of the container
func (s *StoreContainerStep) Execute(ctx context.Context, sess *core.Session) (int, error) {
	e, err := core.EmitterFromContext(ctx)
	if err != nil {
		return -1, err
	}
	// TODO(termie): could probably re-use the tansport's client
	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		return -1, err
	}
	// This is clearly only relevant to docker so we're going to dig into the
	// transport internals a little bit to get the container ID
	dt := sess.Transport().(*DockerTransport)
	containerID := dt.containerID

	repoName := s.DockerRepo()
	tag := s.DockerTag()
	message := s.DockerMessage()

	commitOpts := docker.CommitContainerOptions{
		Container:  containerID,
		Repository: repoName,
		Tag:        tag,
		Author:     "wercker",
		Message:    message,
	}
	s.logger.Debugln("Commit container:", containerID)
	i, err := client.CommitContainer(commitOpts)
	if err != nil {
		return -1, err
	}
	s.logger.WithField("Image", i).Debug("Commit completed")

	e.Emit(core.Logs, &core.LogsArgs{
		Logs: "Exporting container\n",
	})

	file, err := ioutil.TempFile(s.options.BuildPath(), "export-image-")
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to create temporary file")
		return -1, err
	}

	hash := sha256.New()
	w := snappystream.NewWriter(io.MultiWriter(file, hash))

	exportImageOptions := docker.ExportImageOptions{
		Name:         repoName,
		OutputStream: w,
	}
	err = client.ExportImage(exportImageOptions)
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to export image")
		return -1, err
	}

	// Copy is done now, so close temporary file and set the calculatedHash
	file.Close()

	calculatedHash := hex.EncodeToString(hash.Sum(nil))

	s.logger.WithFields(util.LogFields{
		"SHA256":            calculatedHash,
		"TemporaryLocation": file.Name(),
	}).Println("Export image successful")

	key := core.GenerateBaseKey(s.options)
	key = fmt.Sprintf("%s/%s", key, "docker.tar.sz")

	s.artifact = &core.Artifact{
		HostPath:    file.Name(),
		Key:         key,
		Bucket:      s.options.S3Bucket,
		ContentType: "application/x-snappy-framed",
		Meta: map[string]*string{
			"Sha256": &calculatedHash,
		},
	}

	return 0, nil
}

// CollectFile NOP
func (s *StoreContainerStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact return an artifact pointing at the exported thing we made
func (s *StoreContainerStep) CollectArtifact(context.Context, string) (*core.Artifact, error) {
	return s.artifact, nil
}

// ReportPath NOP
func (s *StoreContainerStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = TRUE
func (s *StoreContainerStep) ShouldSyncEnv() bool {
	return true
}
