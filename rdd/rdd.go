//   Copyright (c) 2018, Oracle and/or its affiliates.  All rights reserved.
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

package rdd

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/docker/docker/client"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	grpcmw "github.com/mwitkow/go-grpc-middleware"
	"github.com/wercker/pkg/log"
	rddpb "github.com/wercker/wercker/rddpb"
	"google.golang.org/grpc"
	cli "gopkg.in/urfave/cli.v1"
)

const (
	errorMsgFailOnProvision             = "Error invoking Provision() from Remote Docker Daemon API service at %s for runID %s, Error: %s"
	errorMsgInvalidProvisioningResponse = "Invalid response by Provision() from Remote Docker Daemon API service at %s for runID %s, ResponseID is empty."
	errorMsgTimeOut                     = "Remote Docker Daemon provisioning timed out from Remote Docker Daemon API  service at %s for runID %s after %s."
	errorMsgGetStatusError              = "Error provisioning Remote Docker Daemon from Remote Docker Daemon API service at %s for runID %s."
	errorMsgInvalidRDDUrl               = "Invalid Remote Docker Daemon uri returned from Remote Docker Daemon API service service at %s for runID %s."
)

//RDD - struct containing all parameters and client for RDD access
type RDD struct {
	rddServiceEndpoint  string
	rddProvisionTimeout time.Duration
	runID               string
	rddClient           rddpb.RddClient
	rddDetails          *rddDetails
	rddDeprovRequested  bool
}

type rddDetails struct {
	rddURI                string
	rddProvisionRequestID string
}

//New - initialize a RDD construct, check connection with RDD API service and create a client
func New(rddServiceEndpoint string, rddProvisionTimeout time.Duration, runID string) (*RDD, error) {
	log.Debug("Connecting to rdd service")

	rddInterceptors := []grpc.UnaryClientInterceptor{
		grpc_prometheus.UnaryClientInterceptor,
	}

	rddConn, err := grpc.Dial(rddServiceEndpoint, grpc.WithInsecure(), grpc.WithUnaryInterceptor(grpcmw.ChainUnaryClient(rddInterceptors...)))
	if err != nil {
		errMsg := fmt.Sprintf("Failed to dial rdd service at %s for runID %s, Error: %s", rddServiceEndpoint, runID, err.Error())
		log.WithField("rddServiceEndpoint", rddServiceEndpoint).
			WithError(err).
			Error(errMsg)
		return nil, cli.NewExitError(errMsg, 1)
	}

	rddClient := rddpb.NewRddClient(rddConn)

	rdd := &RDD{rddServiceEndpoint: rddServiceEndpoint,
		rddProvisionTimeout: rddProvisionTimeout,
		runID:               runID,
		rddClient:           rddClient}

	return rdd, nil
}

