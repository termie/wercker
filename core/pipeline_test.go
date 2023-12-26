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
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/util"
)

type PipelineSuite struct {
	*util.TestSuite
}

func TestPipelineSuite(t *testing.T) {
	suiteTester := &PipelineSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *PipelineSuite) TestParseApplicationIDValid() {
	applicationID := "wercker/foobar"

	username, name, err := ParseApplicationID(applicationID)

	s.Equal(nil, err)
	s.Equal("wercker", username)
	s.Equal("foobar", name)
}

func (s *PipelineSuite) TestParseApplicationIDInvalid() {
	applicationID := "foofoo"

	username, name, err := ParseApplicationID(applicationID)

	s.Error(err)
	s.Equal("", username)
	s.Equal("", name)
}

func (s *PipelineSuite) TestParseApplicationIDInvalid2() {
	applicationID := "wercker/foobar/bla"

	username, name, err := ParseApplicationID(applicationID)

	s.Error(err)
	s.Equal("", username)
	s.Equal("", name)
}

func (s *PipelineSuite) TestIsBuildIDValid() {
	buildID := "54e5dde34e104f675e007e3b"

	ok := IsBuildID(buildID)

	s.Equal(true, ok)
}

func (s *PipelineSuite) TestIsBuildIDInvalid() {
	buildID := "54e5dde34e104f675e007e3"

	ok := IsBuildID(buildID)

	s.Equal(false, ok)
}

func (s *PipelineSuite) TestIsBuildIDInvalid2() {
	buildID := "invalidinvalidinvalidinv"

	ok := IsBuildID(buildID)

	s.Equal(false, ok)
}

func (s *PipelineSuite) TestIsBuildIDInvalid3() {
	buildID := "invalid"

	ok := IsBuildID(buildID)

	s.Equal(false, ok)
}
