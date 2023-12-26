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
	"archive/tar"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/google/shlex"
	"github.com/pborman/uuid"
	"github.com/wercker/wercker/auth"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// DockerBuildStep needs to implement Step
type DockerBuildStep struct {
	*core.BaseStep
	options            *core.PipelineOptions
	dockerOptions      *Options
	data               map[string]string
	tag                string
	logger             *util.LogEntry
	dockerfile         string
	extrahosts         []string
	q                  bool
	squash             bool
	buildargs          map[string]*string
	labels             map[string]string
	nocache            bool
	authConfigs        map[string]types.AuthConfig
	dockerBuildContext string
}

// NewDockerBuildStep is a special step for doing docker builds
func NewDockerBuildStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*DockerBuildStep, error) {
	name := "docker-build"
	displayName := "docker build"
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

	return &DockerBuildStep{
		BaseStep:      baseStep,
		data:          stepConfig.Data,
		logger:        util.RootLogger().WithField("Logger", "DockerBuildStep"),
		options:       options,
		dockerOptions: dockerOptions,
	}, nil
}

func (s *DockerBuildStep) configure(env *util.Environment) error {
	if imagename, ok := s.data["image-name"]; ok {
		// note that Execute() fails the step (naming the image-name property) if this is not set
		// we don't let the user specify the tag directly, but prepend it with the build ID
		s.tag = s.options.RunID + env.Interpolate(imagename)
	}

	if dockerfile, ok := s.data["dockerfile"]; ok {
		s.dockerfile = env.Interpolate(dockerfile)
	}

	if labelsProp, ok := s.data["labels"]; ok {
		parsedLabels, err := shlex.Split(labelsProp)
		if err == nil {
			labelMap := make(map[string]string)
			for _, labelPair := range parsedLabels {
				pair := strings.Split(labelPair, "=")
				labelMap[env.Interpolate(pair[0])] = env.Interpolate(pair[1])
			}
			s.labels = labelMap
		}
	}

	if buildargsProp, ok := s.data["build-args"]; ok {
		parsedArgs, err := shlex.Split(buildargsProp)
		if err == nil {
			s.buildargs = make(map[string]*string)
			for _, labelPair := range parsedArgs {
				pair := strings.Split(labelPair, "=")
				name := env.Interpolate(pair[0])
				value := env.Interpolate(pair[1])
				s.buildargs[name] = &value
			}
		}
	}

	s.q = false // default to false (verbose) when value is bad or not set
	if qProp, ok := s.data["q"]; ok {
		q, err := strconv.ParseBool(qProp)
		if err == nil {
			s.q = q
		}
	}

	s.nocache = false // default to false when value is bad or not set
	if nocacheProp, ok := s.data["no-cache"]; ok {
		nocache, err := strconv.ParseBool(nocacheProp)
		if err == nil {
			s.nocache = nocache
		}
	}

	if extrahostsProp, ok := s.data["extra-hosts"]; ok {
		parsedExtrahosts, err := shlex.Split(extrahostsProp)
		if err == nil {
			interpolatedExtrahosts := make([]string, len(parsedExtrahosts))
			for i, thisExtrahost := range parsedExtrahosts {
				interpolatedExtrahosts[i] = env.Interpolate(thisExtrahost)
			}
			s.extrahosts = interpolatedExtrahosts
		}
	}

	s.squash = false // default to false (do not squash) when value is bad or not set
	if squashProp, ok := s.data["squash"]; ok {
		squash, err := strconv.ParseBool(squashProp)
		if err == nil {
			s.squash = squash
		}
	}

	if registryAuthConfig, ok := s.data["registry-auth-config"]; ok {
		in := []byte(registryAuthConfig)
		var raw map[string]types.AuthConfig
		err := json.Unmarshal(in, &raw)
		if err != nil {
			return err
		}
		authConfigs := make(map[string]types.AuthConfig)
		for key, value := range raw {
			registry := dockerauth.NormalizeRegistry(env.Interpolate(key))
			if registry != "" {
				if value.Username != "" && value.Password != "" {
					authConfig := types.AuthConfig{
						Username: env.Interpolate(value.Username),
						Password: env.Interpolate(value.Password),
					}
					authConfigs[registry] = authConfig
				}
			}
		}
		s.authConfigs = authConfigs
	}

	if dockerBuildContext, ok := s.data["context"]; ok {
		s.dockerBuildContext = env.Interpolate(dockerBuildContext)
	}
	return nil
}

// InitEnv parses our data into our config
func (s *DockerBuildStep) InitEnv(ctx context.Context, env *util.Environment) error {
	err := s.configure(env)
	if err != nil {
		return err
	}
	return nil
}

