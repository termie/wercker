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

	shortid "github.com/SKAhack/go-shortid"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/wercker/wercker/util"
)

// GetDockerNetworkName returns docker network name of docker network.
// If docker network does not exist it creates one and return its name.
func (b *DockerBox) GetDockerNetworkName() (string, error) {
	dockerNetworkName := b.dockerOptions.NetworkName
	if dockerNetworkName == "" {
		if b.options.DockerNetworkName == "" {
			preparedDockerNetworkName, err := b.prepareDockerNetworkName()
			if err != nil {
				return "", err
			}

			b.options.DockerNetworkName = preparedDockerNetworkName
			_, err = b.createDockerNetwork(b.options.DockerNetworkName)
			if err != nil {
				b.logger.Errorln("Error while creating network", err)
				return "", err
			}
		}
		return b.options.DockerNetworkName, nil
	}
	client := b.client
	_, err := client.NetworkInfo(dockerNetworkName)
	if err != nil {
		b.logger.Errorln("Network does not exist", err)
		return "", err
	}
	return dockerNetworkName, nil
}

// CleanDockerNetwork remove docker network if created for this pipeline.
func (b *DockerBox) CleanDockerNetwork() error {
	dockerNetworkName := b.dockerOptions.NetworkName
	client := b.client
	if dockerNetworkName == "" {
		dockerNetworkName = b.options.DockerNetworkName
		if dockerNetworkName != "" {
			dockerNetwork, err := client.NetworkInfo(dockerNetworkName)
			if err != nil {
				b.logger.Errorln("Unable to get network Info", err)
				return err
			}
			for k := range dockerNetwork.Containers {
				err = client.DisconnectNetwork(dockerNetwork.ID, docker.NetworkConnectionOptions{
					Container: k,
					Force:     true,
				})
				if err != nil {
					b.logger.Errorln("Error while disconnecting container from network", err)
					return err
				}
			}
			b.logger.WithFields(util.LogFields{
				"Name": dockerNetworkName,
			}).Debugln("Removing docker network ", dockerNetworkName)
			err = client.RemoveNetwork(dockerNetworkName)
			if err != nil {
				b.logger.Errorln("Error while removing docker network", err)
				return err
			}
			b.options.DockerNetworkName = ""
		} else {
			b.logger.Debugln("Network does not exist")
		}
	} else {
		b.logger.Debugln("Custom netork")
	}
	return nil
}

// Create docker network
func (b *DockerBox) createDockerNetwork(dockerNetworkName string) (*docker.Network, error) {
	b.logger.Debugln("Creating docker network")
	client := b.client
	networkOptions := map[string]interface{}{
		"com.docker.network.bridge.enable_icc":           "true",
		"com.docker.network.bridge.enable_ip_masquerade": "true",
		"com.docker.network.driver.mtu":                  "1500",
	}
	b.logger.WithFields(util.LogFields{
		"Name": dockerNetworkName,
	}).Debugln("Creating docker network :", dockerNetworkName)
	return client.CreateNetwork(docker.CreateNetworkOptions{
		Name:           dockerNetworkName,
		CheckDuplicate: true,
		Options:        networkOptions,
	})
}

// Generate docker network name and check if same is already in use. In case name is already in use then it regenerate it upto 3 times before throwing error.
func (b *DockerBox) prepareDockerNetworkName() (string, error) {
	generator := shortid.Generator()
	client := b.client

	for i := 0; i < 3; i++ {
		dockerNetworkName := generator.Generate()
		dockerNetworkName = "w-" + dockerNetworkName
		network, _ := client.NetworkInfo(dockerNetworkName)
		if network != nil {
			b.logger.Debugln("Network name exist, retrying...")
		} else {
			return dockerNetworkName, nil
		}
	}
	err := fmt.Errorf("Unable to prepare unique network name")
	b.logger.Errorln(err)
	return "", err
}
