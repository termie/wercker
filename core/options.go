//   Copyright © 2016, 2019, Oracle and/or its affiliates.  All rights reserved.
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

package core

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/pborman/uuid"
	"github.com/wercker/wercker/util"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/urfave/cli.v1"
)

var (
	DEFAULT_BASE_URL      = "https://app.wercker.com"
	DEFAULT_STEP_REGISTRY = "https://steps.wercker.com"
)

// GlobalOptions applicable to everything
type GlobalOptions struct {
	BaseURL         string
	StepRegistryURL string
	Debug           bool
	Verbose         bool
	ShowColors      bool
	LogJSON         bool

	// Auth
	AuthToken      string
	AuthTokenStore string

	// local-file-store
	LocalFileStore string
}

// guessAuthToken will attempt to read from the token store location if
// no auth token was provided
func guessAuthToken(c util.Settings, e *util.Environment, authTokenStore string) string {
	token, _ := c.GlobalString("auth-token")
	if token != "" {
		return token
	}
	if foundToken, _ := util.Exists(authTokenStore); !foundToken {
		return ""
	}

	tokenBytes, err := ioutil.ReadFile(authTokenStore)
	if err != nil {
		util.RootLogger().WithField("Logger", "Options").Errorln(err)
		return ""
	}
	return strings.TrimSpace(string(tokenBytes))
}

// NewGlobalOptions constructor
func NewGlobalOptions(c util.Settings, e *util.Environment) (*GlobalOptions, error) {
	baseURL, _ := c.GlobalString("base-url", DEFAULT_BASE_URL)
	stepsRegistryURL, _ := c.GlobalString("steps-registry")
	baseURL = strings.TrimRight(baseURL, "/")
	debug, _ := c.GlobalBool("debug")
	verbose, _ := c.GlobalBool("verbose")
	logJSON, _ := c.GlobalBool("log-json")
	// TODO(termie): switch negative flag
	showColors, _ := c.GlobalBool("no-colors")
	showColors = !showColors

	authTokenStore, _ := c.GlobalString("auth-token-store")
	authTokenStore = util.ExpandHomePath(authTokenStore, e.Get("HOME"))
	authToken := guessAuthToken(c, e, authTokenStore)

	localFileStore, _ := c.String("local-file-store")

	// If debug is true, than force verbose and do not use colors.
	if debug {
		verbose = true
		showColors = false
	}

	return &GlobalOptions{
		BaseURL:         baseURL,
		StepRegistryURL: stepsRegistryURL,
		Debug:           debug,
		Verbose:         verbose,
		ShowColors:      showColors,
		LogJSON:         logJSON,

		AuthToken:      authToken,
		AuthTokenStore: authTokenStore,

		LocalFileStore: localFileStore,
	}, nil
}

// AWSOptions for our artifact storage
type AWSOptions struct {
	*GlobalOptions
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSRegion          string
	S3Bucket           string
	S3PartSize         int64
}

// NewAWSOptions constructor
func NewAWSOptions(c util.Settings, e *util.Environment, globalOpts *GlobalOptions) (*AWSOptions, error) {
	awsAccessKeyID, _ := c.String("aws-access-key")
	awsRegion, _ := c.String("aws-region")
	awsSecretAccessKey, _ := c.String("aws-secret-key")
	s3Bucket, _ := c.String("s3-bucket")

	return &AWSOptions{
		GlobalOptions:      globalOpts,
		AWSAccessKeyID:     awsAccessKeyID,
		AWSRegion:          awsRegion,
		AWSSecretAccessKey: awsSecretAccessKey,
		S3Bucket:           s3Bucket,
		S3PartSize:         100 * 1024 * 1024, // 100 MB
	}, nil
}

// OCI flags
const OCI_TENANCY_OCID = "oci-tenancy-ocid"
const OCI_USER_OCID = "oci-user-ocid"
const OCI_REGION = "oci-region"
const OCI_PRIVATE_KEY_PATH = "oci-private-key-path"
const OCI_PRIVATE_KEY_PASSPHRASE = "oci-private-key-passphrase"
const OCI_FINGERPRINT = "oci-fingerprint"
const OCI_BUCKET = "oci-bucket"
const OCI_NAMESPACE = "oci-namespace"