// Fetch NOP
func (s *DockerBuildStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// Execute builds an image
func (s *DockerBuildStep) Execute(ctx context.Context, sess *core.Session) (int, error) {
	s.logger.Debugln("Starting DockerBuildStep", s.data)

	if s.tag == "" {
		return -1, errors.New("image-name not set")
	}

	tarfileName := "currentSourceUnderRoot.tar"
	err := s.buildTarfile(ctx, sess, tarfileName)
	if err != nil {
		return -1, err
	}
	return s.buildImage(ctx, sess, tarfileName)
}

// buildTarfile creates the tarfile that will be sent to the daemon to perform the docker build
func (s *DockerBuildStep) buildTarfile(ctx context.Context, sess *core.Session, tarfileName string) error {

	// This is clearly only relevant to docker so we're going to dig into the
	// transport internals a little bit to get the container ID
	dt := sess.Transport().(*DockerTransport) //              TODO Change this to use code which doesn't use fsouza client
	containerID := dt.containerID

	// Extract the /pipeline/source directory from the running pipeline container
	// and save it as a tarfile currentSource.tar
	_, err := s.CollectArtifact(ctx, containerID)
	if err != nil {
		return err
	}

	// In currentSource.tar, the source directory is in /source
	// Copy all the files that are under /source in currentSource.tar
	// into the / directory of a new tarfile with the specified name
	// This will be the tar we sent to the docker build command
	err = s.buildInputTar("currentSource.tar", tarfileName)
	if err != nil {
		return err
	}
	return nil
}

// buildImage builds an image by running the docker BuildImage function with  the specified tarfile
func (s *DockerBuildStep) buildImage(ctx context.Context, sess *core.Session, tarfileName string) (int, error) {
	s.logger.Debugln("Starting DockerBuildStep", s.data)

	officialClient, err := NewOfficialDockerClient(s.dockerOptions)
	if err != nil {
		return 1, err
	}

	e, err := core.EmitterFromContext(ctx)
	if err != nil {
		return 1, err
	}

	tarFile, err := os.Open(s.options.HostPath(tarfileName))
	tarReader := bufio.NewReader(tarFile)

	s.logger.Debugln("Build image")

	// Note: This is a little hack; if a network was not passed through a flag,
	//       then s.options.DockerNetworkName will contain the generated name.
	networkName := s.dockerOptions.NetworkName
	if networkName == "" {
		networkName = s.options.DockerNetworkName
	}

	officialBuildOpts := types.ImageBuildOptions{
		Dockerfile:     s.dockerfile,
		Tags:           []string{s.tag},
		BuildArgs:      s.buildargs,
		SuppressOutput: s.q,
		Remove:         s.options.ShouldRemove, // remove intermediate containers when successful unless --no-remove specified in CLI
		ForceRemove:    s.options.ShouldRemove, // remove intermediate containers when unsuccessful unless --no-remove specified in CLI
		Labels:         s.labels,
		ExtraHosts:     s.extrahosts,
		Squash:         s.squash,
		PullParent:     !s.dockerOptions.Local, // always pull images unless docker-local is specified
		NoCache:        s.nocache,
		NetworkMode:    networkName,
		AuthConfigs:    s.authConfigs,
	}

	imageBuildResponse, err := officialClient.ImageBuild(ctx, tarReader, officialBuildOpts)
	if err != nil {
		s.logger.Errorln("Failed to build image:", err)
		return -1, err
	}

	err = EmitStatus(e, imageBuildResponse.Body, s.options)
	if err != nil {
		return -1, err
	}
	imageBuildResponse.Body.Close()

	s.logger.Debug("Image built")
	return 0, nil
}

// CollectFile NOP
func (s *DockerBuildStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact copies the /pipeline/source directory from the running pipeline container
// and saves it as a directory currentSource and a tarfile currentSource.tar
func (s *DockerBuildStep) CollectArtifact(ctx context.Context, containerID string) (*core.Artifact, error) {
	artificer := NewArtificer(s.options, s.dockerOptions)

	artifact := &core.Artifact{
		ContainerID:   containerID,
		GuestPath:     s.options.GuestPath("source"),
		HostPath:      s.options.HostPath("currentSource"),
		HostTarPath:   s.options.HostPath("currentSource.tar"),
		ApplicationID: s.options.ApplicationID,
		RunID:         s.options.RunID,
		Bucket:        s.options.S3Bucket,
	}

	s.logger.WithFields(util.LogFields{
		"ContainerID":   artifact.ContainerID,
		"GuestPath":     artifact.GuestPath,
		"HostPath":      artifact.HostPath,
		"HostTarPath":   artifact.HostTarPath,
		"ApplicationID": artifact.ApplicationID,
		"RunID":         artifact.RunID,
		"Bucket":        artifact.Bucket,
	}).Debugln("Collecting artifacts from container to ", artifact.HostTarPath)

	fullArtifact, err := artificer.Collect(ctx, artifact)
	if err != nil {
		return nil, err
	}
	return fullArtifact, nil
}

func (s *DockerBuildStep) buildInputTar(sourceTar string, destTar string) error {
	// In currentSource.tar, the source directory is in /source
	// Copy all the files that are under /source/ + context in currentSource.tar
	// into the / directory of a new tarfile currentSourceInRoot.tar
	artifactReader, err := os.Open(s.options.HostPath(sourceTar))
	if err != nil {
		return err
	}
	defer artifactReader.Close()

	s.logger.Debugln("Building input tar", s.options.HostPath(destTar))

	layerFile, err := os.OpenFile(s.options.HostPath(destTar), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer layerFile.Close()

	tr := tar.NewReader(artifactReader)
	tw := tar.NewWriter(layerFile)

	dockerBuildContext := "source"
	if s.dockerBuildContext != "" {
		dockerBuildContext = filepath.Join(dockerBuildContext, s.dockerBuildContext)
	}
	dockerBuildContext = dockerBuildContext + string(os.PathSeparator)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// finished the tarball
			err = tw.Close()
			if err != nil {
				return err
			}
			break
		}

		if err != nil {
			return err
		}

		// Skip the base dir
		if hdr.Name == "./" {
			continue
		}

		// Copy files under the specified build context into the root of the new tar
		if strings.HasPrefix(hdr.Name, dockerBuildContext) {
			hdr.Name = hdr.Name[len(dockerBuildContext):]
		} else {
			continue
		}

		tw.WriteHeader(hdr)
		_, err = io.Copy(tw, tr)
		if err != nil {
			return err
		}

	}
	return nil
}

// ReportPath NOP
func (s *DockerBuildStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = TRUE
func (s *DockerBuildStep) ShouldSyncEnv() bool {
	// If disable-sync is set, only sync if it is not true
	if disableSync, ok := s.data["disable-sync"]; ok {
		return disableSync != "true"
	}
	return true
}
