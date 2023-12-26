//   Copyright © 2018, Oracle and/or its affiliates.  All rights reserved.
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
	"path"
	"reflect"

	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

const (
	// DefaultDockerRegistryUsername is an arbitrary value. It is unused by callees,
	// so the value can be anything so long as it's not empty.
	DefaultDockerRegistryUsername = "token"
	DefaultDockerCommand          = `/bin/sh -c "if [ -e /bin/bash ]; then /bin/bash; else /bin/sh; fi"`
)

// OfficialDockerClient is a wrapper for client.Client (which makes it easier to substitute a mock for testing)
type OfficialDockerClient struct {
	*client.Client
}

// NewOfficialDockerClient uses the official docker client to create a Client struct
// which can be used to perform operations against a docker server
func NewOfficialDockerClient(options *Options) (*OfficialDockerClient, error) {
	var dockerClient *client.Client
	var err error
	if options.TLSVerify == "1" {
		// We're using TLS, let's locate our certs and such
		// boot2docker puts its certs at...
		dockerCertPath := options.CertPath
		// TODO: maybe fast-fail if these don't exist?
		cert := path.Join(dockerCertPath, fmt.Sprintf("cert.pem"))
		ca := path.Join(dockerCertPath, fmt.Sprintf("ca.pem"))
		key := path.Join(dockerCertPath, fmt.Sprintf("key.pem"))
		dockerClient, err = client.NewClientWithOpts(client.WithHost(options.Host), client.WithTLSClientConfig(ca, cert, key), client.WithVersion("1.24"))
	} else {
		dockerClient, err = client.NewClientWithOpts(client.WithHost(options.Host), client.WithVersion("1.24"))
	}
	if err != nil {
		return nil, err
	}
	return &OfficialDockerClient{Client: dockerClient}, nil
}

// RequireDockerEndpoint attempts to connect to the specified docker daemon and returns an error if unsuccessful
func RequireDockerEndpoint(ctx context.Context, options *Options) error {
	client, err := NewOfficialDockerClient(options)
	if err != nil {
		return fmt.Errorf(`Invalid Docker endpoint: %s
			To specify a different endpoint use the DOCKER_HOST environment variable,
			or the --docker-host command-line flag.
		`, err.Error())
	}
	_, err = client.ServerVersion(ctx)
	if err != nil {
		if reflect.TypeOf(err).String() == "client.errConnectionFailed" {
			return fmt.Errorf(`You don't seem to have a working Docker environment or wercker can't connect to the Docker endpoint:
			%s
		To specify a different endpoint use the DOCKER_HOST environment variable,
		or the --docker-host command-line flag.`, options.Host)
		}
		return err
	}
	return nil
}
