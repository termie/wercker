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

package core

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/util"
)

func DefaultTestPipelineOptions(s *util.TestSuite, more map[string]interface{}) *PipelineOptions {
	overrides := map[string]interface{}{
		"debug": true,
		// "target":      "test",
		"working-dir": s.WorkingDir(),
	}
	for k, v := range more {
		overrides[k] = v
	}

	settings := util.NewCheapSettings(overrides)

	options, err := NewPipelineOptions(settings, util.NewEnvironment())
	if err != nil {
		s.Error(err)
	}
	return options
}

type StepSuite struct {
	*util.TestSuite
}

func TestStepSuite(t *testing.T) {
	suiteTester := &StepSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *StepSuite) TestFetchApi() {
	options := DefaultTestPipelineOptions(s.TestSuite, nil)

	cfg := &StepConfig{
		ID:   "create-file",
		Data: map[string]string{"filename": "foo.txt", "content": "bar"},
	}

	step, err := NewStep(cfg, options)
	s.Nil(err)
	_, err = step.Fetch()
	s.Nil(err)
}

func (s *StepSuite) TestFetchTar() {
	options := DefaultTestPipelineOptions(s.TestSuite, nil)

	werckerInit := `wercker-init "https://github.com/wercker/wercker-init/archive/v1.0.0.tar.gz"`
	cfg := &StepConfig{ID: werckerInit, Data: make(map[string]string)}

	step, err := NewStep(cfg, options)
	s.Nil(err)
	_, err = step.Fetch()
	s.Nil(err)
}

func (s *StepSuite) TestFetchFileNoDev() {
	options := DefaultTestPipelineOptions(s.TestSuite, nil)

	tmpdir, err := ioutil.TempDir("", "wercker")
	s.Nil(err)
	defer os.RemoveAll(tmpdir)

	fileStep := fmt.Sprintf(`foo "file:///%s"`, tmpdir)
	cfg := &StepConfig{ID: fileStep, Data: make(map[string]string)}

	step, err := NewStep(cfg, options)
	s.Nil(err)
	_, err = step.Fetch()
	s.NotNil(err)
}

func (s *StepSuite) TestFetchFileDev() {
	options := DefaultTestPipelineOptions(s.TestSuite, map[string]interface{}{
		"enable-dev-steps": true,
	})

	tmpdir, err := ioutil.TempDir("", "wercker")
	s.Nil(err)
	defer os.RemoveAll(tmpdir)
	os.MkdirAll(filepath.Join(options.WorkingDir, "steps"), 0777)

	fileStep := fmt.Sprintf(`foo "file:///%s"`, tmpdir)
	cfg := &StepConfig{ID: fileStep, Data: make(map[string]string)}

	step, err := NewStep(cfg, options)
	s.Nil(err)
	_, err = step.Fetch()
	s.Nil(err)
}
