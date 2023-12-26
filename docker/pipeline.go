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

	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// DockerPipeline is our docker PipelineConfigurer and Pipeline impl
type DockerPipeline struct {
	*core.BasePipeline
	options       *core.PipelineOptions
	dockerOptions *Options
}

func NewDockerPipeline(name string, config *core.Config, options *core.PipelineOptions, dockerOptions *Options, builder Builder) (*DockerPipeline, error) {
	// decide which configs to use for each thing
	// TODO(termie): this code is not all specific to docker and should be made
	//               into something shared
	pipelineName := options.Pipeline
	pipelineConfig, ok := config.PipelinesMap[pipelineName]
	if !ok {
		return nil, fmt.Errorf("No pipeline named %s", pipelineName)
	}

	if pipelineConfig == nil {
		return nil, fmt.Errorf("Pipeline %s is empty", pipelineName)
	}

	// Select this pipeline's config or the global config
	rawBoxConfig := pipelineConfig.Box
	if rawBoxConfig == nil {
		rawBoxConfig = config.Box
	}
	// no pipeline config or global config
	if rawBoxConfig == nil {
		return nil, fmt.Errorf("No box definition found")
	}
	boxConfig := rawBoxConfig.BoxConfig

	// Select this pipeline's service or the global config
	servicesConfig := pipelineConfig.Services
	if servicesConfig == nil {
		servicesConfig = config.Services
	}

	stepsConfig := pipelineConfig.Steps
	if options.DeployTarget != "" {
		sectionSteps, ok := pipelineConfig.StepsMap[options.DeployTarget]
		if ok {
			stepsConfig = sectionSteps
		}
	}

	afterStepsConfig := pipelineConfig.AfterSteps

	box, err := NewDockerBox(boxConfig, options, dockerOptions)
	if err != nil {
		return nil, err
	}

	var services []core.ServiceBox
	for _, serviceConfig := range servicesConfig {
		service, err := NewServiceBox(serviceConfig.BoxConfig, options, dockerOptions, builder)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	initStep, err := core.NewWerckerInitStep(options)
	if err != nil {
		return nil, err
	}

	steps := []core.Step{initStep}
	for _, stepConfig := range stepsConfig {
		step, err := NewStep(stepConfig.StepConfig, options, dockerOptions)
		if err != nil {
			return nil, err
		}
		if step != nil {
			// we can return a nil step if it's internal and EnableDevSteps is
			// false
			steps = append(steps, step)
		}
	}

	var afterSteps []core.Step
	for _, stepConfig := range afterStepsConfig {
		step, err := NewStep(stepConfig.StepConfig, options, dockerOptions)
		if err != nil {
			return nil, err
		}
		if step != nil {
			// we can return a nil step if it's internal and EnableDevSteps is
			// false
			afterSteps = append(afterSteps, step)
		}
	}
	// if we found some valid after steps, prepend init
	if len(afterSteps) > 0 {
		initStep, err := core.NewWerckerInitStep(options)
		if err != nil {
			return nil, err
		}

		afterSteps = append([]core.Step{initStep}, afterSteps...)
	}

	logger := util.RootLogger().WithField("Logger", "Pipeline")
	base := core.NewBasePipeline(core.BasePipelineOptions{
		Options:    options,
		Config:     pipelineConfig.PipelineConfig,
		Env:        util.NewEnvironment(),
		Box:        box,
		Services:   services,
		Steps:      steps,
		AfterSteps: afterSteps,
		Logger:     logger,
	})
	return &DockerPipeline{BasePipeline: base, options: options, dockerOptions: dockerOptions}, nil
}

// CollectCache extracts the cache from the container to the cachedir
func (p *DockerPipeline) CollectCache(ctx context.Context, containerID string) error {
	client, err := NewOfficialDockerClient(p.dockerOptions)
	if err != nil {
		return err
	}
	dfc := NewDockerFileCollector(client, containerID)

	archive, err := dfc.Collect(ctx, p.options.GuestPath("cache"))
	if err != nil {
		if err == util.ErrEmptyTarball {
			return nil
		}
		return err
	}
	defer archive.Close()

	err = <-archive.Multi("cache", p.options.CachePath(), 1024*1024*1000)
	if err != nil {
		if err == util.ErrEmptyTarball {
			return nil
		}
		return err
	}
	return nil
}
