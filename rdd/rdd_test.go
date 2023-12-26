package rdd

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	context "golang.org/x/net/context"

	"github.com/stretchr/testify/suite"
	rddpb "github.com/wercker/wercker/rddpb"
	"github.com/wercker/wercker/util"
	"google.golang.org/grpc"
)

type RDDSuite struct {
	*util.TestSuite
}

type rddProvStub struct {
	Request  *rddpb.RDDProvisionRequest
	Response *rddpb.RDDProvisionResponse
	Error    error
}

type rddStatusStub struct {
	Request  *rddpb.RDDStatusRequest
	Response *rddpb.RDDStatusResponse
	Error    error
}

type FakeRDDClient struct {
	rddProvStubs   []*rddProvStub
	rddStatusStubs []*rddStatusStub
}

func (c *FakeRDDClient) Provision(ctx context.Context, in *rddpb.RDDProvisionRequest, opts ...grpc.CallOption) (*rddpb.RDDProvisionResponse, error) {
	for _, val := range c.rddProvStubs {
		if reflect.DeepEqual(in, val.Request) {
			return val.Response, val.Error
		}
	}

	return nil, errors.New("stub not found")
}

func (c *FakeRDDClient) GetStatus(ctx context.Context, in *rddpb.RDDStatusRequest, opts ...grpc.CallOption) (*rddpb.RDDStatusResponse, error) {
	for _, val := range c.rddStatusStubs {
		if reflect.DeepEqual(in, val.Request) {
			return val.Response, val.Error
		}
	}

	return nil, errors.New("stub not found")
}

func (c *FakeRDDClient) Deprovision(ctx context.Context, in *rddpb.RDDDeprovisionRequest, opts ...grpc.CallOption) (*rddpb.RDDDeprovisionResponse, error) {
	return nil, nil
}

func (c *FakeRDDClient) Action(ctx context.Context, in *rddpb.ActionRequest, opts ...grpc.CallOption) (*rddpb.ActionResponse, error) {
	return nil, nil
}

func (c *FakeRDDClient) StubRddProvision(req *rddpb.RDDProvisionRequest, result *rddpb.RDDProvisionResponse, err error) *rddProvStub {
	stub := &rddProvStub{Request: req, Response: result, Error: err}
	c.rddProvStubs = append(c.rddProvStubs, stub)
	return stub
}

func (c *FakeRDDClient) StubRddStatus(req *rddpb.RDDStatusRequest, result *rddpb.RDDStatusResponse, err error) *rddStatusStub {
	stub := &rddStatusStub{Request: req, Response: result, Error: err}
	c.rddStatusStubs = append(c.rddStatusStubs, stub)
	return stub
}

var _ rddpb.RddClient = &FakeRDDClient{}
var defaultRDDServiceEndPoint = "localhost:4621"
var defaultRDDProvisionTimeOut = 300 * time.Second
var defaultContext = context.Background()
var defaultRDDClient = &FakeRDDClient{rddProvStubs: []*rddProvStub{}}
var rdd = &RDD{
	rddServiceEndpoint:  defaultRDDServiceEndPoint,
	rddProvisionTimeout: defaultRDDProvisionTimeOut,
	rddClient:           defaultRDDClient,
}

var runIDFailOnProvision = "failme"
var runIDInvalidProvisionResponse = "invalid"
var provisionIDInvalidProvisionResponse = ""
var runIDTimeOut = "timeout"
var provisionIDTimeOut = "timeout"
var runIDGetStatusError = "error"
var provisionIDGetStatusError = "error"
var runIDGetStatusInvalidRDDUrl = "invalidUrl"
var provisionIDGetStatusInvalidRDDUrl = "invalidUrl"

