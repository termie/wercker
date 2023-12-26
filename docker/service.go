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
	"bytes"
	"fmt"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// Builder interface to create an image based on a service config
// kinda needed so we can break a bunch of circular dependencies with cmd
type Builder interface {
	Build(context.Context, *util.Environment, *core.BoxConfig) (*DockerBox, *docker.Image, error)
}

type nilBuilder struct{}

func (b *nilBuilder) Build(ctx context.Context, env *util.Environment, config *core.BoxConfig) (*DockerBox, *docker.Image, error) {
	return nil, nil, nil
}

func NewNilBuilder() *nilBuilder {
	return &nilBuilder{}
}

// InternalServiceBox wraps a box as a service
type InternalServiceBox struct {
	*DockerBox
	logger *util.LogEntry
}

// ExternalServiceBox wraps a box as a service
type ExternalServiceBox struct {
	*InternalServiceBox
	externalConfig *core.BoxConfig
	builder        Builder
}

// NewExternalServiceBox gives us an ExternalServiceBox from config
func NewExternalServiceBox(boxConfig *core.BoxConfig, options *core.PipelineOptions, dockerOptions *Options, builder Builder) (*ExternalServiceBox, error) {
	logger := util.RootLogger().WithField("Logger", "ExternalService")
	box := &DockerBox{options: options, dockerOptions: dockerOptions, config: boxConfig}
	return &ExternalServiceBox{
		InternalServiceBox: &InternalServiceBox{DockerBox: box, logger: logger},
		externalConfig:     boxConfig,
		builder:            builder,
	}, nil
}

// Fetch the image representation of an ExternalServiceBox
// this means running the ExternalServiceBox and comitting the image
func (s *ExternalServiceBox) Fetch(ctx context.Context, env *util.Environment) (*docker.Image, error) {
	originalShortName := s.externalConfig.ID
	box, image, err := s.builder.Build(ctx, env, s.externalConfig)
	if err != nil {
		return nil, err
	}
	box.image = image
	s.DockerBox = box
	s.ShortName = originalShortName
	return image, err
}

func NewServiceBox(config *core.BoxConfig, options *core.PipelineOptions, dockerOptions *Options, builder Builder) (core.ServiceBox, error) {
	if config.IsExternal() {
		return NewExternalServiceBox(config, options, dockerOptions, builder)
	}
	return NewInternalServiceBox(config, options, dockerOptions)
}

// NewServiceBox from a name and other references
func NewInternalServiceBox(boxConfig *core.BoxConfig, options *core.PipelineOptions, dockerOptions *Options) (*InternalServiceBox, error) {
	box, err := NewDockerBox(boxConfig, options, dockerOptions)
	logger := util.RootLogger().WithField("Logger", "Service")
	return &InternalServiceBox{DockerBox: box, logger: logger}, err
}

// TODO(mh) need to add to interface?
func (b *InternalServiceBox) getContainerName() string {
	name := b.config.Name
	if name == "" {
		name = b.Name
	}
	containerName := fmt.Sprintf("wercker-service-%s-%s", strings.Replace(name, "/", "-", -1), b.options.RunID)
	containerName = strings.Replace(containerName, ":", "_", -1)
	return strings.Replace(containerName, ":", "_", -1)
}