// OCIOptions for OCI Object Store
type OCIOptions struct {
	*GlobalOptions
	TenancyOCID          string
	UserOCID             string
	Region               string
	PrivateKeyPath       string
	PrivateKeyPassphrase string
	Fingerprint          string
	Namespace            string
	Bucket               string
	ObjectName           string
	LocalPath            string
	TarBX                bool
}

// NewOCIOptions constructor
func NewOCIOptions(c util.Settings, e *util.Environment, globalOpts *GlobalOptions) (*OCIOptions, error) {
	tenancyOCID, _ := c.String(OCI_TENANCY_OCID)
	userOCID, _ := c.String(OCI_USER_OCID)
	region, _ := c.String(OCI_REGION)
	keyPath, _ := c.String(OCI_PRIVATE_KEY_PATH)
	passPhrase, _ := c.String(OCI_PRIVATE_KEY_PASSPHRASE)
	fingerprint, _ := c.String(OCI_FINGERPRINT)
	bucket, _ := c.String(OCI_BUCKET)
	namespace, _ := c.String(OCI_NAMESPACE)

	return &OCIOptions{
		GlobalOptions:        globalOpts,
		TenancyOCID:          tenancyOCID,
		UserOCID:             userOCID,
		Region:               region,
		PrivateKeyPath:       keyPath,
		PrivateKeyPassphrase: passPhrase,
		Fingerprint:          fingerprint,
		Bucket:               bucket,
		Namespace:            namespace,
	}, nil
}

// GitOptions for the users, mostly
type GitOptions struct {
	*GlobalOptions
	GitBranch     string
	GitTag        string
	GitCommit     string
	GitDomain     string
	GitOwner      string
	GitRepository string
}