func TestRDDSuite(t *testing.T) {
	suiteTester := &RDDSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

//TestProvision_FailOnRDDProvision - Tests the scenario when Provision() API invoked from Get()
//returns an error
func (s *RDDSuite) TestProvision_FailOnRDDProvision() {
	rddProvFailRequest := &rddpb.RDDProvisionRequest{RunID: runIDFailOnProvision}
	errMsg := "some error"
	defaultRDDClient.StubRddProvision(rddProvFailRequest, nil, errors.New(errMsg))
	rdd.runID = runIDFailOnProvision
	rddURI, err := rdd.Provision(defaultContext)
	s.Empty(rddURI, "rddUri should be empty, got %s", rddURI)
	s.Equal(err.Error(), fmt.Sprintf(errorMsgFailOnProvision, defaultRDDServiceEndPoint, runIDFailOnProvision, errMsg))
}

//TestProvision_InvalidResponseOnRDDProvision - Tests the scenario when Provision() API invoked from Get()
//returns an RDDProvisionResponse with empty Id
func (s *RDDSuite) TestProvision_InvalidResponseOnRDDProvision() {
	rddProvRequest := &rddpb.RDDProvisionRequest{RunID: runIDInvalidProvisionResponse}
	rddProvInvalidResponse := &rddpb.RDDProvisionResponse{Id: provisionIDInvalidProvisionResponse}
	defaultRDDClient.StubRddProvision(rddProvRequest, rddProvInvalidResponse, nil)
	rdd.runID = runIDInvalidProvisionResponse
	rddURI, err := rdd.Provision(defaultContext)
	s.Empty(rddURI, "rddUri should be empty, got %s", rddURI)
	s.Equal(err.Error(), fmt.Sprintf(errorMsgInvalidProvisioningResponse, defaultRDDServiceEndPoint, runIDInvalidProvisionResponse))
}

//TestProvision_TimeoutOnRDDProvision - Tests the scenario when GetStatus() API invoked from Get()
//times out
func (s *RDDSuite) TestProvision_TimeoutOnRDDProvision() {
	rddProvRequest := &rddpb.RDDProvisionRequest{RunID: runIDTimeOut}
	rddProvResponse := &rddpb.RDDProvisionResponse{Id: provisionIDTimeOut}
	defaultRDDClient.StubRddProvision(rddProvRequest, rddProvResponse, nil)
	rddStatusRequest := &rddpb.RDDStatusRequest{Id: rddProvResponse.GetId()}
	rddStatusResponse := &rddpb.RDDStatusResponse{RunID: runIDTimeOut, State: rddpb.DaemonState_provisioning}
	defaultRDDClient.StubRddStatus(rddStatusRequest, rddStatusResponse, nil)
	rdd.runID = runIDTimeOut
	rdd.rddProvisionTimeout = 5 * time.Second
	rddURI, err := rdd.Provision(defaultContext)
	s.Empty(rddURI, "rddUri should be empty, got %s", rddURI)
	s.Equal(err.Error(), fmt.Sprintf(errorMsgTimeOut, defaultRDDServiceEndPoint, runIDTimeOut, rdd.rddProvisionTimeout.String()))
}

//TestProvision_ErrorOnGetStatus - Tests the scenario when GetStatus() API invoked from Get()
//returns a response with State DaemonState_error
func (s *RDDSuite) TestProvision_ErrorOnGetStatus() {
	rddProvRequest := &rddpb.RDDProvisionRequest{RunID: runIDGetStatusError}
	rddProvResponse := &rddpb.RDDProvisionResponse{Id: provisionIDGetStatusError}
	defaultRDDClient.StubRddProvision(rddProvRequest, rddProvResponse, nil)
	rddStatusRequest := &rddpb.RDDStatusRequest{Id: rddProvResponse.GetId()}
	rddStatusResponse := &rddpb.RDDStatusResponse{RunID: runIDGetStatusError, State: rddpb.DaemonState_error}
	defaultRDDClient.StubRddStatus(rddStatusRequest, rddStatusResponse, nil)
	rdd.runID = runIDGetStatusError
	rddURI, err := rdd.Provision(defaultContext)
	s.Empty(rddURI, "rddUri should be empty, got %s", rddURI)
	s.Equal(err.Error(), fmt.Sprintf(errorMsgGetStatusError, defaultRDDServiceEndPoint, runIDGetStatusError))
}

//TestProvision_InvalidRDDUrlOnGetStatus - Tests the scenario when GetStatus() API invoked from Get()
//returns a response with empty RDD URL
func (s *RDDSuite) TestProvision_InvalidRDDUrlOnGetStatus() {
	rddProvRequest := &rddpb.RDDProvisionRequest{RunID: runIDGetStatusInvalidRDDUrl}
	rddProvResponse := &rddpb.RDDProvisionResponse{Id: provisionIDGetStatusInvalidRDDUrl}
	defaultRDDClient.StubRddProvision(rddProvRequest, rddProvResponse, nil)
	rddStatusRequest := &rddpb.RDDStatusRequest{Id: rddProvResponse.GetId()}
	rddStatusResponse := &rddpb.RDDStatusResponse{RunID: runIDGetStatusInvalidRDDUrl, State: rddpb.DaemonState_provisioned}
	defaultRDDClient.StubRddStatus(rddStatusRequest, rddStatusResponse, nil)
	rdd.runID = runIDGetStatusInvalidRDDUrl
	rddURI, err := rdd.Provision(defaultContext)
	s.Empty(rddURI, "rddUri should be empty, got %s", rddURI)
	s.Equal(err.Error(), fmt.Sprintf(errorMsgInvalidRDDUrl, defaultRDDServiceEndPoint, runIDGetStatusInvalidRDDUrl))
}
