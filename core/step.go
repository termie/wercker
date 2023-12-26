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

package core

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pborman/uuid"
	"github.com/wercker/wercker/api"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"

	shutil "github.com/termie/go-shutil"
	yaml "gopkg.in/yaml.v2"
)

// StepDesc represents a step.yml
type StepDesc struct {
	Name        string
	Version     string
	Description string
	Keywords    []string
	Properties  []StepDescProperty
}

// StepDescProperty is the structure of the values in the "properties"
// section of the config
type StepDescProperty struct {
	Name     string
	Default  string
	Required bool
	Type     string
}

// ReadStepDesc reads a file, expecting it to be parsed into a StepDesc.
func ReadStepDesc(descPath string) (*StepDesc, error) {
	file, err := ioutil.ReadFile(descPath)
	if err != nil {
		return nil, err
	}

	var m StepDesc
	err = yaml.Unmarshal(file, &m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Defaults returns the default properties for a step as a map.
func (sc *StepDesc) Defaults() map[string]string {
	m := make(map[string]string)
	if sc == nil || sc.Properties == nil {
		return m
	}
	for _, v := range sc.Properties {
		m[v.Name] = v.Default
	}
	return m
}

// Step interface for steps, to be renamed
type Step interface {
	// Bunch of getters
	DisplayName() string
	Env() *util.Environment
	Cwd() string
	ID() string
	Name() string
	Owner() string
	SafeID() string
	Version() string
	ShouldSyncEnv() bool
	Checkpoint() string

	// Actual methods
	Fetch() (string, error)

	InitEnv(context.Context, *util.Environment) error
	Execute(context.Context, *Session) (int, error)
	CollectFile(string, string, string, io.Writer) error
	CollectArtifact(context.Context, string) (*Artifact, error)
	// TODO(termie): don't think this needs to be universal
	ReportPath(...string) string
	Clean()
}

// BaseStepOptions are exported fields so that we can make a BaseStep from
// other packages, see: https://gist.github.com/termie/8b66a2b4206e8e042766
type BaseStepOptions struct {
	DisplayName string
	Env         *util.Environment
	ID          string
	Name        string
	Owner       string
	SafeID      string
	Version     string
	Cwd         string
	Checkpoint  string
}

// BaseStep type for extending
type BaseStep struct {
	displayName string
	env         *util.Environment
	id          string
	name        string
	owner       string
	safeID      string
	version     string
	cwd         string
	checkpoint  string
}

func NewBaseStep(args BaseStepOptions) *BaseStep {
	return &BaseStep{
		displayName: args.DisplayName,
		env:         args.Env,
		id:          args.ID,
		name:        args.Name,
		owner:       args.Owner,
		safeID:      args.SafeID,
		version:     args.Version,
		cwd:         args.Cwd,
		checkpoint:  args.Checkpoint,
	}
}

// DisplayName getter
func (s *BaseStep) DisplayName() string {
	return s.displayName
}

// Env getter
func (s *BaseStep) Env() *util.Environment {
	return s.env
}

// Cwd getter
func (s *BaseStep) Cwd() string {
	return s.cwd
}

// ID getter
func (s *BaseStep) ID() string {
	return s.id
}

// Name getter
func (s *BaseStep) Name() string {
	return s.name
}

// Owner getter
func (s *BaseStep) Owner() string {
	return s.owner
}

// SafeID getter
func (s *BaseStep) SafeID() string {
	return s.safeID
}

// Version getter
func (s *BaseStep) Version() string {
	return s.version
}

// Version getter
func (s *BaseStep) Checkpoint() string {
	return s.checkpoint
}

func (s *BaseStep) Clean() {

}

// ExternalStep is the holder of the Step methods.
type ExternalStep struct {
	*BaseStep
	url      string
	data     map[string]string
	stepDesc *StepDesc
	logger   *util.LogEntry
	options  *PipelineOptions
}

// NewStep sets up the basic parts of a Step.
// Step names can come in a couple forms (x means currently supported):
//   x setup-go-environment (fetches from api)
//   x wercker/hipchat-notify (fetches from api)
//   x wercker/hipchat-notify "http://someurl/thingee.tar" (downloads tarball)
//   x setup-go-environment "file:///some_path" (uses local path)
func NewStep(stepConfig *StepConfig, options *PipelineOptions) (*ExternalStep, error) {
	var identifier string
	var name string
	var owner string
	var version string

	url := ""

	stepID := stepConfig.ID
	data := stepConfig.Data

	// Check for urls
	_, err := fmt.Sscanf(stepID, "%s %q", &identifier, &url)
	if err != nil {
		// There was probably no url part
		identifier = stepID
	}

	// Check for owner/name
	parts := strings.SplitN(identifier, "/", 2)
	if len(parts) > 1 {
		owner = parts[0]
		name = parts[1]
	} else {
		// No owner, "wercker" is the default
		owner = "wercker"
		name = identifier
	}

	versionParts := strings.SplitN(name, "@", 2)
	if len(versionParts) == 2 {
		name = versionParts[0]
		version = versionParts[1]
	} else {
		version = "*"
	}

	// Add a random number to the name to prevent collisions on disk
	stepSafeID := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())

	// Script steps need unique IDs
	if name == "script" {
		stepID = uuid.NewRandom().String()
		version = util.Version()
	}

	// If there is a name in data, make it our displayName and delete it
	displayName := stepConfig.Name
	if displayName == "" {
		displayName = name
	}

	logger := util.RootLogger().WithFields(util.LogFields{
		"Logger": "Step",
		"SafeID": stepSafeID,
	})

	return &ExternalStep{
		BaseStep: &BaseStep{
			displayName: displayName,
			env:         util.NewEnvironment(),
			id:          identifier,
			name:        name,
			owner:       owner,
			safeID:      stepSafeID,
			version:     version,
			cwd:         stepConfig.Cwd,
			checkpoint:  stepConfig.Checkpoint,
		},
		options: options,
		data:    data,
		url:     url,
		logger:  logger,
	}, nil
}

// IsScript should probably not be exported.
func (s *ExternalStep) IsScript() bool {
	return s.name == "script"
}

func normalizeCode(code string) string {
	if !strings.HasPrefix(code, "#!") {
		code = strings.Join([]string{
			"set -e",
			code,
		}, "\n")
	}
	return code
}

// LocalSymlink makes sure we have an easy to use local symlink
func (s *ExternalStep) LocalSymlink() {
	name := strings.Replace(s.DisplayName(), " ", "-", -1)
	checkName := fmt.Sprintf("step-%s", name)
	checkPath := s.options.HostPath(checkName)

	counter := 1
	newPath := checkPath
	for {
		already, _ := util.Exists(newPath)
		if !already {
			os.Symlink(s.HostPath(), newPath)
			break
		}

		newPath = fmt.Sprintf("%s-%d", checkPath, counter)
		counter++
	}
}

// FetchScript turns the raw code in a step into a shell file.
func (s *ExternalStep) FetchScript() (string, error) {
	hostStepPath := s.options.HostPath(s.safeID)
	scriptPath := s.options.HostPath(s.safeID, "run.sh")
	content := normalizeCode(s.data["code"])

	err := os.MkdirAll(hostStepPath, 0755)
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(scriptPath, []byte(content), 0755)
	if err != nil {
		return "", err
	}

	return hostStepPath, nil
}

// Fetch grabs the Step content (or calls FetchScript for script steps).
func (s *ExternalStep) Fetch() (string, error) {
	// NOTE(termie): polymorphism based on kind, we could probably do something
	//               with interfaces here, but this is okay for now
	if s.IsScript() {
		return s.FetchScript()
	}

	stepPath := filepath.Join(s.options.StepPath(), s.CachedName())
	stepExists, err := util.Exists(stepPath)
	if err != nil {
		return "", err
	}

	if !stepExists {
		// If we don't have a url already

		var client api.StepRegistry
		// TODO(termie): probably don't need these in global options?
		if s.options.GlobalOptions.StepRegistryURL == "" {
			apiOptions := api.APIOptions{
				BaseURL: s.options.GlobalOptions.BaseURL,
			}
			// NOTE(kokaz): this client doesn't contain any auth token
			client = api.NewAPIClient(&apiOptions)
		} else {
			client = api.NewWerckerStepRegistry(s.options.GlobalOptions.StepRegistryURL, s.options.GlobalOptions.AuthToken)
		}

		if s.url == "" {
			// Grab the info about the step from the api

			stepInfo, err := client.GetStepVersion(s.Owner(), s.Name(), s.Version())
			if err != nil {
				if apiErr, ok := err.(*api.APIError); ok && apiErr.StatusCode == 404 {
					return "", fmt.Errorf("The step \"%s\" was not found", s.ID())
				}
				return "", err
			}

			s.url = stepInfo.TarballURL
		}

		// If we have a file uri let's just symlink it.
		if strings.HasPrefix(s.url, "file://") {
			if s.options.EnableDevSteps {
				localPath := s.url[len("file://"):]
				localPath, err = filepath.Abs(localPath)
				if err != nil {
					return "", err
				}
				os.MkdirAll(s.options.StepPath(), 0755)
				err = os.Symlink(localPath, stepPath)
				if err != nil {
					return "", err
				}
			} else {
				return "", fmt.Errorf("Dev mode is not enabled so refusing to copy local file urls: %s", s.url)
			}
		} else {
			// Grab the tarball and util.Untargzip it
			resp, err := client.GetTarball(s.url)
			if err != nil {
				return "", err
			}

			// Assuming we have a gzip'd tarball at this point
			err = util.Untargzip(stepPath, resp.Body)
			if err != nil {
				return "", err
			}
		}
	}

	hostStepPath := s.HostPath()

	err = shutil.CopyTree(stepPath, hostStepPath, nil)
	if err != nil {
		return "", nil
	}

	// Now that we have the code, load any step config we might find
	desc, err := ReadStepDesc(s.HostPath("step.yml"))
	if err != nil && !os.IsNotExist(err) {
		// TODO(termie): Log an error instead of printing
		s.logger.Println("ERROR: Reading step.yml:", err)
	}
	if err == nil {
		s.stepDesc = desc
	}
	return hostStepPath, nil
}

// SetupGuest ensures that the guest is ready to run a Step.
func (s *ExternalStep) SetupGuest(sessionCtx context.Context, sess *Session) error {
	defer s.LocalSymlink()

	// TODO(termie): can this even fail? i.e. exit code != 0
	sess.HideLogs()
	defer sess.ShowLogs()
	_, _, err := sess.SendChecked(sessionCtx, fmt.Sprintf(`mkdir -p "%s"`, s.ReportPath("artifacts")))
	_, _, err = sess.SendChecked(sessionCtx, "set +e")
	_, _, err = sess.SendChecked(sessionCtx, fmt.Sprintf(`cp -r "%s" "%s"`, s.MntPath(), s.GuestPath()))
	_, _, err = sess.SendChecked(sessionCtx, fmt.Sprintf(`cd $WERCKER_SOURCE_DIR`))
	if s.Cwd() != "" {
		_, _, err = sess.SendChecked(sessionCtx, fmt.Sprintf(`cd "%s"`, s.Cwd()))
	}
	return err
}

// Execute actually sends the commands for the step.
func (s *ExternalStep) Execute(sessionCtx context.Context, sess *Session) (int, error) {
	err := s.SetupGuest(sessionCtx, sess)
	if err != nil {
		return 1, err
	}
	_, _, err = sess.SendChecked(sessionCtx, s.env.Export()...)
	if err != nil {
		return 1, err
	}

	// if s.options.GlobalOptions.Verbose {
	// 	sess.SendChecked(sessionCtx, "set -xv")
	// }

	if yes, _ := util.Exists(s.HostPath("init.sh")); yes {
		exit, _, err := sess.SendChecked(sessionCtx, fmt.Sprintf(`source "%s"`, s.GuestPath("init.sh")))
		if exit != 0 {
			return exit, errors.New("Ack!")
		}
		if err != nil {
			return 1, err
		}
	}

	if yes, _ := util.Exists(s.HostPath("run.sh")); yes {
		exit, _, err := sess.SendChecked(sessionCtx, fmt.Sprintf(`source "%s" < /dev/null`, s.GuestPath("run.sh")))
		return exit, err
	}

	return 0, nil
}

// CollectFile noop
func (s *ExternalStep) CollectFile(containerID, path, name string, dst io.Writer) error {
	return util.ErrEmptyTarball
}

// CollectArtifact noop
func (s *ExternalStep) CollectArtifact(ctx context.Context, containerID string) (*Artifact, error) {
	return nil, nil
}

// InitEnv sets up the internal environment for the Step.
func (s *ExternalStep) InitEnv(ctx context.Context, env *util.Environment) error {
	a := [][]string{
		[]string{"WERCKER_STEP_ROOT", s.GuestPath()},
		[]string{"WERCKER_STEP_ID", s.safeID},
		[]string{"WERCKER_STEP_OWNER", s.owner},
		[]string{"WERCKER_STEP_NAME", s.name},
		[]string{"WERCKER_REPORT_NUMBERS_FILE", s.ReportPath("numbers.ini")},
		[]string{"WERCKER_REPORT_MESSAGE_FILE", s.ReportPath("message.txt")},
		[]string{"WERCKER_REPORT_ARTIFACTS_DIR", s.ReportPath("artifacts")},
	}
	s.Env().Update(a)

	defaults := s.stepDesc.Defaults()

	for k, defaultValue := range defaults {
		value, ok := s.data[k]
		key := fmt.Sprintf("WERCKER_%s_%s", s.name, k)
		key = strings.Replace(key, "-", "_", -1)
		key = strings.ToUpper(key)
		if !ok {
			s.Env().Add(key, defaultValue)
		} else {
			s.Env().Add(key, value)
		}
	}

	for k, value := range s.data {
		if k == "code" || k == "name" {
			continue
		}
		key := fmt.Sprintf("WERCKER_%s_%s", s.name, k)
		key = strings.Replace(key, "-", "_", -1)
		key = strings.ToUpper(key)
		s.Env().Add(key, value)
	}

	return nil
}

// CachedName returns a name suitable for caching
func (s *ExternalStep) CachedName() string {
	name := fmt.Sprintf("%s-%s", s.owner, s.name)
	if s.version != "*" {
		name = fmt.Sprintf("%s@%s", name, s.version)
	}
	return name
}

// HostPath returns a path relative to the Step on the host.
func (s *ExternalStep) HostPath(p ...string) string {
	newArgs := append([]string{s.safeID}, p...)
	return s.options.HostPath(newArgs...)
}

// GuestPath returns a path relative to the Step on the guest.
func (s *ExternalStep) GuestPath(p ...string) string {
	newArgs := append([]string{s.safeID}, p...)
	return s.options.GuestPath(newArgs...)
}

// MntPath returns a path relative to the read-only mount of the Step on
// the guest.
func (s *ExternalStep) MntPath(p ...string) string {
	newArgs := append([]string{s.safeID}, p...)
	return s.options.MntPath(newArgs...)
}

// ReportPath returns a path to the reports for the step on the guest.
func (s *ExternalStep) ReportPath(p ...string) string {
	newArgs := append([]string{s.safeID}, p...)
	return s.options.ReportPath(newArgs...)
}

// ShouldSyncEnv before this step, default FALSE
func (s *ExternalStep) ShouldSyncEnv() bool {
	return false
}
