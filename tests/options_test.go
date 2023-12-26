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

package tests

import (
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/cmd"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	cli "gopkg.in/urfave/cli.v1"
)

var (
	globalFlags   = cmd.FlagsFor(cmd.GlobalFlagSet)
	pipelineFlags = cmd.FlagsFor(cmd.PipelineFlagSet, cmd.WerckerInternalFlagSet)
	emptyFlags    = []cli.Flag{}
)

func emptyEnv() *util.Environment {
	return util.NewEnvironment()
}

func emptyPipelineOptions() *core.PipelineOptions {
	return &core.PipelineOptions{GlobalOptions: &core.GlobalOptions{}}
}

func run(s suite.TestingSuite, gFlags []cli.Flag, cFlags []cli.Flag, action func(c *cli.Context), args []string) {
	util.RootLogger().SetLevel("debug")
	os.Clearenv()
	app := cli.NewApp()
	app.Flags = gFlags
	app.Commands = []cli.Command{
		{
			Name:      "test",
			ShortName: "t",
			Usage:     "test command",
			Action:    action,
			Flags:     cFlags,
		},
	}
	app.CommandNotFound = func(c *cli.Context, command string) {
		s.T().Fatalf("Command not found: %s", command)
	}
	app.Action = func(c *cli.Context) {
		s.T().Fatal("No command specified")
	}
	app.Run(args)
}

func defaultArgs(more ...string) []string {
	args := []string{
		"wercker",
		"--debug",
		"--wercker-endpoint", "http://example.com/wercker-endpoint",
		"--base-url", "http://example.com/base-url/",
		"--auth-token", "test-token",
		"--auth-token-store", "/tmp/.wercker/test-token",
		"test",
	}
	return append(args, more...)
}

type OptionsSuite struct {
	*util.TestSuite
}

func TestOptionsSuite(t *testing.T) {
	suiteTester := &OptionsSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *OptionsSuite) TestGlobalOptions() {
	args := defaultArgs()
	test := func(c *cli.Context) {
		opts, err := core.NewGlobalOptions(util.NewCLISettings(c), emptyEnv())
		s.Nil(err)
		s.Equal(true, opts.Debug)
		s.Equal("http://example.com/base-url", opts.BaseURL)
		s.Equal("test-token", opts.AuthToken)
		s.Equal("/tmp/.wercker/test-token", opts.AuthTokenStore)
	}
	run(s, globalFlags, emptyFlags, test, args)
}

func (s *OptionsSuite) TestGuessAuthToken() {
	tmpFile, err := ioutil.TempFile("", "test-auth-token")
	s.Nil(err)

	token := uuid.NewRandom().String()
	_, err = tmpFile.Write([]byte(token))
	s.Nil(err)

	tokenStore := tmpFile.Name()
	defer os.Remove(tokenStore)
	defer tmpFile.Close()

	args := []string{
		"wercker",
		"--auth-token-store", tokenStore,
		"test",
	}

	test := func(c *cli.Context) {
		opts, err := core.NewGlobalOptions(util.NewCLISettings(c), emptyEnv())
		s.Nil(err)
		s.Equal(token, opts.AuthToken)
	}

	run(s, globalFlags, emptyFlags, test, args)
}

func (s *OptionsSuite) TestEmptyPipelineOptionsEmptyDir() {
	tmpDir, err := ioutil.TempDir("", "empty-directory")
	s.Nil(err)
	defer os.RemoveAll(tmpDir)

	basename := filepath.Base(tmpDir)
	currentUser, err := user.Current()
	s.Nil(err)
	username := currentUser.Username

	// Target the path
	args := defaultArgs(tmpDir)
	test := func(c *cli.Context) {
		opts, err := core.NewPipelineOptions(util.NewCLISettings(c), emptyEnv())
		s.Nil(err)
		s.Equal(basename, opts.ApplicationID)
		s.Equal(basename, opts.ApplicationName)
		s.Equal(username, opts.ApplicationOwnerName)
		s.Equal(username, opts.ApplicationStartedByName)
		s.Equal(tmpDir, opts.ProjectPath)
		s.Equal(basename, opts.ProjectID)
		// Pretty much all the git stuff should be empty
		s.Equal("", opts.GitBranch)
		s.Equal("", opts.GitCommit)
		s.Equal("", opts.GitDomain)
		s.Equal(username, opts.GitOwner)
		s.Equal("", opts.GitRepository)
		cmd.DumpOptions(opts)
	}
	run(s, globalFlags, pipelineFlags, test, args)
}

