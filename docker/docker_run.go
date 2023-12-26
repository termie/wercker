//   Copyright © 2018, Oracle and/or its affiliates.  All rights reserved.
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
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	"github.com/pborman/uuid"
	"github.com/wercker/wercker/auth"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

type DockerRunStep struct {
	*core.BaseStep
	options               *core.PipelineOptions
	dockerOptions         *Options
	data                  map[string]string
	env                   []string
	logger                *util.LogEntry
	Cmd                   []string
	EntryPoint            []string
	WorkingDir            string
	PortBindings          map[docker.Port][]docker.PortBinding
	ExposedPorts          map[docker.Port]struct{}
	User                  string
	ContainerName         string
	OriginalContainerName string
	Image                 string
	ContainerID           string
	auth                  dockerauth.CheckAccessOptions `yaml:",inline"`
}

type BoxDockerRun struct {
	*DockerBox
}

// NewDockerRunStep is a special step for doing docker runs. "image" is the required property for this step. It first checks if the input
// "image" is build by docker-build and resided in docker-deaon. If not, then it uses the DockerBox logic to fetch the image and starts
// a new container on that image. The new container is started on the Box network and cleaned up at the end of pipeline.
func NewDockerRunStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*DockerRunStep, error) {
	name := "docker-run"
	displayName := "docker run"
	if stepConfig.Name == "" {
		err := fmt.Errorf("\"name\" is a required field")
		return nil, err
	}
	originalContainerName := stepConfig.Name

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

	return &DockerRunStep{
		BaseStep:              baseStep,
		data:                  stepConfig.Data,
		logger:                util.RootLogger().WithField("Logger", "DockerRunStep"),
		options:               options,
		dockerOptions:         dockerOptions,
		OriginalContainerName: originalContainerName,
	}, nil
}

// InitEnv parses our data into our config
func (s *DockerRunStep) InitEnv(ctx context.Context, env *util.Environment) error {
	err := s.configure(env)
	return err
}

func (s *DockerRunStep) configure(env *util.Environment) error {
	if s.options.ExposePorts {
		if ports, ok := s.data["ports"]; ok {
			parts, err := shlex.Split(ports)
			if err != nil {
				return err
			}
			s.PortBindings = portBindings(parts)
			s.ExposedPorts = exposedPorts(parts)

		}
	}

	if workingDir, ok := s.data["working-dir"]; ok {
		s.WorkingDir = env.Interpolate(workingDir)
	}

	image, err := getCorrectImageName(env, s)
	if err != nil {
		return err
	}
	s.Image = image

	s.OriginalContainerName = env.Interpolate(s.OriginalContainerName)
	s.ContainerName = s.options.RunID + s.OriginalContainerName

	if cmd, ok := s.data["cmd"]; ok {
		parts, err := shlex.Split(cmd)
		if err != nil {
			return err
		}
		s.Cmd = parts
	}

	if entryPoint, ok := s.data["entrypoint"]; ok {
		parts, err := shlex.Split(entryPoint)
		if err != nil {
			return err
		}
		s.EntryPoint = parts
	}

	if envi, ok := s.data["env"]; ok {
		parsedEnv, err := shlex.Split(envi)

		if err != nil {
			return err
		}
		interpolatedEnv := make([]string, len(parsedEnv))
		for i, envVar := range parsedEnv {
			interpolatedEnv[i] = env.Interpolate(envVar)
		}
		s.env = interpolatedEnv
	}

	if user, ok := s.data["user"]; ok {
		s.User = env.Interpolate(user)
	}

	opts := dockerauth.CheckAccessOptions{}
	if username, ok := s.data["username"]; ok {
		opts.Username = env.Interpolate(username)
	}
	if password, ok := s.data["password"]; ok {
		opts.Password = env.Interpolate(password)
	}
	if registry, ok := s.data["registry"]; ok {
		opts.Registry = dockerauth.NormalizeRegistry(env.Interpolate(registry))
	}
	s.auth = opts
	return nil
}

// The most important point in this function is that the required image is not pulled at first but the pipeline run-id is appended to
// the input image name and its existence is checked locally. If that fails only then the input image is pulled from its respective registry.
// This is done like this because docker-run is usually run in integration with docker-build. And, docker-build image is present in the
// docker-deamon by <BuildId><image> name.
func getCorrectImageName(env *util.Environment, s *DockerRunStep) (string, error) {
	i := env.Interpolate(s.data["image"])
	if i == "" {
		err := fmt.Errorf("\"image\" is a required field")
		return "", err
	}

	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		return "", err
	}

	// Determine whether the specified image-name corresponds to an image created by a prior docker-build step (in which case image-name will be pre-pended by the build id).
	localImageName := s.options.RunID + i
	_, err = client.InspectImage(localImageName)
	if err != nil {
		return i, nil
	}
	return localImageName, nil

}

