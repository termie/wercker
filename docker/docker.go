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
	"archive/tar"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/go-connections/nat"
	"github.com/google/shlex"
	digest "github.com/opencontainers/go-digest"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/wercker/docker-check-access"
	"github.com/wercker/wercker/auth"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

const (
	NoPushConfirmationInStatus = "Docker push failed to complete. Please check logs for any error condition.."
)

// GenerateDockerID will generate a cryptographically random 256 bit hex Docker
// identifier.
func GenerateDockerID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

// DockerScratchPushStep creates a new image based on a scratch tarball and
// pushes it
type DockerScratchPushStep struct {
	*DockerPushStep
}

// NewDockerScratchPushStep constructorama
func NewDockerScratchPushStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*DockerScratchPushStep, error) {
	name := "docker-scratch-push"
	displayName := "docker scratch'n'push"
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

	dockerPushStep := &DockerPushStep{
		BaseStep:      baseStep,
		data:          stepConfig.Data,
		dockerOptions: dockerOptions,
		options:       options,
		logger:        util.RootLogger().WithField("Logger", "DockerScratchPushStep"),
	}

	return &DockerScratchPushStep{DockerPushStep: dockerPushStep}, nil
}

// Execute the scratch-n-push
func (s *DockerScratchPushStep) Execute(ctx context.Context, sess *core.Session) (int, error) {
	s.tags = s.buildTags()
	s.repository = s.authenticator.Repository(s.repository)

	named, err := reference.WithName(s.repository)
	if err != nil {
		return -1, errors.Wrapf(err, "Invalid repository: %s", s.repository)
	}

	err = validateTags(named, s.tags)
	if err != nil {
		return -1, err
	}

	// This is clearly only relevant to docker so we're going to dig into the
	// transport internals a little bit to get the container ID
	dt := sess.Transport().(*DockerTransport)
	containerID := dt.containerID

	_, err = s.CollectArtifact(ctx, containerID)
	if err != nil {
		return -1, err
	}

	// layer.tar has an extra folder in it so we have to strip it :/
	artifactReader, err := os.Open(s.options.HostPath("layer.tar"))
	if err != nil {
		return -1, err
	}
	defer artifactReader.Close()

	layerFile, err := os.OpenFile(s.options.HostPath("real_layer.tar"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return -1, err
	}
	defer layerFile.Close()

	digester := digest.Canonical.Digester()
	mwriter := io.MultiWriter(layerFile, digester.Hash())

	tr := tar.NewReader(artifactReader)
	tw := tar.NewWriter(mwriter)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// finished the tarball
			break
		}

		if err != nil {
			return -1, err
		}

		// Skip the base dir
		if hdr.Name == "./" {
			continue
		}

		if strings.HasPrefix(hdr.Name, "output/") {
			hdr.Name = hdr.Name[len("output/"):]
		} else if strings.HasPrefix(hdr.Name, "source/") {
			hdr.Name = hdr.Name[len("source/"):]
		}

		if len(hdr.Name) == 0 {
			continue
		}

		tw.WriteHeader(hdr)
		_, err = io.Copy(tw, tr)
		if err != nil {
			return -1, err
		}
	}

	config := &container.Config{
		Cmd:          s.cmd,
		Entrypoint:   s.entrypoint,
		Env:          s.env,
		Hostname:     containerID[:16],
		WorkingDir:   s.workingDir,
		Volumes:      s.volumes,
		ExposedPorts: s.ports,
	}

	// Make the JSON file we need
	t := time.Now()
	base := image.V1Image{
		Architecture: "amd64",
		Container:    containerID,
		ContainerConfig: container.Config{
			Hostname: containerID[:16],
		},
		DockerVersion: "1.10",
		Created:       t,
		OS:            "linux",
		Config:        config,
	}

	imageJSON := image.Image{
		V1Image: base,
		History: []image.History{image.History{Created: t}},
		RootFS: &image.RootFS{
			Type:    "layers",
			DiffIDs: []layer.DiffID{layer.DiffID(digester.Digest())},
		},
	}

	js, err := imageJSON.MarshalJSON()
	if err != nil {
		return -1, err
	}

	hash := sha256.New()
	hash.Write(js)
	layerID := hex.EncodeToString(hash.Sum(nil))

	err = os.MkdirAll(s.options.HostPath("scratch", layerID), 0755)
	if err != nil {
		return -1, err
	}

	layerFile.Close()

	err = os.Rename(layerFile.Name(), s.options.HostPath("scratch", layerID, "layer.tar"))
	if err != nil {
		return -1, err
	}
	defer os.RemoveAll(s.options.HostPath("scratch"))

	// VERSION file
	versionFile, err := os.OpenFile(s.options.HostPath("scratch", layerID, "VERSION"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return -1, err
	}
	defer versionFile.Close()

	_, err = versionFile.Write([]byte("1.0"))
	if err != nil {
		return -1, err
	}

	err = versionFile.Sync()
	if err != nil {
		return -1, err
	}

	// json file
	jsonFile, err := os.OpenFile(s.options.HostPath("scratch", layerID, "json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return -1, err
	}
	defer jsonFile.Close()

	_, err = jsonFile.Write(js)
	if err != nil {
		return -1, err
	}

	err = jsonFile.Sync()
	if err != nil {
		return -1, err
	}

	// repositories file
	repositoriesFile, err := os.OpenFile(s.options.HostPath("scratch", "repositories"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return -1, err
	}
	defer repositoriesFile.Close()

	_, err = repositoriesFile.Write([]byte(fmt.Sprintf(`{"%s":{`, s.authenticator.Repository(s.repository))))
	if err != nil {
		return -1, err
	}

	for i, tag := range s.tags {
		_, err = repositoriesFile.Write([]byte(fmt.Sprintf(`"%s":"%s"`, tag, layerID)))
		if err != nil {
			return -1, err
		}
		if i != len(s.tags)-1 {
			_, err = repositoriesFile.Write([]byte{','})
			if err != nil {
				return -1, err
			}
		}
	}

	_, err = repositoriesFile.Write([]byte{'}', '}'})
	err = repositoriesFile.Sync()
	if err != nil {
		return -1, err
	}

	// Build our output tarball and start writing to it
	imageFile, err := os.Create(s.options.HostPath("scratch.tar"))
	if err != nil {
		return -1, err
	}
	defer imageFile.Close()

	err = util.TarPath(imageFile, s.options.HostPath("scratch"))
	if err != nil {
		return -1, err
	}
	imageFile.Close()

	client, err := NewOfficialDockerClient(s.dockerOptions)
	if err != nil {
		return 1, err
	}

	s.logger.WithFields(util.LogFields{
		"Repository": s.repository,
		"Tags":       s.tags,
		"Message":    s.message,
	}).Debug("Scratch push to registry")

	// Okay, we can access it, do a docker load to import the image then push it
	loadFile, err := os.Open(s.options.HostPath("scratch.tar"))
	if err != nil {
		return -1, err
	}
	defer loadFile.Close()

	e, err := core.EmitterFromContext(ctx)
	if err != nil {
		return 1, err
	}

	imageLoadResponse, err := client.ImageLoad(ctx, loadFile, false)
	if err != nil {
		return 1, err
	}
	defer imageLoadResponse.Body.Close()
	EmitStatus(e, imageLoadResponse.Body, s.options)

	return s.tagAndPush(ctx, layerID, e, client)
}

// CollectArtifact is copied from the build, we use this to get the layer
// tarball that we'll include in the image tarball
func (s *DockerScratchPushStep) CollectArtifact(ctx context.Context, containerID string) (*core.Artifact, error) {
	artificer := NewArtificer(s.options, s.dockerOptions)

	// Ensure we have the host directory

	artifact := &core.Artifact{
		ContainerID:   containerID,
		GuestPath:     s.options.GuestPath("output"),
		HostPath:      s.options.HostPath("layer"),
		HostTarPath:   s.options.HostPath("layer.tar"),
		ApplicationID: s.options.ApplicationID,
		RunID:         s.options.RunID,
		Bucket:        s.options.S3Bucket,
	}

	sourceArtifact := &core.Artifact{
		ContainerID:   containerID,
		GuestPath:     s.options.BasePath(),
		HostPath:      s.options.HostPath("layer"),
		HostTarPath:   s.options.HostPath("layer.tar"),
		ApplicationID: s.options.ApplicationID,
		RunID:         s.options.RunID,
		Bucket:        s.options.S3Bucket,
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

// DockerPushStep needs to implemenet IStep
type DockerPushStep struct {
	*core.BaseStep
	options       *core.PipelineOptions
	dockerOptions *Options
	data          map[string]string
	email         string
	env           []string
	stopSignal    string
	builtInPush   bool
	labels        map[string]string
	user          string
	authServer    string
	repository    string
	author        string
	message       string
	tags          []string
	ports         nat.PortSet
	volumes       map[string]struct{}
	cmd           []string
	entrypoint    []string
	logger        *util.LogEntry
	workingDir    string
	authenticator auth.Authenticator
	// imageName contains the value specified in image-name option of the step.
	// This image-name MUST be same as image-name option specified in one of the previous internal/docker-build steps.
	// if set, the name of the existing image built by a previous internal/docker-build step is computed by prepending the build ID to the specified image-name property.
	// if imageName is set then the existing pre-built image is tagged and pushed.
	// if imageName is not set then the pipeline container is committed, tagged and pushed (classic behaviour)
	imageName string
}

// NewDockerPushStep is a special step for doing docker pushes
func NewDockerPushStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*DockerPushStep, error) {
	name := "docker-push"
	displayName := "docker push"
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

	return &DockerPushStep{
		BaseStep:      baseStep,
		data:          stepConfig.Data,
		logger:        util.RootLogger().WithField("Logger", "DockerPushStep"),
		options:       options,
		dockerOptions: dockerOptions,
	}, nil
}

func (s *DockerPushStep) configure(env *util.Environment) error {
	if email, ok := s.data["email"]; ok {
		s.email = env.Interpolate(email)
	}

	if authServer, ok := s.data["auth-server"]; ok {
		s.authServer = env.Interpolate(authServer)
	}

	if repository, ok := s.data["repository"]; ok {
		s.repository = env.Interpolate(repository)
	}

	if tags, ok := s.data["tag"]; ok {
		interpolatedTags := env.Interpolate(tags)
		s.tags = util.SplitSpaceOrComma(interpolatedTags)
	}

	if author, ok := s.data["author"]; ok {
		s.author = env.Interpolate(author)
	}

	if message, ok := s.data["message"]; ok {
		s.message = env.Interpolate(message)
	}

	if ports, ok := s.data["ports"]; ok {
		iPorts := env.Interpolate(ports)
		parts := util.SplitSpaceOrComma(iPorts)
		portset := make(nat.PortSet)
		for _, portAndProto := range parts {
			portAndProto = strings.TrimSpace(portAndProto) // The number can end with /tcp or /udp. If omitted,/tcp will be used.
			portAndProtoSplit := strings.Split(portAndProto, "/")
			var port, proto string
			if len(portAndProtoSplit) > 1 {
				port = portAndProtoSplit[0]
				proto = portAndProtoSplit[1]
			} else {
				port = portAndProtoSplit[0]
				proto = "tcp"
			}
			p, err := nat.NewPort(proto, port)
			if err != nil {
				return fmt.Errorf("Invalid port %s: %s", port, err.Error())
			}
			portset[p] = struct{}{}
		}
		s.ports = portset
	}

	if volumes, ok := s.data["volumes"]; ok {
		iVolumes := env.Interpolate(volumes)
		parts := util.SplitSpaceOrComma(iVolumes)
		volumemap := make(map[string]struct{})
		for _, volume := range parts {
			volume = strings.TrimSpace(volume)
			volumemap[volume] = struct{}{}
		}
		s.volumes = volumemap
	}

	if workingDir, ok := s.data["working-dir"]; ok {
		s.workingDir = env.Interpolate(workingDir)
	}

	if cmd, ok := s.data["cmd"]; ok {
		parts, err := shlex.Split(cmd)
		if err == nil {
			s.cmd = parts
		}
	}

	if entrypoint, ok := s.data["entrypoint"]; ok {
		parts, err := shlex.Split(entrypoint)
		if err == nil {
			s.entrypoint = parts
		}
	}

	if envi, ok := s.data["env"]; ok {
		parsedEnv, err := shlex.Split(envi)

		if err == nil {
			interpolatedEnv := make([]string, len(parsedEnv))
			for i, envVar := range parsedEnv {
				interpolatedEnv[i] = env.Interpolate(envVar)
			}
			s.env = interpolatedEnv
		}
	}

	if stopsignal, ok := s.data["stopsignal"]; ok {
		s.stopSignal = env.Interpolate(stopsignal)
	}

	if labels, ok := s.data["labels"]; ok {
		parsedLabels, err := shlex.Split(labels)
		if err == nil {
			labelMap := make(map[string]string)
			for _, labelPair := range parsedLabels {
				pair := strings.Split(labelPair, "=")
				if len(pair) != 2 {
					return fmt.Errorf("label specification %s is not of the form label=value", labelPair)
				}
				labelMap[env.Interpolate(pair[0])] = env.Interpolate(pair[1])
			}
			s.labels = labelMap
		}
	}

	if user, ok := s.data["user"]; ok {
		s.user = env.Interpolate(user)
	}

	if imageName, ok := s.data["image-name"]; ok {
		s.imageName = env.Interpolate(imageName)
	}

	return nil
}

func (s *DockerPushStep) buildAutherOpts(ctx context.Context, env *util.Environment) (dockerauth.CheckAccessOptions, error) {
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
	if awsAccessKey, ok := s.data["aws-access-key"]; ok {
		opts.AwsAccessKey = env.Interpolate(awsAccessKey)
	}

	if awsSecretKey, ok := s.data["aws-secret-key"]; ok {
		opts.AwsSecretKey = env.Interpolate(awsSecretKey)
	}

	if awsRegion, ok := s.data["aws-region"]; ok {
		opts.AwsRegion = env.Interpolate(awsRegion)
	}

	if awsAuth, ok := s.data["aws-strict-auth"]; ok {
		auth, err := strconv.ParseBool(awsAuth)
		if err == nil {
			opts.AwsStrictAuth = auth
		}
	}

	if awsRegistryID, ok := s.data["aws-registry-id"]; ok {
		opts.AwsRegistryID = env.Interpolate(awsRegistryID)
	}

	if azureClient, ok := s.data["azure-client-id"]; ok {
		opts.AzureClientID = env.Interpolate(azureClient)
	}

	if azureClientSecret, ok := s.data["azure-client-secret"]; ok {
		opts.AzureClientSecret = env.Interpolate(azureClientSecret)
	}

	if azureSubscriptionID, ok := s.data["azure-subscription-id"]; ok {
		opts.AzureSubscriptionID = env.Interpolate(azureSubscriptionID)
	}

	if azureTenantID, ok := s.data["azure-tenant-id"]; ok {
		opts.AzureTenantID = env.Interpolate(azureTenantID)
	}

	if azureResourceGroupName, ok := s.data["azure-resource-group"]; ok {
		opts.AzureResourceGroupName = env.Interpolate(azureResourceGroupName)
	}

	if azureRegistryName, ok := s.data["azure-registry-name"]; ok {
		opts.AzureRegistryName = env.Interpolate(azureRegistryName)
	}

	if azureLoginServer, ok := s.data["azure-login-server"]; ok {
		opts.AzureLoginServer = env.Interpolate(azureLoginServer)
	}

	// If user use Azure or AWS container registry we don't infer.
	if opts.AzureClientSecret == "" && opts.AwsSecretKey == "" {
		repository, registry, err := InferRegistryAndRepository(ctx, s.repository, opts.Registry, s.options)
		if err != nil {
			return dockerauth.CheckAccessOptions{}, err
		}
		s.repository = repository
		opts.Registry = registry
	}

	// Set user and password automatically if using wercker registry
	if opts.Registry == s.options.WerckerContainerRegistry.String() {
		opts.Username = DefaultDockerRegistryUsername
		opts.Password = s.options.AuthToken
		s.builtInPush = true
	}

	return opts, nil
}

// log logs the specified message using the specified emitter
func emit(e *core.NormalizedEmitter, message string) {
	e.Emit(core.Logs, &core.LogsArgs{
		Logs: message,
	})
}

//InferRegistryAndRepository infers the registry and repository to be used from input registry and repository.
// 1. If no repository is specified then an error "Repository not specified" will be returned.
// 2. In case a repository is provided but no registry - registry is derived from the name of the domain (if any)
//    from the registry - e.g. for a repository quay.io/<repo-owner>/<repo-name> - quay.io will be the registry host
//    and https://quay.io/v2/ will be the registry url. In case the repository name does not contain a domain name -
//    docker hub is assumed to be the registry and therefore any authorization with supplied username/password is carried
//    out with docker hub.
// 3. In case both repository and registry are provided -
//    3(a) - In case registry provided points to a wrong url - we use registry inferred from the domain name(if any) prefixed
//           to the repository. However in this case if no domain name is specified in repository - we return an error since
//           user probably wanted to use this repository with a different registry and not docker hub and should be alerted
//           that the registry url is invalid.In case registry url is valid - we evaluate scenarios 4(b) and 4(c)
//    3(b) - In case no domain name is prefixed to the repository - we assume repository belongs to the registry specified
//           and prefix domain name extracted from registry.
//    3(c) - In case repository also contains a domain name - we check if domain name of registry and repository are same,
//           we assume that user wanted to use the registry host as specified in repository and change the registry to point
//           to domain name present in repository. If domain names in both registry and repository are same - no changes are
//           made.
func InferRegistryAndRepository(ctx context.Context, repository string, registry string, pipelineOptions *core.PipelineOptions) (inferredRepository string, inferredRegistry string, err error) {
	e, err := core.EmitterFromContext(ctx)
	if err != nil {
		return "", "", err
	}
	if repository == "" {
		return "", "", fmt.Errorf("Repository not specified")
	}
	// Docker repositories must be lowercase
	inferredRepository = strings.ToLower(repository)
	inferredRegistry = registry
	x, err := reference.ParseNormalizedNamed(inferredRepository)
	if err != nil {
		return "", "", fmt.Errorf("%s is not a valid repository, error while validating repository name: %s", inferredRepository, err.Error())
	}
	domainFromRepository := reference.Domain(x)
	registryInferredFromRepository := ""
	if domainFromRepository != "docker.io" {
		reg := &url.URL{Scheme: "https", Host: domainFromRepository, Path: "/v2"}
		registryInferredFromRepository = reg.String() + "/"
	}

	if len(strings.TrimSpace(inferredRegistry)) != 0 {
		registryURLFromStepConfig, err := url.ParseRequestURI(inferredRegistry)
		if err != nil {
			emit(e, fmt.Sprintf("Invalid registry url specified: %s\n", err.Error()))
			if registryInferredFromRepository != "" {
				emit(e, fmt.Sprintf("Using registry url inferred from repository: %s\n"+registryInferredFromRepository))
				inferredRegistry = registryInferredFromRepository
			} else {
				emit(e, "Please specify valid registry parameter.If you intended to use docker hub as registry, you may omit registry parameter")
				return "", "", fmt.Errorf("%s is not a valid registry URL, error: %s", inferredRegistry, err.Error())
			}

		} else {
			domainFromRegistryURL := registryURLFromStepConfig.Host
			if len(strings.TrimSpace(domainFromRepository)) != 0 && domainFromRepository != "docker.io" {
				if domainFromRegistryURL != domainFromRepository {
					emit(e, "Different registry hosts specified in repository: "+domainFromRepository+" and registry: "+domainFromRegistryURL)
					inferredRegistry = registryInferredFromRepository
					emit(e, "Using registry inferred from repository: "+inferredRegistry)
				}
			} else {
				inferredRepository = domainFromRegistryURL + "/" + inferredRepository
				emit(e, "Using repository inferred from registry: "+inferredRepository)
			}

		}
	} else {
		inferredRegistry = registryInferredFromRepository
	}
	return inferredRepository, inferredRegistry, nil
}

// InitEnv parses our data into our config
func (s *DockerPushStep) InitEnv(ctx context.Context, env *util.Environment) error {
	err := s.configure(env)
	if err != nil {
		return err
	}
	opts, err := s.buildAutherOpts(ctx, env)
	if err != nil {
		return err
	}

	auther, err := dockerauth.GetRegistryAuthenticator(opts)
	if err != nil {
		return err
	}

	s.authenticator = auther
	return nil
}

// Fetch NOP
func (s *DockerPushStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// Execute - commits the current container, tags the image based on tags provided in step options
// and pushes it to the configured registry when image-name property is not specified, in which case,
// it tags and pushes an existing image built by a previous internal/docker-build step
func (s *DockerPushStep) Execute(ctx context.Context, sess *core.Session) (int, error) {
	s.tags = s.buildTags()
	s.repository = s.authenticator.Repository(s.repository)

	named, err := reference.WithName(s.repository)
	if err != nil {
		return -1, errors.Wrapf(err, "Invalid repository: %s", s.repository)
	}

	err = validateTags(named, s.tags)
	if err != nil {
		return -1, err
	}

	ref, _ := reference.WithTag(named, s.tags[0])

	client, err := NewOfficialDockerClient(s.dockerOptions)
	if err != nil {
		return 1, err
	}
	e, err := core.EmitterFromContext(ctx)
	if err != nil {
		return 1, err
	}

	s.logger.WithFields(util.LogFields{
		"Repository": s.repository,
		"Tags":       s.tags,
		"Message":    s.message,
	}).Debug("Push to registry")

	// This is clearly only relevant to docker so we're going to dig into the
	// transport internals a little bit to get the container ID
	dt := sess.Transport().(*DockerTransport)
	containerID := dt.containerID

	var imageRef = ""
	// if imageName is not specified then create a new image by committing the pipeline container
	if s.imageName == "" {
		config := container.Config{
			Cmd:          s.cmd,
			Entrypoint:   s.entrypoint,
			WorkingDir:   s.workingDir,
			User:         s.user,
			Env:          s.env,
			StopSignal:   s.stopSignal,
			Labels:       s.labels,
			ExposedPorts: s.ports,
			Volumes:      s.volumes,
		}

		containerCommitOptions := types.ContainerCommitOptions{
			Reference: ref.String(),
			Comment:   s.message,
			Author:    s.author,
			Config:    &config,
		}

		s.logger.Debugln("Commiting container:", containerID)
		idResponse, err := client.ContainerCommit(ctx, containerID, containerCommitOptions)
		if err != nil {
			return -1, err
		}
		imageRef = idResponse.ID
		s.logger.WithField("imageId", imageRef).Debug("Commit completed")
	} else {
		// if imageName is specified then compute image name by prepedning the runID to value of imageName
		imageRef = s.options.RunID + s.imageName
		msg := fmt.Sprintf("Pushing image built using internal/docker-build step, image name: %s\n", s.imageName)
		s.logger.Debug(msg)
		emit(e, msg)
	}
	return s.tagAndPush(ctx, imageRef, e, client)
}

func (s *DockerPushStep) buildTags() []string {
	if len(s.tags) == 0 && !s.builtInPush {
		s.tags = []string{"latest"}
	} else if len(s.tags) == 0 && s.builtInPush {
		gitTag := fmt.Sprintf("%s-%s", s.options.GitBranch, s.options.GitCommit)
		s.tags = []string{"latest", gitTag}
	}
	return s.tags
}

func (s *DockerPushStep) tagAndPush(ctx context.Context, imageRef string, e *core.NormalizedEmitter, client *OfficialDockerClient) (int, error) {
	// Create a pipe since we want a io.Reader but Docker expects a io.Writer
	r, w := io.Pipe()
	// emitStatusses in a different go routine
	go EmitStatus(e, r, s.options)
	defer w.Close()
	for _, tag := range s.tags {

		target := fmt.Sprintf("%s:%s", s.repository, tag)
		s.logger.Println("Pushing image for ", target)
		err := client.ImageTag(ctx, imageRef, target)
		if err != nil {
			s.logger.Errorln("Failed to push:", err)
			return 1, err
		}
		if s.dockerOptions.CleanupImage {
			defer cleanupImage(ctx, s.logger, client.Client, s.repository, tag)
		}
		if !s.dockerOptions.Local {
			authConfig := types.AuthConfig{
				Username: s.authenticator.Username(),
				Password: s.authenticator.Password(),
				Email:    s.email,
			}
			authEncodedJSON, err := json.Marshal(authConfig)
			if err != nil {
				s.logger.Errorln("Failed to encode auth:", err)
				return 1, err
			}

			authEncodedJSON, err = fixForGCR(authEncodedJSON, authConfig.Username, s.repository)
			if err != nil {
				s.logger.Errorln(err)
				return 1, err
			}

			authStr := base64.URLEncoding.EncodeToString(authEncodedJSON)
			imagePushOptions := types.ImagePushOptions{
				RegistryAuth: authStr,
			}
			response, err := client.ImagePush(ctx, target, imagePushOptions)
			if err != nil {
				s.logger.Errorln("Failed to push:", err)
				return 1, err
			}
			defer response.Close()

			err = EmitStatus(e, response, s.options)
			if err != nil {
				return 1, err
			}
			emit(e, fmt.Sprintf("\nPushed %s:%s\n", s.repository, tag))
		}
	}
	return 0, nil
}

func cleanupImage(ctx context.Context, logger *util.LogEntry, client *client.Client, repository, tag string) {
	imageName := fmt.Sprintf("%s:%s", repository, tag)
	_, err := client.ImageRemove(ctx, imageName, types.ImageRemoveOptions{})
	if err != nil {
		logger.
			WithError(err).
			WithField("imageName", imageName).
			Warn("Failed to delete image")
	} else {
		logger.
			WithField("imageName", imageName).
			Debug("Deleted image")
	}
}

// CollectFile NOP
func (s *DockerPushStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact NOP
func (s *DockerPushStep) CollectArtifact(context.Context, string) (*core.Artifact, error) {
	return nil, nil
}

// ReportPath NOP
func (s *DockerPushStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = TRUE
func (s *DockerPushStep) ShouldSyncEnv() bool {
	// If disable-sync is set, only sync if it is not true
	if disableSync, ok := s.data["disable-sync"]; ok {
		return disableSync != "true"
	}
	return true
}

// validateTags verifies that all of the `tags` are in valid format
// and returns an error if not.
func validateTags(repository reference.Named, tags []string) error {
	for _, tag := range tags {
		_, err := reference.WithTag(repository, tag)
		if err != nil {
			return errors.Wrapf(err, "Invalid tag: %s", tag)
		}
	}
	return nil
}

// fixForGCR verifies whether docker push is for a gcr repository and is using json key file
// for authentication as described in https://cloud.google.com/container-registry/docs/advanced-authentication#json_key_file.
// If yes, replaces all occurrences of \\n with \n from the encoded json
func fixForGCR(authEncodedJSON []byte, username string, repository string) ([]byte, error) {
	if username != "_json_key" {
		return authEncodedJSON, nil
	}

	x, err := reference.ParseNormalizedNamed(repository)
	// This error condition is impossible as we already check this in InferRegistryAndRepository but its better
	// to not proceed with push if this indeed fails
	if err != nil {
		return nil, fmt.Errorf("%s is not a valid repository, error while validating repository name: %s", repository, err.Error())
	}

	hostNameFromRepository := reference.Domain(x)
	isGCR := ("gcr.io" == hostNameFromRepository) || strings.HasSuffix(hostNameFromRepository, ".gcr.io")
	if !isGCR {
		return authEncodedJSON, nil
	}

	authEncodedJSON = bytes.Replace(authEncodedJSON, []byte("\\\\n"), []byte("\\n"), -1)
	return authEncodedJSON, nil
}