func guessGitBranch(c util.Settings, e *util.Environment) string {
	branch, _ := c.String("git-branch")
	if branch != "" {
		return branch
	}

	projectPath := guessProjectPath(c, e)
	if projectPath == "" {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	defer os.Chdir(cwd)
	os.Chdir(projectPath)

	git, err := exec.LookPath("git")
	if err != nil {
		return ""
	}

	var out bytes.Buffer
	cmd := exec.Command(git, "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return ""
	}
	branch = strings.Trim(out.String(), "\n")
	// In case of tag, branch output is HEAD.
	if branch == "HEAD" {
		return ""
	}
	return branch
}

func guessGitTag(c util.Settings, e *util.Environment) string {
	tag, _ := c.String("git-tag")
	if tag != "" {
		return tag
	}
	projectPath := guessProjectPath(c, e)
	if projectPath == "" {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	defer os.Chdir(cwd)
	os.Chdir(projectPath)

	git, err := exec.LookPath("git")
	if err != nil {
		return ""
	}

	var out bytes.Buffer
	cmd := exec.Command(git, "tag", "--points-at", "HEAD")
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return ""
	}

	cmdOut := strings.TrimSuffix(out.String(), "\n")
	arr := strings.Split(cmdOut, "\n")

	return arr[len(arr)-1]
}

func guessGitCommit(c util.Settings, e *util.Environment) string {
	commit, _ := c.String("git-commit")
	if commit != "" {
		return commit
	}

	projectPath := guessProjectPath(c, e)
	if projectPath == "" {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	defer os.Chdir(cwd)
	os.Chdir(projectPath)

	git, err := exec.LookPath("git")
	if err != nil {
		return ""
	}

	var out bytes.Buffer
	cmd := exec.Command(git, "rev-parse", "HEAD")
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return ""
	}
	return strings.Trim(out.String(), "\n")
}

func guessGitOwner(c util.Settings, e *util.Environment) string {
	owner, _ := c.String("git-owner")
	if owner != "" {
		return owner
	}

	u, err := user.Current()
	if err == nil {
		owner = u.Username
	}
	return owner
}

func guessGitRepository(c util.Settings, e *util.Environment) string {
	repository, _ := c.String("git-repository")
	if repository != "" {
		return repository
	}
	// repository, err := guessApplicationName(c, env)
	// if err != nil {
	//   return ""
	// }
	return repository
}

// NewGitOptions constructor
func NewGitOptions(c util.Settings, e *util.Environment, globalOpts *GlobalOptions) (*GitOptions, error) {
	gitBranch := guessGitBranch(c, e)
	gitTag := guessGitTag(c, e)
	gitCommit := guessGitCommit(c, e)
	gitDomain, _ := c.String("git-domain")
	gitOwner := guessGitOwner(c, e)
	gitRepository := guessGitRepository(c, e)

	return &GitOptions{
		GlobalOptions: globalOpts,
		GitBranch:     gitBranch,
		GitTag:        gitTag,
		GitCommit:     gitCommit,
		GitDomain:     gitDomain,
		GitOwner:      gitOwner,
		GitRepository: gitRepository,
	}, nil
}

// ReporterOptions for our reporting
type ReporterOptions struct {
	*GlobalOptions
	ReporterHost string
	ReporterKey  string
	ShouldReport bool
}

// NewReporterOptions constructor
func NewReporterOptions(c util.Settings, e *util.Environment, globalOpts *GlobalOptions) (*ReporterOptions, error) {
	shouldReport, _ := c.Bool("report")
	reporterHost, _ := c.String("wercker-host")
	reporterKey, _ := c.String("wercker-token")

	if shouldReport {
		if reporterKey == "" {
			return nil, errors.New("wercker-token is required")
		}

		if reporterHost == "" {
			return nil, errors.New("wercker-host is required")
		}
	}

	return &ReporterOptions{
		GlobalOptions: globalOpts,
		ReporterHost:  reporterHost,
		ReporterKey:   reporterKey,
		ShouldReport:  shouldReport,
	}, nil
}

func werckerContainerRegistry(c util.Settings) (*url.URL, error) {
	containerRegistry, _ := c.String("wercker-container-registry")
	containerRegistryURL, err := url.Parse(containerRegistry)
	if err != nil {
		return nil, fmt.Errorf("Container Registry URL is not well-formatted: %v", err)
	}
	return containerRegistryURL, nil
}

// PipelineOptions for builds and deploys
type PipelineOptions struct {
	*GlobalOptions
	*AWSOptions
	*OCIOptions
	// *DockerOptions
	*GitOptions
	*ReporterOptions

	// TODO(termie): i'd like to remove this, it is only used in a couple
	//               places by BasePipeline
	HostEnv *util.Environment

	RunID             string
	DeployTarget      string
	Pipeline          string
	DockerNetworkName string

	ApplicationID            string
	ApplicationName          string
	ApplicationOwnerName     string
	ApplicationStartedByName string

	WerckerContainerRegistry *url.URL

	ShouldCommit   bool
	Repository     string
	Tag            string
	Message        string
	ShouldStoreS3  bool
	ShouldStoreOCI bool

	// will be true if either ShouldStoreS3 or ShouldStoreOCI it true
	ShouldStore bool

	WorkingDir string

	GuestRoot  string
	MntRoot    string
	ReportRoot string
	// will be set by pipeline when it initializes
	PipelineBasePath string

	ProjectID   string
	ProjectURL  string
	ProjectPath string

	// Used when running workflows with fan-in locally.
	ProjectPathsByPipeline map[string]string

	CommandTimeout    int
	NoResponseTimeout int
	ShouldArtifacts   bool
	ShouldRemove      bool
	SourceDir         string
	IgnoreFile        string

	AttachOnError  bool
	DirectMount    bool
	EnableDevSteps bool
	PublishPorts   []string
	ExposePorts    bool
	EnableVolumes  bool
	WerckerYml     string
	Checkpoint     string

	DefaultsUsed PipelineDefaultsUsed

	WorkflowsInYml    bool
	SuppressBuildLogs bool
}

type PipelineDefaultsUsed struct {
	IgnoreFile bool
}

func guessApplicationID(c util.Settings, e *util.Environment, name string) string {
	id, _ := c.String("application-id")
	if id == "" {
		id = name
	}
	return id
}

// Some logic to guess the application name
func guessApplicationName(c util.Settings, e *util.Environment) (string, error) {
	applicationName, _ := c.String("application-name")
	if applicationName != "" {
		return applicationName, nil
	}

	// Otherwise, check our build target, it can be a url...
	target, _ := c.String("target")
	projectURL := ""
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		projectURL = target
		base := path.Base(projectURL)
		// Special handling for github tarballs
		if base == "tarball" {
			base = path.Base(path.Dir(projectURL))
		}
		ext := path.Ext(base)
		base = base[:len(ext)]
		return base, nil
	}

	// ... or a file path
	if target == "" {
		target = "."
	}
	stat, err := os.Stat(target)
	if err != nil || !stat.IsDir() {
		return "", fmt.Errorf("target '%s' is not a directory", target)
	}
	abspath, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	return filepath.Base(abspath), nil
}

