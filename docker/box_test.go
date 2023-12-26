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
	"testing"

	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

func boxByID(s string) (core.Box, error) {
	ctx := context.Background()
	settings := util.NewCheapSettings(nil)
	env := util.NewEnvironment()
	dockerOptions, err := NewOptions(ctx, settings, env)
	if err != nil {
		return nil, err
	}
	return NewDockerBox(
		&core.BoxConfig{ID: s},
		core.EmptyPipelineOptions(),
		dockerOptions,
	)
}

type BoxSuite struct {
	*util.TestSuite
}

func TestBoxSuite(t *testing.T) {
	suiteTester := &BoxSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *BoxSuite) TestName() {
	_, err := boxByID("wercker/base@1.0.0")
	s.NotNil(err)

	noTag, err := boxByID("wercker/base")
	s.Nil(err)
	s.Equal("wercker/base:latest", noTag.GetName())

	withTag, err := boxByID("wercker/base:foo")
	s.Nil(err)
	s.Equal("wercker/base:foo", withTag.GetName())
}

func (s *BoxSuite) TestPortBindings() {
	published := []string{
		"8000",
		"8001:8001",
		"127.0.0.1::8002",
		"127.0.0.1:8004:8003/udp",
	}
	checkBindings := [][]string{
		[]string{"8000/tcp", "", "8000"},
		[]string{"8001/tcp", "", "8001"},
		[]string{"8002/tcp", "127.0.0.1", "8002"},
		[]string{"8003/udp", "127.0.0.1", "8004"},
	}

	bindings := portBindings(published)
	s.Equal(len(checkBindings), len(bindings))
	for _, check := range checkBindings {
		binding := bindings[docker.Port(check[0])]
		s.Equal(check[1], binding[0].HostIP)
		s.Equal(check[2], binding[0].HostPort)
	}
}
