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
package cmd

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

const initEnvErrorMessage = "InitEnv failed"

type RunnerSuite struct {
	*util.TestSuite
}

func TestRunnerSuite(t *testing.T) {
	suiteTester := &RunnerSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

//MockStep mocks Step
type MockStep struct {
	*core.BaseStep
}

// Mock methods not implemented by BaseStep
func (s *MockStep) CollectArtifact(context.Context, string) (*core.Artifact, error) {
	return nil, nil
}

func (s *MockStep) CollectFile(string, string, string, io.Writer) error {
	return nil
}

func (s *MockStep) Execute(context.Context, *core.Session) (int, error) {
	return 0, nil
}

func (s *MockStep) Fetch() (string, error) {
	return "", nil
}

func (s *MockStep) ReportPath(...string) string {
	return ""
}

func (s *MockStep) ShouldSyncEnv() bool {
	return false
}

func (s *MockStep) InitEnv(context.Context, *util.Environment) error {
	return errors.New(initEnvErrorMessage)
}

//MockPipeline mocks Pipeline
type MockPipeline struct {
	*core.BasePipeline
}

//Mock methods not implemented by BasePipeLine
func (s *MockPipeline) CollectArtifact(context.Context, string) (*core.Artifact, error) {
	return nil, nil
}

func (s *MockPipeline) CollectCache(context.Context, string) error {
	return nil
}

func (s *MockPipeline) DockerMessage() string {
	return ""
}

func (s *MockPipeline) DockerRepo() string {
	return ""
}

func (s *MockPipeline) DockerTag() string {
	return ""
}

func (s *MockPipeline) InitEnv(context.Context, *util.Environment) {

}

func (s *MockPipeline) LocalSymlink() {

}

func (s *MockPipeline) Env() *util.Environment {
	return nil
}

//TestRunnerStepFailedOnInitEnvError tests the scenario when a step in the pipleine
// will fail when an error occurs in initEnv() in step
func (s *RunnerSuite) TestRunnerStepFailedOnInitEnvError() {
	ctx := context.Background()
	mockPipeline := &MockPipeline{}
	shared := &RunnerShared{pipeline: mockPipeline}
	step := &MockStep{BaseStep: core.NewBaseStep(core.BaseStepOptions{ID: "MockID", Name: "MockStep", DisplayName: "MockStep"})}
	runner := &Runner{}
	runner.emitter = core.NewNormalizedEmitter()

	sr, err := runner.RunStep(ctx, shared, step, 1)
	s.Error(err)
	fmt.Println(err.Error())
	s.Contains(err.Error(), "Step initEnv failed with error message")
	s.Equal(sr.Message, initEnvErrorMessage)
	s.NotEqual(sr.ExitCode, 0)
}