func guessApplicationOwnerName(c util.Settings, e *util.Environment) string {
	name, _ := c.String("application-owner-name")
	if name == "" {
		u, err := user.Current()
		if err == nil {
			name = u.Username
		}
	}
	if name == "" {
		name = "wercker"
	}
	return name
}

func guessMessage(c util.Settings, e *util.Environment) string {
	message, _ := c.String("message")
	return message
}

func guessTag(c util.Settings, e *util.Environment) string {
	tag, _ := c.String("tag")
	if tag == "" {
		tag = guessGitBranch(c, e)
	}
	tag = strings.Replace(tag, "/", "_", -1)
	return tag
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func guessProjectID(c util.Settings, e *util.Environment) string {
	projectID, _ := c.String("project-id")
	if projectID != "" {
		return projectID
	}

	// If this was going to fail it already failed and we exited
	name, _ := guessApplicationName(c, e)
	return name
}

func guessProjectPath(c util.Settings, e *util.Environment) string {
	target, _ := c.String("target")
	if looksLikeURL(target) {
		return ""
	}
	if target == "" {
		target = "."
	}
	abs, _ := filepath.Abs(target)
	return abs
}

func guessProjectURL(c util.Settings, e *util.Environment) string {
	target, _ := c.String("target")
	if !looksLikeURL(target) {
		return ""
	}
	return target
}

// NewPipelineOptions big-ass constructor
func NewPipelineOptions(c util.Settings, e *util.Environment) (*PipelineOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}

	// dockerOpts, err := NewDockerOptions(c, e, globalOpts)
	// if err != nil {
	//   return nil, err
	// }

	awsOpts, err := NewAWSOptions(c, e, globalOpts)
	if err != nil {
		return nil, err
	}

	ociOpts, err := NewOCIOptions(c, e, globalOpts)
	if err != nil {
		return nil, err
	}

	gitOpts, err := NewGitOptions(c, e, globalOpts)
	if err != nil {
		return nil, err
	}

	reporterOpts, err := NewReporterOptions(c, e, globalOpts)
	if err != nil {
		return nil, err
	}

	runID, _ := c.String("run-id")

	deployTarget, _ := c.String("deploy-target")
	pipeline, _ := c.String("pipeline")

	applicationName, err := guessApplicationName(c, e)
	if err != nil {
		return nil, err
	}
	applicationID := guessApplicationID(c, e, applicationName)
	applicationOwnerName := guessApplicationOwnerName(c, e)
	applicationStartedByName, _ := c.String("application-started-by-name")
	if applicationStartedByName == "" {
		applicationStartedByName = applicationOwnerName
	}

	containerRegistry, _ := c.String("wercker-container-registry")
	containerRegistryURL, err := url.Parse(containerRegistry)
	if err != nil {
		return nil, fmt.Errorf("Container Registry URL is not well-formatted: %v", err)
	}

	repository, _ := c.String("commit")
	shouldCommit := (repository != "")
	tag := guessTag(c, e)
	message := guessMessage(c, e)
	shouldStoreS3, _ := c.Bool("store-s3")
	shouldStoreOCI, _ := c.Bool("store-oci")
	shouldStore := shouldStoreS3 || shouldStoreOCI

	workingDir, _ := c.String("working-dir")
	workingDir, _ = filepath.Abs(workingDir)

	guestRoot, _ := c.String("guest-root")
	mntRoot, _ := c.String("mnt-root")
	reportRoot, _ := c.String("report-root")

	projectID := guessProjectID(c, e)
	projectPath := guessProjectPath(c, e)
	projectURL := guessProjectURL(c, e)

	if projectPath == workingDir {
		return nil, fmt.Errorf("Project path can't be the same as the working dir")
	}

	// These timeouts are given in minutes but we store them as milliseconds
	commandTimeoutFloat, _ := c.Float64("command-timeout")
	commandTimeout := int(commandTimeoutFloat * 1000 * 60)
	noResponseTimeoutFloat, _ := c.Float64("no-response-timeout")
	noResponseTimeout := int(noResponseTimeoutFloat * 1000 * 60)
	shouldArtifacts, _ := c.Bool("artifacts")
	// TODO(termie): switch negative flag
	shouldRemove, _ := c.Bool("no-remove")
	shouldRemove = !shouldRemove
	sourceDir, _ := c.String("source-dir")
	ignoreFile, ignoreFileSet := c.String("ignore-file")

	attachOnError, _ := c.Bool("attach-on-error")
	directMount, _ := c.Bool("direct-mount")
	enableDevSteps, _ := c.Bool("enable-dev-steps")
	suppressBuildLogs, _ := c.Bool("suppress-build-logs")
	// Deprecated
	publishPorts, _ := c.StringSlice("publish")
	exposePorts, _ := c.Bool("expose-ports")
	enableVolumes, _ := c.Bool("enable-volumes")
	werckerYml, _ := c.String("wercker-yml")
	checkpoint, _ := c.String("checkpoint")

	defaultsUsed := PipelineDefaultsUsed{
		IgnoreFile: !ignoreFileSet,
	}

	workflowsInYml, _ := c.Bool("workflows-in-yml")

	return &PipelineOptions{
		GlobalOptions: globalOpts,
		AWSOptions:    awsOpts,
		OCIOptions:    ociOpts,
		// DockerOptions:   dockerOpts,
		GitOptions:      gitOpts,
		ReporterOptions: reporterOpts,

		HostEnv: e,

		RunID:        runID,
		DeployTarget: deployTarget,
		Pipeline:     pipeline,

		ApplicationID:            applicationID,
		ApplicationName:          applicationName,
		ApplicationOwnerName:     applicationOwnerName,
		ApplicationStartedByName: applicationStartedByName,

		Message:        message,
		Tag:            tag,
		Repository:     repository,
		ShouldCommit:   shouldCommit,
		ShouldStoreS3:  shouldStoreS3,
		ShouldStoreOCI: shouldStoreOCI,
		ShouldStore:    shouldStore,

		WorkingDir: workingDir,

		GuestRoot:  guestRoot,
		MntRoot:    mntRoot,
		ReportRoot: reportRoot,

		ProjectID:   projectID,
		ProjectURL:  projectURL,
		ProjectPath: projectPath,

		WerckerContainerRegistry: containerRegistryURL,

		CommandTimeout:    commandTimeout,
		NoResponseTimeout: noResponseTimeout,
		ShouldArtifacts:   shouldArtifacts,
		ShouldRemove:      shouldRemove,
		SourceDir:         sourceDir,
		IgnoreFile:        ignoreFile,

		AttachOnError:     attachOnError,
		DirectMount:       directMount,
		EnableDevSteps:    enableDevSteps,
		SuppressBuildLogs: suppressBuildLogs,
		// Deprecated
		PublishPorts:  publishPorts,
		ExposePorts:   exposePorts,
		EnableVolumes: enableVolumes,
		WerckerYml:    werckerYml,
		Checkpoint:    checkpoint,

		DefaultsUsed: defaultsUsed,

		WorkflowsInYml: workflowsInYml,
	}, nil
}

