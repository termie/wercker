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

package dockerlocal

import (
	"io"

	"github.com/fsouza/go-dockerclient"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// DockerTransport for docker containers
type DockerTransport struct {
	options     *core.PipelineOptions
	client      *DockerClient
	containerID string
	logger      *util.LogEntry
}

// NewDockerTransport constructor
func NewDockerTransport(options *core.PipelineOptions, dockerOptions *Options, containerID string) (core.Transport, error) {
	client, err := NewDockerClient(dockerOptions)
	if err != nil {
		return nil, err
	}
	logger := util.RootLogger().WithField("Logger", "DockerTransport")
	return &DockerTransport{options: options, client: client, containerID: containerID, logger: logger}, nil
}

// Attach the given reader and writers to the transport, return a context
// that will be closed when the transport dies
func (t *DockerTransport) Attach(sessionCtx context.Context, stdin io.Reader, stdout, stderr io.Writer) (context.Context, error) {
	t.logger.Debugln("Attaching to container: ", t.containerID)
	started := make(chan struct{})
	transportCtx, cancel := context.WithCancel(sessionCtx)

	// exit := make(chan int)
	// t.exit = exit

	opts := docker.AttachToContainerOptions{
		Container:    t.containerID,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		Logs:         false,
		Success:      started,
		InputStream:  stdin,
		ErrorStream:  stdout,
		OutputStream: stderr,
		RawTerminal:  false,
	}

	go func() {
		defer cancel()
		err := t.client.AttachToContainer(opts)
		if err != nil {
			t.logger.Panicln(err)
		}
	}()

	// Wait for attach
	<-started
	go func() {
		defer cancel()
		status, err := t.client.WaitContainer(t.containerID)
		if err != nil {
			t.logger.Errorln("Error waiting", err)
		}
		t.logger.Debugln("Container finished with status code:", status, t.containerID)
	}()
	started <- struct{}{}
	return transportCtx, nil
}