//Provision - Invokes RDD Service to Provision remote docker daemon URL by first executing a Provision()
//request followed by polling GetStatus()
func (rdd *RDD) Provision(ctx context.Context) (string, error) {
	rddProvRequest := &rddpb.RDDProvisionRequest{RunID: rdd.runID}
	rddProvResponse, err := rdd.rddClient.Provision(ctx, rddProvRequest)
	if err != nil {
		errMsg := fmt.Sprintf(errorMsgFailOnProvision, rdd.rddServiceEndpoint, rdd.runID, err.Error())
		log.Error(errMsg)
		return "", cli.NewExitError(errMsg, 1)
	}

	rddResponseID := rddProvResponse.GetId()
	if rddResponseID == "" {
		errMsg := fmt.Sprintf(errorMsgInvalidProvisioningResponse, rdd.rddServiceEndpoint, rdd.runID)
		log.Error(errMsg)
		return "", cli.NewExitError(errMsg, 1)
	}

	rdd.rddDetails = &rddDetails{rddProvisionRequestID: rddResponseID}

	if rdd.rddProvisionTimeout <= 0*time.Second {
		log.Warningf("Invalid timeout value from input rdd-provision-timeout of %s, Default value of 300s will be used.", rdd.rddProvisionTimeout.String())
		rdd.rddProvisionTimeout = 300 * time.Second
	}
	timeout := time.After(rdd.rddProvisionTimeout)
	tick := time.Tick(5 * time.Second)

	for {
		select {

		case <-timeout:
			errMsg := fmt.Sprintf(errorMsgTimeOut, rdd.rddServiceEndpoint, rdd.runID, rdd.rddProvisionTimeout.String())
			log.Error(errMsg)
			return "", cli.NewExitError(errMsg, 1)

		case <-tick:
			if rdd.rddDeprovRequested {
				return "", fmt.Errorf("Remote Docker Daemon deprovisioning requested before provision was complete for runID: %s", rdd.runID)
			}
			rddStatusRequest := &rddpb.RDDStatusRequest{Id: rddResponseID}
			rddStatusResponse, err := rdd.rddClient.GetStatus(ctx, rddStatusRequest)
			if err != nil {
				errMsg := fmt.Sprintf("Error invoking GetStatus() from rdd service at %s for runID %s, Error: %s. Retrying...", rdd.rddServiceEndpoint, rdd.runID, err.Error())
				log.Error(errMsg)
				continue
			}
			currentRDDState := rddStatusResponse.GetState()
			if currentRDDState == rddpb.DaemonState_error {
				errMsg := fmt.Sprintf(errorMsgGetStatusError, rdd.rddServiceEndpoint, rdd.runID)
				log.Error(errMsg)
				return "", cli.NewExitError(errMsg, 1)
			}
			if currentRDDState == rddpb.DaemonState_provisioned {
				rddURI := rddStatusResponse.URL
				if rddURI == "" {
					errMsg := fmt.Sprintf(errorMsgInvalidRDDUrl, rdd.rddServiceEndpoint, rdd.runID)
					log.Error(errMsg)
					return "", cli.NewExitError(errMsg, 1)
				}
				err := rdd.verify(ctx, rddURI)
				if err != nil {
					rdd.Deprovision()
					return "", err
				}
				rdd.rddDetails.rddURI = rddURI
				return rddURI, nil

			}
			log.Info(fmt.Sprintf("runID: %s, RDD Service URI: %s, RDD Provisioning status: %s", rdd.runID, rdd.rddServiceEndpoint, currentRDDState.String()))

		}
	}
}

//Deprovision - Deprovisions a RDD previously provisioned for a build
func (rdd *RDD) Deprovision() {
	if rdd.rddDeprovRequested {
		log.Warnf("RDD Deprovisioning already requested for runID: %s", rdd.runID)
		return
	}
	rdd.rddDeprovRequested = true
	if rdd.rddDetails == nil || rdd.rddDetails.rddProvisionRequestID == "" {
		log.Debug("No RDD to deprovision")
		return
	}
	log.Infof("Deprovisioning RDD for runID: %s", rdd.runID)
	rddDeProvRequest := &rddpb.RDDDeprovisionRequest{Id: rdd.rddDetails.rddProvisionRequestID}
	_, err := rdd.rddClient.Deprovision(context.TODO(), rddDeProvRequest)
	if err != nil {
		errMsg := fmt.Sprintf("Error invoking Deprovision() from rdd service at %s for runID %s, Error: %s. Ignoring", rdd.rddServiceEndpoint, rdd.runID, err.Error())
		log.Warning(errMsg)
		return
	}
	log.Infof("Finished deprovisioning RDD for runID: %s", rdd.runID)
}

// verify the RDD url by connecting to it and retrieving its version
func (rdd *RDD) verify(ctx context.Context, rddURI string) error {
	log.Debugf("Verifying RDD at %s", rddURI)
	maxRetries := 5
	try := 0

	for try < maxRetries {
		if rdd.rddDeprovRequested {
			return fmt.Errorf("Remote Docker Daemon deprovisioning requested before verification was complete for runID: %s", rdd.runID)
		}
		try++

		dockerClient, err := client.NewClientWithOpts(client.WithHost(rddURI))
		if err != nil {
			return fmt.Errorf(`Unable to create a docker client with RDD URI: %s, Error: %s`, rddURI, err.Error())
		}

		version, err := dockerClient.ServerVersion(ctx)
		if err != nil {
			if reflect.TypeOf(err).String() == "client.errConnectionFailed" {
				log.Debug("Unable to connect to Docker daemon, retying")
				time.Sleep(time.Second * 5)
				continue
			}

			return err
		}

		if version.Version == "" {
			return fmt.Errorf(`Unidentifiable docker version at RDD URI: %s
			`, rddURI)
		}

		log.Info(fmt.Sprintf("Successfully connected to RDD at %s, Docker version: %s, Docker API version: %s", rddURI, version.Version, version.APIVersion))

		return nil
	}

	log.Warn("Tries exceeds max tries, aborting")

	return errors.New("unable to connect to remote Docker daemon, tries exceeded max tries")
}