// HostPath returns a path relative to the build root on the host.
func (o *PipelineOptions) HostPath(s ...string) string {
	return path.Join(o.BuildPath(), o.RunID, path.Join(s...))
}

// WorkingPath returns paths relative to our working dir (usually ".wercker")
func (o *PipelineOptions) WorkingPath(s ...string) string {
	return path.Join(o.WorkingDir, path.Join(s...))
}

// GuestPath returns a path relative to the build root on the guest.
func (o *PipelineOptions) GuestPath(s ...string) string {
	return path.Join(o.GuestRoot, path.Join(s...))
}

func (o *PipelineOptions) BasePath() string {
	basePath := o.GuestPath("source")
	if o.PipelineBasePath != "" {
		basePath = o.PipelineBasePath
	}
	return basePath
}

func (o *PipelineOptions) SourcePath() string {
	return path.Join(o.BasePath(), o.SourceDir)
}

func (o *PipelineOptions) WorkflowURL() string {
	return fmt.Sprintf("%s/%s/%s/runs/%s/%s", o.BaseURL, o.ApplicationOwnerName, o.ApplicationName, o.Pipeline, o.RunID)
}

// MntPath returns a path relative to the read-only mount root on the guest.
func (o *PipelineOptions) MntPath(s ...string) string {
	return path.Join(o.MntRoot, path.Join(s...))
}

