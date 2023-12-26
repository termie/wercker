package dockerlocal

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/suite"
	"github.com/wercker/docker-check-access"
	"github.com/wercker/wercker/auth"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

const (
	repoErrorInPush        = "fail_me/error"
	repoSuccessful         = "pass_me/successful"
	repoSuccessfulImageTag = "sometag"
)

type PushSuite struct {
	*util.TestSuite
}

func TestPushSuite(t *testing.T) {
	suiteTester := &PushSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

//TestEmptyPush tests if you juse did something like this
// - internal/docker-push
// it should fill in a tag of the git branch and commit
// check to see if its pushing up to the right registry or not
func (s *PushSuite) TestEmptyPush() {
	config := &core.StepConfig{
		ID:   "internal/docker-push",
		Data: map[string]string{},
	}
	options := &core.PipelineOptions{
		GitOptions: &core.GitOptions{
			GitBranch: "master",
			GitCommit: "s4k2r0d6a9b",
		},
		ApplicationID:            "1000001",
		ApplicationName:          "myproject",
		ApplicationOwnerName:     "wercker",
		WerckerContainerRegistry: &url.URL{Scheme: "https", Host: "wcr.io", Path: "/v2/"},
		GlobalOptions: &core.GlobalOptions{
			AuthToken: "su69persec420uret0k3n",
		},
	}
	step, err := NewDockerPushStep(config, options, nil)
	s.NoError(err)
	ctx := core.NewEmitterContext(context.TODO())
	err = step.InitEnv(ctx, nil)
	s.Equal("Repository not specified", err.Error())
	s.Empty(step.repository)
}

// TestLabelsInvalid tests that if the labels property contains a label specification
// which is not of the form label=value then the correct error is returned.
func (s *PushSuite) TestLabelSpecificationInvalid() {

	stepData := make(map[string]string)
	stepData["username"] = "user"
	stepData["password"] = "pass"
	stepData["repository"] = repoSuccessful
	stepData["registry"] = "https://quay.io"
	stepData["tag"] = repoSuccessfulImageTag
	stepData["labels"] = "label1=value1 label2value2 label3=value3"

	config := &core.StepConfig{
		ID:   "internal/docker-push",
		Data: stepData,
	}

	options := &core.PipelineOptions{
		WerckerContainerRegistry: &url.URL{Scheme: "https", Host: "myregistry.io", Path: "/v2/"},
	}

	step, err := NewDockerPushStep(config, options, nil)
	s.NoError(err)
	ctx := core.NewEmitterContext(context.TODO())
	err = step.InitEnv(ctx, nil)
	s.Equal("label specification label2value2 is not of the form label=value", err.Error())
}

func (s *PushSuite) TestInferRegistryAndRepository() {
	repoTests := []struct {
		registry           string
		repository         string
		expectedRegistry   string
		expectedRepository string
		expectedError      error
	}{
		{"", "appowner/appname", "", "appowner/appname", nil},
		{"", "", "", "", fmt.Errorf("Repository not specified")},
		{"", "someregistry.com/appowner/appname", "https://someregistry.com/v2/", "someregistry.com/appowner/appname", nil},
		{"", "appOWNER/appname", "", "appowner/appname", nil},
		{"https://someregistry.com", "appowner/appname", "https://someregistry.com", "someregistry.com/appowner/appname", nil},
		{"https://someregistry.com/v1", "appowner/appname", "https://someregistry.com/v1", "someregistry.com/appowner/appname", nil},
		{"https://someregistry.com/v2", "appowner/appname", "https://someregistry.com/v2", "someregistry.com/appowner/appname", nil},
		{"https://someregistry.com", "someotherregistry.com/appowner/appname", "https://someotherregistry.com/v2/", "someotherregistry.com/appowner/appname", nil},
		{"https://someregistry.com", "appowner/appname", "https://someregistry.com", "someregistry.com/appowner/appname", nil},
	}

	ctx := core.NewEmitterContext(context.TODO())
	for _, tt := range repoTests {
		options := &core.PipelineOptions{}
		opts := dockerauth.CheckAccessOptions{
			Registry: tt.registry,
		}
		repo, registry, err := InferRegistryAndRepository(ctx, tt.repository, opts.Registry, options)
		s.Equal(tt.expectedError, err)
		opts.Registry = registry
		s.Equal(tt.expectedRegistry, opts.Registry, "%q, wants %q", opts.Registry, tt.expectedRegistry)
		s.Equal(tt.expectedRepository, repo, "%q, wants %q", repo, tt.expectedRepository)
	}

}

// TestTagAndPushStatusReportingForErrorInPush - Tests a scenario when
// push fails and the returned JSON contains an error message
func (s *PushSuite) TestTagAndPushStatusReportingForErrorInPush() {
	stepData := make(map[string]string)
	stepData["username"] = "user"
	stepData["password"] = "pass"
	stepData["repository"] = repoErrorInPush
	stepData["registry"] = "https://quay.io"
	stepData["tag"] = "test"

	exitCode, error := executeTagAndPush(stepData)
	s.NotEqual(exitCode, 0)
	s.NotNil(error)
	if error != nil {
		s.Contains(error.Error(), errorMessage)
	}
}

// TestTagAndPushStatusReportingForSuccessfulPush - Tests the scenario when a push is
// successful and tagAndPush will only return success if the status message from docker will
// contain digest and tag of pushed container
func (s *PushSuite) TestTagAndPushStatusReportingForSuccessfulPush() {
	stepData := make(map[string]string)
	stepData["username"] = "user"
	stepData["password"] = "pass"
	stepData["repository"] = repoSuccessful
	stepData["registry"] = "https://quay.io"
	stepData["tag"] = repoSuccessfulImageTag

	exitCode, error := executeTagAndPush(stepData)
	s.Equal(exitCode, 0)
	s.Nil(error)
}

// executeTagAndPush - Invokes tagAndPush
func executeTagAndPush(stepData map[string]string) (int, error) {
	step, ctx, mockEmittor, mockDockerClient := prepareDockerPush(stepData)
	return step.tagAndPush(ctx, "test", mockEmittor, mockDockerClient)
}

// ImageTag - Mocks Docker client function ImageTag
func (cli *OfficialDockerClient) ImageTag(ctx context.Context, source, target string) error {
	return nil
}

// ImageRemove - Mocks Docker client function ImageRemove
func (client *OfficialDockerClient) ImageRemove(ctx context.Context, imageID string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
	return nil, nil
}

type mockReadCloser struct {
	io.Reader
}

func (mockReadCloser) Close() error {
	return nil
}

// ImagePush - Mocks Docker client function ImagePush
func (cli *OfficialDockerClient) ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error) {
	// The returned JSON depends on which pretend repo has been specified
	var reader io.Reader
	if strings.HasPrefix(image, repoErrorInPush) {
		reader = bytes.NewReader(getJSONOutputForMockErrorInPush())
	} else {
		// RepoSuccessful
		reader = bytes.NewReader(getJSONOutputForMockSuccessfulPush())
	}
	return &mockReadCloser{Reader: reader}, nil
}