// NewBoxDockerRun gives a wrapper for a box.
func NewBoxDockerRun(boxConfig *core.BoxConfig, options *core.PipelineOptions, dockerOptions *Options) (*BoxDockerRun, error) {
	box, err := NewDockerBox(boxConfig, options, dockerOptions)
	if err != nil {
		return nil, err
	}
	return &BoxDockerRun{DockerBox: box}, err
}

// Fetch NOP
func (s *DockerRunStep) Fetch() (string, error) {
	return "", nil
}

// Execute creates the container and starts the container.
func (s *DockerRunStep) Execute(ctx context.Context, sess *core.Session) (int, error) {

	boxConfig := &core.BoxConfig{
		ID:   s.Image,
		Auth: s.auth,
	}
	dockerRunDockerBox, err := NewBoxDockerRun(boxConfig, s.options, s.dockerOptions)
	if err != nil {
		s.logger.Errorln("Error in creating a box from boxConfig ", boxConfig)
		return 1, err
	}

	// Pull the image from the registry unless it was created by a prior docker-build step.
	if !strings.HasPrefix(s.Image, s.options.RunID) {
		_, err = dockerRunDockerBox.Fetch(ctx, s.Env())
		if err != nil {
			return 1, err
		}
	}

	networkName, err := dockerRunDockerBox.GetDockerNetworkName()
	if err != nil {
		return 1, err
	}

	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		return 1, err
	}

	conf := &docker.Config{
		Image:        s.Image,
		Cmd:          s.Cmd,
		Env:          s.env,
		ExposedPorts: s.ExposedPorts,
		Entrypoint:   s.EntryPoint,
		DNS:          s.dockerOptions.DNS,
		WorkingDir:   s.WorkingDir,
	}

	hostconfig := &docker.HostConfig{
		DNS:          s.dockerOptions.DNS,
		PortBindings: s.PortBindings,
		NetworkMode:  networkName,
	}

	endpointConfig := &docker.EndpointConfig{
		Aliases: []string{s.OriginalContainerName},
	}
	endpointConfigMap := make(map[string]*docker.EndpointConfig)
	endpointConfigMap[networkName] = endpointConfig

	networkingconfig := &docker.NetworkingConfig{
		EndpointsConfig: endpointConfigMap,
	}

	container, err := s.createContainer(client, conf, hostconfig, networkingconfig)
	if err != nil {
		s.logger.Errorln("Error in creating container name : ", s.ContainerName)
		return 1, err
	}
	s.logger.Infoln("Container is created with container id : ", container.ID)

	s.ContainerID = container.ID

	err = s.startContainer(client, hostconfig)
	if err != nil {
		s.logger.Errorln("Error in starting container name : ", s.ContainerName)
		return 1, err
	}
	s.logger.Infoln("Container is successfully started name : ", s.ContainerName)

	return 0, nil
}

func (s *DockerRunStep) createContainer(client *DockerClient, conf *docker.Config, hostconfig *docker.HostConfig, networkingConfig *docker.NetworkingConfig) (*docker.Container, error) {
	container, err := client.CreateContainerWithRetries(
		docker.CreateContainerOptions{
			Name:             s.ContainerName,
			Config:           conf,
			HostConfig:       hostconfig,
			NetworkingConfig: networkingConfig,
		})
	return container, err
}

func (s *DockerRunStep) startContainer(client *DockerClient, hostConfig *docker.HostConfig) error {
	err := client.StartContainer(s.ContainerName, hostConfig)
	return err
}

// CollectFile NOP
func (s *DockerRunStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact NOP
func (s *DockerRunStep) CollectArtifact(context.Context, string) (*core.Artifact, error) {
	return nil, nil
}

// ReportPath NOP
func (s *DockerRunStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = TRUE
func (s *DockerRunStep) ShouldSyncEnv() bool {
	// If disable-sync is set, only sync if it is not true
	if disableSync, ok := s.data["disable-sync"]; ok {
		return disableSync != "true"
	}
	return true
}

func (s *DockerRunStep) Clean() {
	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		s.logger.Errorln("Error in creating docker client")
		return
	}

	container, _ := client.InspectContainer(s.ContainerID)
	if container != nil {

		err = client.StopContainer(s.ContainerID, 1)
		if err != nil {
			s.logger.Errorln("Error in stopping the container with id : ", s.ContainerID)
		}

		opts := docker.RemoveContainerOptions{
			ID:            s.ContainerID,
			RemoveVolumes: true,
			Force:         true,
		}
		err = client.RemoveContainer(opts)
		if err != nil {
			s.logger.Errorln("Error in deleting the container with id : ", s.ContainerID)
		}
	} else {
		s.logger.Debugln(fmt.Sprintf("Container %s, already removed", s.ContainerID))
	}
}