// ReportPath returns a path relative to the report root on the guest.
func (o *PipelineOptions) ReportPath(s ...string) string {
	return path.Join(o.ReportRoot, path.Join(s...))
}

// ContainerPath returns the path where exported containers live
func (o *PipelineOptions) ContainerPath() string {
	return path.Join(o.WorkingDir, "containers")
}

// BuildPath returns the path where created builds live
func (o *PipelineOptions) BuildPath(s ...string) string {
	return path.Join(o.WorkingDir, "builds", path.Join(s...))
}

// CachePath returns the path for storing pipeline cache
func (o *PipelineOptions) CachePath() string {
	return path.Join(o.WorkingDir, "cache")
}

// ProjectDownloadPath returns the path where downloaded projects live
func (o *PipelineOptions) ProjectDownloadPath() string {
	return path.Join(o.WorkingDir, "projects")
}

// StepPath returns the path where downloaded steps live
func (o *PipelineOptions) StepPath() string {
	return path.Join(o.WorkingDir, "steps")
}

// IgnoreFilePath return the absolute path of the ignore file
func (o *PipelineOptions) IgnoreFilePath() string {
	expandedIgnoreFile := util.ExpandHomePath(o.IgnoreFile, o.HostEnv.Get("HOME"))

	if filepath.IsAbs(expandedIgnoreFile) {
		return expandedIgnoreFile
	} else {
		return path.Join(o.ProjectPath, o.IgnoreFile)
	}
}

// Options per Command

type optionsGetter func(*cli.Context, *util.Environment) (*PipelineOptions, error)

// NewBuildOptions constructor
func NewBuildOptions(c util.Settings, e *util.Environment) (*PipelineOptions, error) {
	pipelineOpts, err := NewPipelineOptions(c, e)
	if err != nil {
		return nil, err
	}
	if pipelineOpts.RunID == "" {
		pipelineOpts.RunID = bson.NewObjectId().Hex()
	}
	return pipelineOpts, nil
}

// NewDevOptions ctor
func NewDevOptions(c util.Settings, e *util.Environment) (*PipelineOptions, error) {
	pipelineOpts, err := NewBuildOptions(c, e)
	if err != nil {
		return nil, err
	}
	return pipelineOpts, nil
}

// NewCheckConfigOptions constructor
func NewCheckConfigOptions(c util.Settings, e *util.Environment) (*PipelineOptions, error) {
	pipelineOpts, err := NewPipelineOptions(c, e)
	if err != nil {
		return nil, err
	}
	return pipelineOpts, nil
}