func getJSONOutputForMockSuccessfulPush() []byte {
	result := "{\"status\":\"The push refers to repository [docker.io/foo/bar]\"}"
	return []byte(result)
}

var errorMessage = "error parsing HTTP 404 response body: invalid character"

func getJSONOutputForMockErrorInPush() []byte {
	result := "{\"errorDetail\":{\"message\":\"" + errorMessage + "\",\"error\":\"" + errorMessage + "\"}}"
	return []byte(result)
}

//TestInferRegistryAndRepositoryInvalidInputs validates that poper errors
// are being returned by InferRegistryAndRepository menthod when invalid
// inputs are provided for repository and registry
func (s *PushSuite) TestInferRegistryAndRepositoryInvalidInputs() {
	testWerckerRegistry, _ := url.Parse("https://test.wcr.io/v2")
	repoTests := []struct {
		registry           string
		repository         string
		expectedRegistry   string
		expectedRepository string
		errorMessage       string
	}{
		{"invalidregistry", "appowner/appname", "", "", "not a valid registry URL"},
		{"https://someregistry.com", "appowner//appname", "", "", "not a valid repository"},
		{"https://someregistry.com", "https://someregistry.com/appowner/appname", "", "", "not a valid repository"},
	}

	ctx := core.NewEmitterContext(context.TODO())
	for _, tt := range repoTests {
		options := &core.PipelineOptions{
			ApplicationOwnerName:     "appowner",
			ApplicationName:          "appname",
			WerckerContainerRegistry: testWerckerRegistry,
		}
		opts := dockerauth.CheckAccessOptions{
			Registry: tt.registry,
		}
		repo, registry, err := InferRegistryAndRepository(ctx, tt.repository, opts.Registry, options)
		opts.Registry = registry
		s.Error(err)
		s.Contains(err.Error(), tt.errorMessage)
		s.Equal(tt.expectedRegistry, opts.Registry, "%q, wants %q", opts.Registry, tt.expectedRegistry)
		s.Equal(tt.expectedRepository, repo, "%q, wants %q", repo, tt.expectedRepository)
	}

}

