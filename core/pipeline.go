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
	"path/filepath"
	"regexp"
	"strings"

	"github.com/wercker/wercker/util"

	"golang.org/x/net/context"
)

// ParseApplicationID parses input and returns the username and application
// name. A valid application ID is two strings separated by a /.
func ParseApplicationID(input string) (username, name string, err error) {
	split := strings.Split(input, "/")
	if len(split) == 2 {
		return split[0], split[1], nil
	}
	return "", "", errors.New("Unable to parse applicationID")
}

var buildRegex = regexp.MustCompile("^[0-9a-fA-F]{24}$")

// IsBuildID checks if input is a BuildID. BuildID is defined as a 24 character
// hex string.
func IsBuildID(input string) bool {
	return buildRegex.Match([]byte(input))
}

// Pipeline is a set of steps to run, this is the interface shared by
// both Build and Deploy
type Pipeline interface {
	// Getters
	Env() *util.Environment // base
	Box() Box               // base
	Services() []ServiceBox //base
	Steps() []Step          // base
	AfterSteps() []Step     // base

	// Methods
	CommonEnv() [][]string                      // base
	InitEnv(context.Context, *util.Environment) // impl
	CollectArtifact(context.Context, string) (*Artifact, error)
	CollectCache(context.Context, string) error
	LocalSymlink()
	SetupGuest(context.Context, *Session) error
	ExportEnvironment(context.Context, *Session) error
	SyncEnvironment(context.Context, *Session) error

	LogEnvironment()
	DockerRepo() string
	DockerTag() string
	DockerMessage() string
	//Docker() - returns true if the build requires a Remote Docker Daemon
	Docker() bool
}

// PipelineResult keeps track of the results of a build or deploy
// mostly so that we can use it to run after-steps
type PipelineResult struct {
	Success           bool
	FailedStepName    string
	FailedStepMessage string
}

// ExportEnvironment for this pipeline result (used in after-steps)
func (pr *PipelineResult) ExportEnvironment(sessionCtx context.Context, sess *Session) error {
	e := util.NewEnvironment()
	result := "failed"
	if pr.Success {
		result = "passed"
	}
	e.Add("WERCKER_RESULT", result)
	if !pr.Success {
		e.Add("WERCKER_FAILED_STEP_DISPLAY_NAME", pr.FailedStepName)
		e.Add("WERCKER_FAILED_STEP_MESSAGE", pr.FailedStepMessage)
	}

	exit, _, err := sess.SendChecked(sessionCtx, e.Export()...)
	if err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("Pipeline failed with exit code: %d", exit)
	}
	return nil
}

type BasePipelineOptions struct {
	Options    *PipelineOptions
	Config     *PipelineConfig
	Env        *util.Environment
	Box        Box
	Services   []ServiceBox
	Steps      []Step
	AfterSteps []Step
	Logger     *util.LogEntry
}

// BasePipeline is the base class for Build and Deploy
type BasePipeline struct {
	options    *PipelineOptions
	config     *PipelineConfig
	env        *util.Environment
	box        Box
	services   []ServiceBox
	steps      []Step
	afterSteps []Step
	logger     *util.LogEntry
}

func NewBasePipeline(args BasePipelineOptions) *BasePipeline {
	args.Options.PipelineBasePath = args.Config.BasePath
	return &BasePipeline{
		options:    args.Options,
		config:     args.Config,
		env:        args.Env,
		box:        args.Box,
		services:   args.Services,
		steps:      args.Steps,
		afterSteps: args.AfterSteps,
		logger:     args.Logger,
	}

}

// Box is a getter for the box
func (p *BasePipeline) Box() Box {
	return p.box
}

// Services is a getter for the Services
func (p *BasePipeline) Services() []ServiceBox {
	return p.services
}

// Steps is a getter for steps
func (p *BasePipeline) Steps() []Step {
	return p.steps
}

// AfterSteps is a getter for afterSteps
func (p *BasePipeline) AfterSteps() []Step {
	return p.afterSteps
}

// Env is a getter for env
func (p *BasePipeline) Env() *util.Environment {
	return p.env
}

// CommonEnv is shared by both builds and deploys
func (p *BasePipeline) CommonEnv() [][]string {
	a := [][]string{
		[]string{"WERCKER", "true"},
		[]string{"WERCKER_ROOT", p.options.GuestPath("source")},
		[]string{"WERCKER_SOURCE_DIR", p.options.SourcePath()},
		// TODO(termie): Support cache dir
		[]string{"WERCKER_CACHE_DIR", p.options.GuestPath("cache")},
		[]string{"WERCKER_OUTPUT_DIR", p.options.GuestPath("output")},
		[]string{"WERCKER_PIPELINE_DIR", p.options.GuestPath()},
		[]string{"WERCKER_REPORT_DIR", p.options.GuestPath("report")},
		[]string{"WERCKER_APPLICATION_ID", p.options.ApplicationID},
		[]string{"WERCKER_APPLICATION_NAME", p.options.ApplicationName},
		[]string{"WERCKER_APPLICATION_OWNER_NAME", p.options.ApplicationOwnerName},
		[]string{"WERCKER_APPLICATION_URL", fmt.Sprintf("%s/#applications/%s", p.options.BaseURL, p.options.ApplicationID)},
		//[]string{"WERCKER_STARTED_BY", ...},
		[]string{"TERM", "xterm-256color"},
	}
	return a
}