// NewDeployOptions constructor
func NewDeployOptions(c util.Settings, e *util.Environment) (*PipelineOptions, error) {
	pipelineOpts, err := NewPipelineOptions(c, e)
	if err != nil {
		return nil, err
	}
	// default to last build output if none defined
	target, _ := c.String("target")
	if target == "" {
		latestPath := pipelineOpts.WorkingPath("latest", "output")
		found, err := util.Exists(latestPath)
		if err == nil && found {
			util.RootLogger().Println("No target specified, using recent build output.")
			pipelineOpts.ProjectPath, _ = filepath.Abs(latestPath)
		}
	}

	// if the deploy target path does not have a wercker.yml, use the current one
	werckerYml, _ := c.String("wercker-yml")
	if werckerYml == "" {
		found, _ := util.Exists(filepath.Join(pipelineOpts.ProjectPath, "wercker.yml"))
		if !found {
			pipelineOpts.WerckerYml = "./wercker.yml"
		}
	}

	if pipelineOpts.RunID == "" {
		pipelineOpts.RunID = uuid.NewRandom().String()
	}
	return pipelineOpts, nil
}

// DetectOptions for detect command
type DetectOptions struct {
	*GlobalOptions
}

// NewDetectOptions constructor
func NewDetectOptions(c util.Settings, e *util.Environment) (*DetectOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}
	return &DetectOptions{globalOpts}, nil
}

// InspectOptions for inspect command
type InspectOptions struct {
	*PipelineOptions
}

// NewInspectOptions constructor
func NewInspectOptions(c util.Settings, e *util.Environment) (*InspectOptions, error) {
	pipelineOpts, err := NewPipelineOptions(c, e)
	if err != nil {
		return nil, err
	}
	return &InspectOptions{pipelineOpts}, nil
}

// LoginOptions for the login command
type LoginOptions struct {
	*GlobalOptions
}

// NewLoginOptions constructor
func NewLoginOptions(c util.Settings, e *util.Environment) (*LoginOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}
	return &LoginOptions{globalOpts}, nil
}

// LogoutOptions for the login command
type LogoutOptions struct {
	*GlobalOptions
}

// NewLogoutOptions constructor
func NewLogoutOptions(c util.Settings, e *util.Environment) (*LogoutOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}
	return &LogoutOptions{globalOpts}, nil
}

// PullOptions for the pull command
type PullOptions struct {
	*GlobalOptions
	// *DockerOptions

	Repository string
	Branch     string
	Commit     string
	Status     string
	Result     string
	Output     string
	Load       bool
	Force      bool
}

// NewPullOptions constructor
func NewPullOptions(c util.Settings, e *util.Environment) (*PullOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}

	// dockerOpts, err := NewDockerOptions(c, e, globalOpts)
	// if err != nil {
	//   return nil, err
	// }

	repository, _ := c.String("target")
	output, _ := c.String("output")
	outputDir, err := filepath.Abs(output)
	if err != nil {
		return nil, err
	}
	branch, _ := c.String("branch")
	status, _ := c.String("status")
	result, _ := c.String("result")
	load, _ := c.Bool("load")
	force, _ := c.Bool("force")

	return &PullOptions{
		GlobalOptions: globalOpts,
		// DockerOptions: dockerOpts,

		Repository: repository,
		Branch:     branch,
		Status:     status,
		Result:     result,
		Output:     outputDir,
		Load:       load,
		Force:      force,
	}, nil
}

// VersionOptions contains the options associated with the version
// command.
type VersionOptions struct {
	OutputJSON     bool
	BetaChannel    bool
	CheckForUpdate bool
}

// NewVersionOptions constructor
func NewVersionOptions(c util.Settings, e *util.Environment) (*VersionOptions, error) {
	json, _ := c.Bool("json")
	beta, _ := c.Bool("beta")
	noUpdateCheck, _ := c.Bool("no-update-check")

	return &VersionOptions{
		OutputJSON:     json,
		BetaChannel:    beta,
		CheckForUpdate: !noUpdateCheck,
	}, nil
}

type WerckerDockerOptions struct {
	*GlobalOptions
	WerckerContainerRegistry *url.URL
}

func NewWerckerDockerOptions(c util.Settings, e *util.Environment) (*WerckerDockerOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}

	wcr, err := werckerContainerRegistry(c)
	if err != nil {
		return nil, err
	}

	return &WerckerDockerOptions{
		GlobalOptions:            globalOpts,
		WerckerContainerRegistry: wcr,
	}, nil
}

