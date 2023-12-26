//   Copyright 2016 Wercker Holding BV
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

package cmd

import (
	"flag"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/fsouza/go-dockerclient"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/docker"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
	cli "gopkg.in/urfave/cli.v1"
)

type DockerBuilder struct {
	options       *core.PipelineOptions
	dockerOptions *dockerlocal.Options
}

func NewDockerBuilder(options *core.PipelineOptions, dockerOptions *dockerlocal.Options) *DockerBuilder {
	return &DockerBuilder{
		options:       options,
		dockerOptions: dockerOptions,
	}
}

func (b *DockerBuilder) configURL(config *core.BoxConfig) (*url.URL, error) {
	return url.Parse(config.URL)
}

func (b *DockerBuilder) getOptions(env *util.Environment, config *core.BoxConfig) (*core.PipelineOptions, error) {
	c, err := b.configURL(config)
	if err != nil {
		return nil, err
	}
	servicePath := filepath.Join(c.Host, c.Path)
	if !filepath.IsAbs(servicePath) {
		servicePath, err = filepath.Abs(
			filepath.Join(b.options.ProjectPath, servicePath))
		if err != nil {
			return nil, err
		}
	}

	flagSet := func(name string, flags []cli.Flag) *flag.FlagSet {
		set := flag.NewFlagSet(name, flag.ContinueOnError)

		for _, f := range flags {
			f.Apply(set)
		}
		return set
	}

	set := flagSet("runservice", FlagsFor(PipelineFlagSet, WerckerInternalFlagSet))
	args := []string{
		servicePath,
	}
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	ctx := cli.NewContext(nil, set, nil)
	settings := util.NewCLISettings(ctx)
	newOptions, err := core.NewBuildOptions(settings, env)
	if err != nil {
		return nil, err
	}

	newOptions.GlobalOptions = b.options.GlobalOptions
	newOptions.ShouldCommit = true
	newOptions.PublishPorts = b.options.PublishPorts
	newOptions.Pipeline = c.Fragment
	return newOptions, nil
}

// Build the image and commit it so we can use it as a service
func (b *DockerBuilder) Build(ctx context.Context, env *util.Environment, config *core.BoxConfig) (*dockerlocal.DockerBox, *docker.Image, error) {
	newOptions, err := b.getOptions(env, config)

	if err != nil {
		return nil, nil, err
	}

	newDockerOptions := *b.dockerOptions

	shared, err := cmdBuild(ctx, newOptions, &newDockerOptions)
	if err != nil {
		return nil, nil, err
	}
	// TODO(termie): this causes the ID to get overwritten but
	//               we want the shortname that the user specified as an ID
	//               so we probably wnat to make a copy or something here
	bc := config
	bc.ID = fmt.Sprintf("%s:%s", shared.pipeline.DockerRepo(),
		shared.pipeline.DockerTag())

	box, err := dockerlocal.NewDockerBox(bc, b.options, &newDockerOptions)
	if err != nil {
		return nil, nil, err
	}

	client, err := dockerlocal.NewDockerClient(&newDockerOptions)
	image, err := client.InspectImage(box.Name)
	if err != nil {
		return nil, nil, err
	}
	return box, image, nil
}