// Run executes the service
func (b *InternalServiceBox) Run(ctx context.Context, env *util.Environment, envVars []string) (*docker.Container, error) {
	e, err := core.EmitterFromContext(ctx)
	if err != nil {
		return nil, err
	}
	f := &util.Formatter{}

	client, err := NewDockerClient(b.dockerOptions)
	if err != nil {
		return nil, err
	}

	// Import the environment and command
	myEnv := dockerEnv(b.config.Env, env)
	myEnv = append(myEnv, envVars...)

	origEntrypoint := b.image.Config.Entrypoint
	origCmd := b.image.Config.Cmd
	cmdInfo := []string{}

	var entrypoint []string
	if b.entrypoint != "" {
		entrypoint, err = shlex.Split(b.entrypoint)
		if err != nil {
			return nil, err
		}
		cmdInfo = append(cmdInfo, entrypoint...)
	} else {
		cmdInfo = append(cmdInfo, origEntrypoint...)
	}

	var cmd []string
	if b.config.Cmd != "" {
		cmd, err = shlex.Split(b.config.Cmd)
		if err != nil {
			return nil, err
		}
		cmdInfo = append(cmdInfo, cmd...)
	} else {
		cmdInfo = append(cmdInfo, origCmd...)
	}

	binds := make([]string, 0)

	if b.options.EnableVolumes {
		vols := util.SplitSpaceOrComma(b.config.Volumes)
		var interpolatedVols []string
		for _, vol := range vols {
			if strings.Contains(vol, ":") {
				pair := strings.SplitN(vol, ":", 2)
				interpolatedVols = append(interpolatedVols, env.Interpolate(pair[0]))
				interpolatedVols = append(interpolatedVols, env.Interpolate(pair[1]))
			} else {
				interpolatedVols = append(interpolatedVols, env.Interpolate(vol))
				interpolatedVols = append(interpolatedVols, env.Interpolate(vol))
			}
		}
		b.volumes = interpolatedVols
		for i := 0; i < len(b.volumes); i += 2 {
			binds = append(binds, fmt.Sprintf("%s:%s:rw", b.volumes[i], b.volumes[i+1]))
		}
	}

	portsToBind := []string{""}

	if b.options.ExposePorts {
		portsToBind = b.config.Ports
	}

	networkName, err := b.GetDockerNetworkName()
	if err != nil {
		return nil, err
	}

	hostConfig := &docker.HostConfig{
		DNS:          b.dockerOptions.DNS,
		PortBindings: portBindings(portsToBind),
		NetworkMode:  networkName,
	}

	if len(binds) > 0 {
		hostConfig.Binds = binds
	}

	conf := &docker.Config{
		Image:           b.Name,
		Cmd:             cmd,
		Env:             myEnv,
		ExposedPorts:    exposedPorts(b.config.Ports),
		NetworkDisabled: b.networkDisabled,
		DNS:             b.dockerOptions.DNS,
		Entrypoint:      entrypoint,
	}

	// TODO(termie): terrible hack
	// Get service count so we can divvy memory
	serviceCount := ctx.Value("ServiceCount").(int)
	if b.dockerOptions.Memory != 0 {
		mem := b.dockerOptions.Memory
		mem = int64(float64(mem) * 0.25 / float64(serviceCount))

		swap := b.dockerOptions.MemorySwap
		if swap == 0 {
			swap = 2 * mem
		}

		conf.Memory = mem
		conf.MemorySwap = swap
	}

	endpointConfig := &docker.EndpointConfig{
		Aliases: []string{b.GetServiceAlias()},
	}
	endpointConfigMap := make(map[string]*docker.EndpointConfig)
	endpointConfigMap[networkName] = endpointConfig

	container, err := client.CreateContainerWithRetries(
		docker.CreateContainerOptions{
			Name:       b.getContainerName(),
			Config:     conf,
			HostConfig: hostConfig,
			NetworkingConfig: &docker.NetworkingConfig{
				EndpointsConfig: endpointConfigMap,
			},
		})

	if err != nil {
		return nil, err
	}

	out := []string{}
	for _, part := range cmdInfo {
		if strings.Contains(part, " ") {
			out = append(out, fmt.Sprintf("%q", part))
		} else {
			out = append(out, part)
		}
	}
	if b.options.Verbose {
		b.logger.Println(f.Info(fmt.Sprintf("Starting service %s", b.ShortName), strings.Join(out, " ")))
	}

	client.StartContainer(container.ID, hostConfig)
	b.container = container

	go func() {
		status, err := client.WaitContainer(container.ID)
		if err != nil {
			b.logger.Errorln("Error waiting", err)
		}
		b.logger.Debugln("Service container finished with status code:", status, container.ID)

		if status != 0 {
			var errstream bytes.Buffer
			var outstream bytes.Buffer
			// recv := make(chan string)
			// outputStream := NewReceiver(recv)
			opts := docker.LogsOptions{
				Container:    container.ID,
				Stdout:       true,
				Stderr:       true,
				ErrorStream:  &errstream,
				OutputStream: &outstream,
				RawTerminal:  false,
			}
			err = client.Logs(opts)
			if err != nil {
				b.logger.Panicln(err)
			}
			e.Emit(core.Logs, &core.LogsArgs{
				Stream: fmt.Sprintf("%s-stdout", b.Name),
				Logs:   outstream.String(),
			})
			e.Emit(core.Logs, &core.LogsArgs{
				Stream: fmt.Sprintf("%s-stderr", b.Name),
				Logs:   errstream.String(),
			})
		}
	}()
	return container, nil
}

// GetServiceAlias returns service alias for the service.
func (b *InternalServiceBox) GetServiceAlias() string {
	name := b.config.Name
	if name == "" {
		name = b.ShortName
	}
	return name
}