type WerckerStepOptions struct {
	*GlobalOptions
	Owner   string
	Private bool
	StepDir string
}

func NewWerckerStepOptions(c util.Settings, e *util.Environment) (*WerckerStepOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}

	owner, _ := c.String("owner")
	private, _ := c.Bool("private")

	return &WerckerStepOptions{
		GlobalOptions: globalOpts,
		Owner:         owner,
		Private:       private,
	}, nil
}

// WerckerRunnerOptions -
type WerckerRunnerOptions struct {
	*GlobalOptions
	RunnerName     string
	RunnerGroup    string
	RunnerOrgs     string
	RunnerApps     string
	Workflows      string
	StorePath      string
	LoggerPath     string
	BearerToken    string
	DockerEndpoint string
	ImageName      string
	NumRunners     int
	Polling        int
	AllOption      bool
	NoWait         bool
	PullRemote     bool
	Production     bool
	OCIOptions     *OCIOptions
	OCIDownload    string
}

// NewExternalRunnerOptions -
func NewExternalRunnerOptions(c util.Settings, e *util.Environment) (*WerckerRunnerOptions, error) {
	globalOpts, err := NewGlobalOptions(c, e)
	if err != nil {
		return nil, err
	}
	rname, _ := c.String("name")
	rgroup, _ := c.String("group")
	rorgs, _ := c.String("orgs")
	flows, _ := c.String("workflows")
	rapps, _ := c.String("apps")
	spath, _ := c.String("storepath")
	lpath, _ := c.String("logpath")
	norun, _ := c.Int("runners")
	token, _ := c.String("token")
	pfreq, _ := c.Int("poll-frequency")
	isall, _ := c.Bool("all")
	dhost, _ := c.String("docker-host")
	nwait, _ := c.Bool("nowait")
	pulls, _ := c.Bool("pull")
	image, _ := c.String("image-name")
	storeOCI, _ := c.Bool("store-oci")
	download, _ := c.String("oci-download")

	prod := false
	site, _ := c.String("using")
	if site == "prod" {
		prod = true
	}

	if dhost == "" {
		dhost = "unix:///var/run/docker.sock"
	}

	// Determine if OCI object store or the local file system will be used
	var ociOpts *OCIOptions
	if storeOCI {
		if spath != "" {
			// Make sure storepath is not specified as it will confuse kiddie-pool
			return nil, errors.New("--store-oci and --storepath are incapatible specified together")
		}
		ociOpts, err = NewOCIOptions(c, e, globalOpts)
		if err != nil {
			return nil, err
		}
	} else {
		if spath == "" {
			spath = "/tmp/wercker"
		}
		os.MkdirAll(spath, 0776)
	}

	return &WerckerRunnerOptions{
		GlobalOptions:  globalOpts,
		BearerToken:    token,
		RunnerName:     rname,
		RunnerGroup:    rgroup,
		RunnerOrgs:     rorgs,
		RunnerApps:     rapps,
		Workflows:      flows,
		StorePath:      spath,
		LoggerPath:     lpath,
		NumRunners:     norun,
		Polling:        pfreq,
		AllOption:      isall,
		NoWait:         nwait,
		DockerEndpoint: dhost,
		PullRemote:     pulls,
		Production:     prod,
		ImageName:      image,
		OCIOptions:     ociOpts,
		OCIDownload:    download,
	}, nil
}

// WorkflowOptions currently uses PipelineOptions
// along with its constructor for simplicity.
type WorkflowOptions struct {
	PipelineOptions PipelineOptions
	WorkflowName    string
}

// NewWorkflowOptions is a constructor for WorkflowOptions.
func NewWorkflowOptions(c util.Settings, e *util.Environment) (*WorkflowOptions, error) {
	pipelineOpts, err := NewPipelineOptions(c, e)
	if err != nil {
		return nil, err
	}

	return &WorkflowOptions{
		PipelineOptions: *pipelineOpts,
	}, nil
}