// SetupGuest ensures that the guest is prepared to run the pipeline.
func (p *BasePipeline) SetupGuest(sessionCtx context.Context, sess *Session) error {
	sess.HideLogs()
	defer sess.ShowLogs()

	timer := util.NewTimer()
	f := &util.Formatter{ShowColors: p.options.GlobalOptions.ShowColors}

	cmds := []string{}

	if !p.options.DirectMount {
		cmds = append(cmds,
			// Make sure our guest path exists
			fmt.Sprintf(`mkdir -p "%s"`, p.options.GuestPath()),
			// Make sure our base path exists
			fmt.Sprintf(`rm -rf "%s"`, filepath.Dir(p.options.BasePath())),
			fmt.Sprintf(`mkdir -p "%s"`, filepath.Dir(p.options.BasePath())),
			// Copy the source from the mounted directory to the base path
			fmt.Sprintf(`cp -r "%s" "%s"`, p.options.MntPath("source"), p.options.BasePath()),
			// Copy the cache from the mounted directory to the pipeline dir
			fmt.Sprintf(`cp -r "%s" "%s"`, p.options.MntPath("cache"), p.options.GuestPath("cache")),
		)
	}

	// Make sure the output path exists
	cmds = append(cmds, fmt.Sprintf(`mkdir -p "%s"`, p.options.GuestPath("output")))

	cmds = append(cmds, fmt.Sprintf(`chmod a+rx "%s"`, p.options.BasePath()))

	p.logger.Printf(f.Info("Copying source to container"))
	for _, cmd := range cmds {
		exit, _, err := sess.SendChecked(sessionCtx, cmd)
		if exit != 0 {
			return fmt.Errorf("Guest command failed with exit code %d: %s", exit, cmd)
		}
		if err != nil {
			return err
		}
	}
	if p.options.Verbose {
		p.logger.Printf(f.Success("Source+Cache -> Guest", timer.String()))
	}
	return nil
}

// ExportEnvironment to the session
func (p *BasePipeline) ExportEnvironment(sessionCtx context.Context, sess *Session) error {
	exit, _, err := sess.SendChecked(sessionCtx, p.Env().Export()...)
	if err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("Build failed with exit code: %d", exit)
	}
	// Export the hidden variables separately
	sess.HideLogs()
	defer sess.ShowLogs()
	exit, _, err = sess.SendChecked(sessionCtx, p.Env().Hidden.Export()...)
	if err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("Build failed with exit code: %d", exit)
	}
	return nil
}

// LogEnvironment dumps the base environment
func (p *BasePipeline) LogEnvironment() {
	p.logger.Debugln("Base Pipeline Environment:")
	for _, pair := range p.env.Ordered() {
		p.logger.Debugln(" ", pair[0], pair[1])
	}
}

// SyncEnvironment fetches the current environment from sess, and merges the
// result with p.env. This requires the `env` command to be available on the
// container.
func (p *BasePipeline) SyncEnvironment(sessionCtx context.Context, sess *Session) error {
	p.logger.Debugln("Syncing environment")
	var lines []string

	sess.HideLogs()
	defer sess.ShowLogs()

	// 'env' with --null parameter, which prevents issues from overlapping \n
	// inside the values.
	exit, output, err := sess.SendChecked(sessionCtx, "set +e", "env --null", "set -e")
	err = checkError(exit, output, err)
	if err != nil {
		return err
	}

	// send just 'env' if 'env --null' has failed (e.g. for alpine images)
	if strings.HasPrefix(output[0], "env: unrecognized option: null") {
		exit, output, err = sess.SendChecked(sessionCtx, "set +e", "env", "set -e")
		err = checkError(exit, output, err)
		if err != nil {
			return err
		}

		// Concat every output line into a single string, then split on the newline
		full := strings.Join(output, "")
		lines = strings.Split(full, "\n")
	} else {
		// Concat every output line into a single string, then split on the null byte
		full := strings.Join(output, "")
		lines = strings.Split(full, "\x00")
	}

	for _, line := range lines {
		if line == "" {
			continue
		}

		s := strings.SplitN(line, "=", 2)
		if len(s) != 2 {
			p.logger.Warnf("Unable to parse env line: \"%s\"", line)
			continue
		}

		key := s[0]
		value := s[1]

		p.env.Add(key, value)
	}

	return nil
}

//Docker - returns true if the build requires a Remote Docker Daemon
func (p *BasePipeline) Docker() bool {
	return p.config.Docker
}

func checkError(exit int, output []string, err error) error {
	if err != nil {
		return err
	}

	if exit != 0 {
		return fmt.Errorf("Unable to sync environment, exit code: %d", exit)
	}

	if output == nil || len(output) == 0 {
		return fmt.Errorf("No output returned by env command from container")
	}

	return nil
}
