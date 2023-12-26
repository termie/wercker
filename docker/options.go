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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// DockerOptions for our docker client
type Options struct {
	Host                string
	TLSVerify           string
	CertPath            string
	DNS                 []string
	Local               bool
	CPUPeriod           int64
	CPUQuota            int64
	Memory              int64
	MemoryReservation   int64
	MemorySwap          int64
	KernelMemory        int64
	CleanupImage        bool
	NetworkName         string
	RddServiceURI       string
	RddProvisionTimeout time.Duration
}

func guessAndUpdateDockerOptions(ctx context.Context, opts *Options, e *util.Environment) {
	if opts.Host != "" {
		return
	}

	logger := util.RootLogger().WithField("Logger", "docker")
	// f := &util.Formatter{opts.GlobalOptions.ShowColors}
	f := &util.Formatter{ShowColors: false}

	// Check the unix socket, default on linux
	// This will fail instantly so don't bother with the goroutine

	unixSocket := "/var/run/docker.sock"
	logger.Println(f.Info("No Docker host specified, checking", unixSocket))

	if _, err := os.Stat(unixSocket); err == nil {
		unixSocket = fmt.Sprintf("unix://%s", unixSocket)
		client, err := NewOfficialDockerClient(&Options{
			Host: unixSocket,
		})
		if err == nil {
			_, err = client.ServerVersion(ctx)
			if err == nil {
				opts.Host = unixSocket
				return
			}
		}
	}

	// Check the boot2docker port with default cert paths and such
	b2dCertPath := filepath.Join(e.Get("HOME"), ".boot2docker/certs/boot2docker-vm")
	b2dHost := "tcp://192.168.59.103:2376"

	logger.Printf(f.Info("No Docker host specified, checking for boot2docker", b2dHost))
	client, err := NewDockerClient(&Options{
		Host:      b2dHost,
		CertPath:  b2dCertPath,
		TLSVerify: "1",
	})
	if err == nil {
		// This can take a long time if it isn't up, so toss it in a
		// goroutine so we can time it out
		result := make(chan bool)
		go func() {
			_, err = client.Version()
			if err == nil {
				result <- true
			} else {
				result <- false
			}
		}()
		select {
		case success := <-result:
			if success {
				opts.Host = b2dHost
				opts.CertPath = b2dCertPath
				opts.TLSVerify = "1"
				return
			}
		case <-time.After(1 * time.Second):
		}
	}

	// Pick a default localhost port and hope for the best :/
	opts.Host = "tcp://127.0.0.1:2375"
	logger.Println(f.Info("No Docker host found, falling back to default", opts.Host))
}

// NewDockerOptions constructor
func NewOptions(ctx context.Context, c util.Settings, e *util.Environment) (*Options, error) {
	dockerHost, _ := c.String("docker-host")
	dockerTLSVerify, _ := c.String("docker-tls-verify")
	dockerCertPath, _ := c.String("docker-cert-path")
	dockerDNS, _ := c.StringSlice("docker-dns")
	dockerLocal, _ := c.Bool("docker-local")
	dockerCPUPeriod, _ := c.Int("docker-cpu-period")
	dockerCPUQuota, _ := c.Int("docker-cpu-quota")
	dockerMemory, _ := c.Int("docker-memory")
	dockerMemoryReservation, _ := c.Int("docker-memory-reservation")
	dockerMemorySwap, _ := c.Int("docker-memory-swap")
	dockerKernelMemory, _ := c.Int("docker-kernel-memory")
	dockerCleanupImage, _ := c.Bool("docker-cleanup-image")
	dockerNetworkName, _ := c.String("docker-network")
	rddServiceURI, _ := c.String("rdd-service-uri")
	rddProvisionTimeout, _ := c.Duration("rdd-provision-timeout")

	speculativeOptions := &Options{
		Host:                dockerHost,
		TLSVerify:           dockerTLSVerify,
		CertPath:            dockerCertPath,
		DNS:                 dockerDNS,
		Local:               dockerLocal,
		CPUPeriod:           int64(dockerCPUPeriod),
		CPUQuota:            int64(dockerCPUQuota),
		Memory:              int64(dockerMemory) * 1024 * 1024,
		MemoryReservation:   int64(dockerMemoryReservation) * 1024 * 1024,
		MemorySwap:          int64(dockerMemorySwap) * 1024 * 1024,
		KernelMemory:        int64(dockerKernelMemory) * 1024 * 1024,
		CleanupImage:        dockerCleanupImage,
		NetworkName:         dockerNetworkName,
		RddServiceURI:       rddServiceURI,
		RddProvisionTimeout: rddProvisionTimeout,
	}

	// We're going to try out a few settings and set DockerHost if
	// one of them works, it they don't we'll get a nice error when
	// requireDockerEndpoint triggers later on
	guessAndUpdateDockerOptions(ctx, speculativeOptions, e)
	return speculativeOptions, nil
}
