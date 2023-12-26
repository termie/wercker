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

package dockerlocal

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

type ArtifactSuite struct {
	*util.TestSuite
}

func TestArtifactSuite(t *testing.T) {
	suiteTester := &ArtifactSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *ArtifactSuite) TestDockerFileCollectorSingle() {
	ctx := context.Background()
	client := DockerOrSkip(ctx, s.T())

	container, err := TempBusybox(ctx, client)
	s.Nil(err)
	defer container.Remove(ctx)

	dfc := NewDockerFileCollector(client, container.ID)

	archive, err := dfc.Collect(ctx, "/etc/alpine-release")
	s.Nil(err)

	var b bytes.Buffer
	err = <-archive.SingleBytes("alpine-release", &b)
	s.Nil(err)

	s.Equal("3.1.4\n", b.String())
}

func (s *ArtifactSuite) TestDockerFileCollectorSingleNotFound() {
	ctx := context.Background()
	client := DockerOrSkip(ctx, s.T())

	container, err := TempBusybox(ctx, client)
	s.Nil(err)
	defer container.Remove(ctx)

	dfc := NewDockerFileCollector(client, container.ID)

	// Fail first from docker client
	archive1, err1 := dfc.Collect(ctx, "/notfound/file")
	s.Equal(err1, util.ErrEmptyTarball)
	if err1 == nil {
		defer archive1.Close()
	}
	// Or from archive
	archive2, err2 := dfc.Collect(ctx, "/etc/issue") // this file exists
	s.Nil(err2)
	if err2 == nil {
		defer archive2.Close()
	}
	var b2 bytes.Buffer
	err3 := <-archive2.SingleBytes("notfound", &b2) // this does not exist
	s.Equal(util.ErrEmptyTarball, err3)
}

func (s *ArtifactSuite) TestDockerFileCollectorMulti() {
	ctx := context.Background()
	client := DockerOrSkip(ctx, s.T())

	container, err1 := TempBusybox(ctx, client)
	if err1 != nil {
		println(err1.Error())
	}
	s.Nil(err1)
	defer container.Remove(ctx)

	dfc := NewDockerFileCollector(client, container.ID)

	archive, err2 := dfc.Collect(ctx, "/etc/apk")
	s.Nil(err2)
	defer archive.Close()

	var b bytes.Buffer
	err3 := <-archive.SingleBytes("apk/arch", &b)
	s.Nil(err3)

	check := "x86_64\n"
	s.Equal(check, b.String())
}

func (s *ArtifactSuite) TestDockerFileCollectorMultiEmptyTarball() {
	ctx := context.Background()
	client := DockerOrSkip(ctx, s.T())

	container, err1 := TempBusybox(ctx, client)
	s.Nil(err1)
	defer container.Remove(ctx)

	dfc := NewDockerFileCollector(client, container.ID)

	archive, err2 := dfc.Collect(ctx, "/var/tmp")
	s.Nil(err2)
	defer archive.Close()

	tmp, err3 := ioutil.TempDir("", "test-")
	s.Nil(err3)
	defer os.RemoveAll(tmp)

	err4 := <-archive.Multi("tmp", tmp, maxArtifactSize)
	s.Equal(err4, util.ErrEmptyTarball)
}

func (s *ArtifactSuite) TestDockerFileCollectorMultiNotFound() {
	ctx := context.Background()
	client := DockerOrSkip(ctx, s.T())

	container, err1 := TempBusybox(ctx, client)
	s.Nil(err1)
	defer container.Remove(ctx)

	dfc := NewDockerFileCollector(client, container.ID)

	archive, err2 := dfc.Collect(ctx, "/notfound")
	s.Equal(err2, util.ErrEmptyTarball)
	if err2 == nil {
		defer archive.Close()
	}
	tmp, err3 := ioutil.TempDir("", "test-")
	s.Nil(err3)
	defer os.RemoveAll(tmp)

	// case <-archive.Multi("default", tmp, maxArtifactSize):
	// 	s.FailNow()
	// case err := <-errs:
	// 	s.Equal(err, util.ErrEmptyTarball)
	// }
}