// TestDockerPushExecute_AllInvalidTags - Tests that error is
// returned by step.Execute  on internal/docker-push when all input tags are invalid
func (s *PushSuite) TestDockerPushExecute_AllInvalidTags() {
	stepData := make(map[string]string)
	stepData["username"] = "user"
	stepData["password"] = "pass"
	stepData["repository"] = "docker.io/valid"
	stepData["registry"] = "https://quay.io"
	stepData["tag"] = "invalid/invalid,not-valid:not-valid"
	exitCode, err := executeDockerPushStep(stepData)
	s.NotEqual(0, exitCode)
	s.NotNil(err)
	s.Contains(err.Error(), "Invalid tag")
}

// TestDockerPushExecute_OneInvalidTag - Tests that error is
// returned by step.Execute  on internal/docker-push when even one input tag is invalid
func (s *PushSuite) TestDockerPushExecute_OneInvalidTag() {
	stepData := make(map[string]string)
	stepData["username"] = "user"
	stepData["password"] = "pass"
	stepData["repository"] = "docker.io/valid"
	stepData["registry"] = "https://quay.io"
	stepData["tag"] = "valid,not-valid:not-valid"
	exitCode, err := executeDockerPushStep(stepData)
	s.NotEqual(0, exitCode)
	s.NotNil(err)
	s.Contains(err.Error(), "Invalid tag")
}

// TestDockerPushExecute_InvalidRepo - Tests that error is
// returned by step.Execute  on internal/docker-push when input repository is invalid
func (s *PushSuite) TestDockerPushExecute_InvalidRepo() {
	stepData := make(map[string]string)
	stepData["username"] = "user"
	stepData["password"] = "pass"
	stepData["repository"] = "INVALID:INVALID"
	stepData["registry"] = "https://quay.io"
	stepData["tag"] = "valid"
	exitCode, err := executeDockerPushStep(stepData)
	s.NotEqual(0, exitCode)
	s.NotNil(err)
	s.Contains(err.Error(), "Invalid repository")
}

// TestDockerPushExecute_InvalidTagDockerBuild - Tests that error is
// returned by step.Execute  on internal/docker-push when even the input tag is invalid
// and image to be pushed was built using internal/docker-build
func (s *PushSuite) TestDockerPushExecute_InvalidTagDockerBuild() {
	stepData := make(map[string]string)
	stepData["username"] = "user"
	stepData["password"] = "pass"
	stepData["repository"] = "docker.io/valid"
	stepData["registry"] = "https://quay.io"
	stepData["tag"] = "not-valid:not-valid"
	stepData["image-name"] = "imageName"
	exitCode, err := executeDockerPushStep(stepData)
	s.NotEqual(0, exitCode)
	s.NotNil(err)
	s.Contains(err.Error(), "Invalid tag")
}

// TestDockerScratchPushExecute_AllInvalidTags - Tests that error is
// returned by step.Execute on internal/docker-scratch-push when all input tags are invalid
func (s *PushSuite) TestDockerScratchPushExecute_AllInvalidTags() {
	stepData := make(map[string]string)
	stepData["username"] = "user"
	stepData["password"] = "pass"
	stepData["repository"] = "docker.io/valid"
	stepData["registry"] = "https://quay.io"
	stepData["tag"] = "invalid/invalid,not-valid:not-valid"
	exitCode, err := executeDockerScratchPushStep(stepData)
	s.NotEqual(0, exitCode)
	s.NotNil(err)
	s.Contains(err.Error(), "Invalid tag")
}

