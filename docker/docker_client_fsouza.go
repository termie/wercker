//   Copyright © 2016 Oracle and/or its affiliates.  All rights reserved.
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
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"

	dockersignal "github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/term"
	"github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/wercker/wercker/util"
)

var (
	// DefaultMaxRetriesCreateContainer - Default for maximum no. of CreateContainer retries
	DefaultMaxRetriesCreateContainer = 10
)

// DockerClient is our wrapper for docker.Client
type DockerClient struct {
	*docker.Client
	logger *util.LogEntry
}

// NewDockerClient based on options and env
func NewDockerClient(options *Options) (*DockerClient, error) {
	dockerHost := options.Host
	tlsVerify := options.TLSVerify

	logger := util.RootLogger().WithField("Logger", "Docker")

	var (
		client *docker.Client
		err    error
	)

	if tlsVerify == "1" {
		// We're using TLS, let's locate our certs and such
		// boot2docker puts its certs at...
		dockerCertPath := options.CertPath

		// TODO(termie): maybe fast-fail if these don't exist?
		cert := path.Join(dockerCertPath, fmt.Sprintf("cert.pem"))
		ca := path.Join(dockerCertPath, fmt.Sprintf("ca.pem"))
		key := path.Join(dockerCertPath, fmt.Sprintf("key.pem"))
		client, err = docker.NewVersionnedTLSClient(dockerHost, cert, key, ca, "")
		if err != nil {
			return nil, err
		}
	} else {
		client, err = docker.NewClient(dockerHost)
		if err != nil {
			return nil, err
		}
	}
	return &DockerClient{Client: client, logger: logger}, nil
}

// RunAndAttach gives us a raw connection to a newly run container
func (c *DockerClient) RunAndAttach(name string) error {
	hostConfig := &docker.HostConfig{}
	cmd, _ := shlex.Split(DefaultDockerCommand)
	container, err := c.CreateContainerWithRetries(
		docker.CreateContainerOptions{
			Name: uuid.NewRandom().String(),
			Config: &docker.Config{
				Image:        name,
				Tty:          true,
				OpenStdin:    true,
				Cmd:          cmd,
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
				// NetworkDisabled: b.networkDisabled,
				// Volumes: volumes,
			},
			HostConfig: hostConfig,
		})
	if err != nil {
		return err
	}
	c.StartContainer(container.ID, hostConfig)

	return c.AttachTerminal(container.ID)
}

// AttachInteractive starts an interactive session and runs cmd
func (c *DockerClient) AttachInteractive(containerID string, cmd []string, initialStdin []string) error {

	exec, err := c.CreateExec(docker.CreateExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          cmd,
		Container:    containerID,
	})

	if err != nil {
		return err
	}

	// Dump any initial stdin then go into os.Stdin
	readers := []io.Reader{}
	for _, s := range initialStdin {
		if s != "" {
			readers = append(readers, strings.NewReader(s+"\n"))
		}
	}
	readers = append(readers, os.Stdin)
	stdin := io.MultiReader(readers...)

	// This causes our ctrl-c's to be passed to the stuff in the terminal
	var oldState *term.State
	oldState, err = term.SetRawTerminal(os.Stdin.Fd())
	if err != nil {
		return err
	}
	defer term.RestoreTerminal(os.Stdin.Fd(), oldState)

	// Handle resizes
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, dockersignal.SIGWINCH)
	go func() {
		for range sigchan {
			c.ResizeTTY(exec.ID)
		}
	}()

	err = c.StartExec(exec.ID, docker.StartExecOptions{
		InputStream:  stdin,
		OutputStream: os.Stdout,
		ErrorStream:  os.Stderr,
		Tty:          true,
		RawTerminal:  true,
	})

	return err
}

// ResizeTTY resizes the tty size of docker connection so output looks normal
func (c *DockerClient) ResizeTTY(execID string) error {
	ws, err := term.GetWinsize(os.Stdout.Fd())
	if err != nil {
		c.logger.Debugln("Error getting term size: %s", err)
		return err
	}
	err = c.ResizeExecTTY(execID, int(ws.Height), int(ws.Width))
	if err != nil {
		c.logger.Debugln("Error resizing term: %s", err)
		return err
	}
	return nil
}

// AttachTerminal connects us to container and gives us a terminal
func (c *DockerClient) AttachTerminal(containerID string) error {
	c.logger.Println("Attaching to ", containerID)
	opts := docker.AttachToContainerOptions{
		Container:    containerID,
		Logs:         true,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		InputStream:  os.Stdin,
		ErrorStream:  os.Stderr,
		OutputStream: os.Stdout,
		RawTerminal:  true,
	}

	var oldState *term.State

	oldState, err := term.SetRawTerminal(os.Stdin.Fd())
	if err != nil {
		return err
	}
	defer term.RestoreTerminal(os.Stdin.Fd(), oldState)

	go func() {
		err := c.AttachToContainer(opts)
		if err != nil {
			c.logger.Panicln("attach panic", err)
		}
	}()

	_, err = c.WaitContainer(containerID)
	return err
}

// ExecOne uses docker exec to run a command in the container
func (c *DockerClient) ExecOne(containerID string, cmd []string, output io.Writer) error {
	exec, err := c.CreateExec(docker.CreateExecOptions{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          cmd,
		Container:    containerID,
	})
	if err != nil {
		return err
	}

	err = c.StartExec(exec.ID, docker.StartExecOptions{
		OutputStream: output,
	})
	if err != nil {
		return err
	}

	return nil
}

// CreateContainerWithRetries create a container - retry on "no such image" error
func (c *DockerClient) CreateContainerWithRetries(opts docker.CreateContainerOptions) (*docker.Container, error) {
	var numRetry int

	for numRetry < DefaultMaxRetriesCreateContainer {
		container, err := c.CreateContainer(opts)

		if err == nil {
			return container, nil
		}

		if err != docker.ErrNoSuchImage {
			return nil, err
		}

		numRetry++

		delayTime := 500 * numRetry
		time.Sleep((time.Duration)(delayTime) * time.Millisecond)

	}

	return nil, errors.New("Failed trying to create container")
}
