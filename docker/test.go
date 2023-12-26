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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/pkg/errors"

	"github.com/docker/docker/api/types"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// DockerOrSkip checks for a docker container and skips the test
// if one is not available
func DockerOrSkip(ctx context.Context, t *testing.T) *OfficialDockerClient {
	if os.Getenv("SKIP_DOCKER_TEST") == "true" {
		t.Skip("$SKIP_DOCKER_TEST=true, skipping test")
		return nil
	}

	client, err := NewOfficialDockerClient(MinimalDockerOptions())
	_, err = client.Ping(ctx)
	if err != nil {
		t.Skip("Docker not available, skipping test")
		return nil
	}
	return client
}

func MinimalDockerOptions() *Options {
	ctx := context.Background()
	opts := &Options{}
	guessAndUpdateDockerOptions(ctx, opts, util.NewEnvironment(os.Environ()...))
	return opts
}

type ContainerRemover struct {
	*container.ContainerCreateCreatedBody
	client *OfficialDockerClient
}

func TempBusybox(ctx context.Context, client *OfficialDockerClient) (*ContainerRemover, error) {
	_, _, err := client.ImageInspectWithRaw(ctx, "alpine:3.1")
	if err != nil {
		readCloser, err := client.ImagePull(ctx, "alpine:3.1", types.ImagePullOptions{})
		if err != nil {
			return nil, err
		}
		defer readCloser.Close()
	}

	// It seems it takes some time for the new image to become available for container creation,
	// so if the first attempt at starting the container fails with "No such image" we try again
	timeout := time.After(10 * time.Second)
	tick := time.Tick(500 * time.Millisecond)
	for {
		select {
		case <-timeout:
			return nil, errors.New("Timed out trying to start container")
		case <-tick:
			container, err := client.ContainerCreate(ctx,
				&container.Config{
					Image:           "alpine:3.1",
					Cmd:             []string{"/bin/sh"},
					Tty:             false,
					OpenStdin:       true,
					AttachStdin:     true,
					AttachStdout:    true,
					AttachStderr:    true,
					NetworkDisabled: true,
				}, &container.HostConfig{}, &network.NetworkingConfig{}, "temp-busybox")
			if err == nil {
				return &ContainerRemover{ContainerCreateCreatedBody: &container, client: client}, nil
			} else if strings.HasPrefix(err.Error(), "Error: No such image:") {
				// try again
			} else if strings.HasPrefix(err.Error(), "Error response from daemon: Conflict. The container name \"/temp-busybox\" is already in use by container") {
				// need to delete container left over from previous test run
				err := client.ContainerRemove(ctx, "temp-busybox", types.ContainerRemoveOptions{RemoveVolumes: true})
				if err != nil {
					return nil, err
				}
				// try again
			} else {
				return nil, err
			}
		}
	}
}

func (c *ContainerRemover) Remove(ctx context.Context) error {
	if c == nil {
		return nil
	}
	return c.client.ContainerRemove(ctx, c.ContainerCreateCreatedBody.ID, types.ContainerRemoveOptions{RemoveVolumes: true})
}