// TestDockerScratchPushExecute_OneInvalidTag - Tests that error is
// returned by step.Execute  on internal/docker-scratch-push  when even one input tag is invalid
func (s *PushSuite) TestDockerScratchPushExecute_OneInvalidTag() {
	stepData := make(map[string]string)
	stepData["username"] = "user"
	stepData["password"] = "pass"
	stepData["repository"] = "docker.io/valid"
	stepData["registry"] = "https://quay.io"
	stepData["tag"] = "valid,not-valid:not-valid"
	exitCode, err := executeDockerScratchPushStep(stepData)
	s.NotEqual(0, exitCode)
	s.NotNil(err)
	s.Contains(err.Error(), "Invalid tag")
}

// TestDockerScratchPushExecute_InvalidRepo - Tests that error is
// returned by step.Execute  on internal/docker-scratch-push when input repository is invalid
func (s *PushSuite) TestDockerScratchPushExecute_InvalidRepo() {
	stepData := make(map[string]string)
	stepData["username"] = "user"
	stepData["password"] = "pass"
	stepData["repository"] = "INVALID:INVALID"
	stepData["registry"] = "https://quay.io"
	stepData["tag"] = "valid"
	exitCode, err := executeDockerScratchPushStep(stepData)
	s.NotEqual(0, exitCode)
	s.NotNil(err)
	s.Contains(err.Error(), "Invalid repository")
}

// executeDockerPushStep - Invokes Execute on DockerPushStep
func executeDockerPushStep(stepData map[string]string) (int, error) {
	step, ctx, _, _ := prepareDockerPush(stepData)
	step.authenticator = &MockAuth{}
	exitCode, err := step.Execute(ctx, core.NewSession(nil, &DockerTransport{}))
	return exitCode, err
}

// executeDockerScratchPushStep - Invokes Execute on DockerScratchPushStep
func executeDockerScratchPushStep(stepData map[string]string) (int, error) {
	step, ctx, _, _ := prepareDockerScratchPush(stepData)
	step.authenticator = &MockAuth{}
	exitCode, err := step.Execute(ctx, core.NewSession(nil, &DockerTransport{}))
	return exitCode, err
}

// MockAuth - Mock implementation of auth.Authenticator
type MockAuth struct {
}

func (m *MockAuth) CheckAccess(repository string, scope auth.Scope) (bool, error) {
	return true, nil
}

func (m *MockAuth) Username() string {
	return ""
}

func (m *MockAuth) Password() string {
	return ""
}

func (m *MockAuth) Repository(repo string) string {
	return repo
}

// prepareDockerPush - Prepares stepConfig for docker-push step from input stepData
func prepareDockerPush(stepData map[string]string) (*DockerPushStep, context.Context, *core.NormalizedEmitter, *OfficialDockerClient) {
	config := &core.StepConfig{
		ID:   "internal/docker-push",
		Data: stepData,
	}
	options := &core.PipelineOptions{}
	step, _ := NewDockerPushStep(config, options, nil)
	step.configure(&util.Environment{})
	step.dockerOptions = &Options{}
	step.authenticator = &auth.DockerAuth{}
	step.logger = util.NewLogger().WithFields(util.LogFields{
		"Logger": "Test",
	})
	mockEmittor := core.NewNormalizedEmitter()
	mockDockerClient := &OfficialDockerClient{}
	ctx := context.WithValue(context.Background(), "Emitter", mockEmittor)
	return step, ctx, mockEmittor, mockDockerClient
}

// prepareDockerScratchPush - Prepares stepConfig for docker-scratch-push step from input stepData
func prepareDockerScratchPush(stepData map[string]string) (*DockerScratchPushStep, context.Context, *core.NormalizedEmitter, *OfficialDockerClient) {
	config := &core.StepConfig{
		ID:   "internal/docker-scratch-push",
		Data: stepData,
	}
	options := &core.PipelineOptions{}
	step, _ := NewDockerScratchPushStep(config, options, nil)
	step.configure(&util.Environment{})
	step.dockerOptions = &Options{}
	step.authenticator = &auth.DockerAuth{}
	step.logger = util.NewLogger().WithFields(util.LogFields{
		"Logger": "Test",
	})
	mockEmittor := core.NewNormalizedEmitter()
	mockDockerClient := &OfficialDockerClient{}
	ctx := context.WithValue(context.Background(), "Emitter", mockEmittor)
	return step, ctx, mockEmittor, mockDockerClient
}