func (s *OptionsSuite) TestEmptyBuildOptions() {
	args := defaultArgs()
	test := func(c *cli.Context) {
		opts, err := core.NewBuildOptions(util.NewCLISettings(c), emptyEnv())
		s.Nil(err)
		s.NotEqual("", opts.RunID)
		s.Equal(opts.RunID, opts.RunID)
	}
	run(s, globalFlags, pipelineFlags, test, args)
}

func (s *OptionsSuite) TestBuildOptions() {
	args := defaultArgs("--run-id", "fake-build-id")
	test := func(c *cli.Context) {
		opts, err := core.NewBuildOptions(util.NewCLISettings(c), emptyEnv())
		s.Nil(err)
		s.Equal("fake-build-id", opts.RunID)
	}
	run(s, globalFlags, pipelineFlags, test, args)
}

func (s *OptionsSuite) TestEmptyDeployOptions() {
	args := defaultArgs()
	test := func(c *cli.Context) {
		opts, err := core.NewDeployOptions(util.NewCLISettings(c), emptyEnv())
		s.Nil(err)
		s.NotEqual("", opts.RunID)
		s.Equal(opts.RunID, opts.RunID)
	}
	run(s, globalFlags, pipelineFlags, test, args)
}

func (s *OptionsSuite) TestDeployOptions() {
	args := defaultArgs("--run-id", "fake-deploy-id")
	test := func(c *cli.Context) {
		opts, err := core.NewDeployOptions(util.NewCLISettings(c), emptyEnv())
		s.Nil(err)
		s.Equal("fake-deploy-id", opts.RunID)
	}
	run(s, globalFlags, pipelineFlags, test, args)
}

func (s *OptionsSuite) TestReporterOptions() {
	args := defaultArgs(
		"--report",
		"--wercker-host", "http://example.com/wercker-host",
		"--wercker-token", "test-token",
	)
	test := func(c *cli.Context) {
		e := emptyEnv()
		gOpts, err := core.NewGlobalOptions(util.NewCLISettings(c), e)
		opts, err := core.NewReporterOptions(util.NewCLISettings(c), e, gOpts)
		s.Nil(err)
		s.Equal(true, opts.ShouldReport)
		s.Equal("http://example.com/wercker-host", opts.ReporterHost)
		s.Equal("test-token", opts.ReporterKey)
	}
	run(s, globalFlags, pipelineFlags, test, args)
}

func (s *OptionsSuite) TestReporterMissingOptions() {
	test := func(c *cli.Context) {
		e := emptyEnv()
		gOpts, err := core.NewGlobalOptions(util.NewCLISettings(c), e)
		_, err = core.NewReporterOptions(util.NewCLISettings(c), e, gOpts)
		s.NotNil(err)
	}

	missingHost := defaultArgs(
		"--report",
		"--wercker-token", "test-token",
	)

	missingKey := defaultArgs(
		"--report",
		"--wercker-host", "http://example.com/wercker-host",
	)

	run(s, globalFlags, cmd.ReporterFlags, test, missingHost)
	run(s, globalFlags, cmd.ReporterFlags, test, missingKey)
}

func (s *OptionsSuite) TestTagEscaping() {
	args := defaultArgs("--tag", "feature/foo")
	test := func(c *cli.Context) {
		opts, err := core.NewPipelineOptions(util.NewCLISettings(c), emptyEnv())
		s.Nil(err)
		s.Equal("feature_foo", opts.Tag)
	}
	run(s, globalFlags, pipelineFlags, test, args)
}

func (s *OptionsSuite) TestWorkingDir() {
	tempDir, err := ioutil.TempDir("", "wercker-test-")
	s.Nil(err)
	defer os.RemoveAll(tempDir)

	args := defaultArgs("--working-dir", tempDir)

	test := func(c *cli.Context) {
		opts, err := core.NewPipelineOptions(util.NewCLISettings(c), emptyEnv())
		s.Nil(err)
		s.Equal(tempDir, opts.WorkingDir)
	}

	run(s, globalFlags, pipelineFlags, test, args)
}

func (s *OptionsSuite) TestWorkingDirCWD() {
	args := defaultArgs()
	cwd, err := filepath.Abs(".")
	s.Nil(err)

	test := func(c *cli.Context) {
		opts, err := core.NewPipelineOptions(util.NewCLISettings(c), emptyEnv())
		s.Nil(err)
		s.Equal(filepath.Join(cwd, ".wercker"), opts.WorkingDir)
	}

	run(s, globalFlags, pipelineFlags, test, args)
}
