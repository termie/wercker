package dockerlocal

import (
	"archive/tar"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

type DockerBuildSuite struct {
	*util.TestSuite
}

func TestDockerBuildSuite(t *testing.T) {
	suiteTester := &DockerBuildSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

// TestBuildFailure verifies that if the docker build fails because of a problem with the dockerfile
// then EmitStatus (which is called at the end of the Execute step) returns an error
func (s *DockerBuildSuite) TestBuildFailure() {
	ctx := context.TODO()
	_ = DockerOrSkip(ctx, s.T())

	filesWithBadDockerfile := filesToTar{
		{"readme.txt", "This is the archive that is sent to the docker daemon"},
		// This Dockerfile has a COPY statement which refers to a non-existent file, so the build will fail and an image will not be created
		{"Dockerfile", "FROM alpine\nCOPY FOOBAR .\nCMD [\"echo\",\"Hello World\"]"},
	}
	code, err := testBuild(ctx, s.T(), filesWithBadDockerfile)
	require.Equal(s.T(), -1, code)
	require.Error(s.T(), err)
	require.True(s.T(), strings.HasPrefix(err.Error(), "COPY failed:"))
	require.True(s.T(), strings.HasSuffix(err.Error(), "no such file or directory"))
}

// TestBuildSuccess is the same as TestBuildFailure but with a good dockerfile.
// It verifies that if the docker build is successful
// then EmitStatus (called at the end of the Execute step) does not return an error
func (s *DockerBuildSuite) TestBuildSuccess() {
	ctx := context.TODO()
	_ = DockerOrSkip(ctx, s.T())

	filesWithGoodDockerfile := filesToTar{
		{"readme.txt", "This is the archive that is sent to the docker daemon"},
		// This Dockerfile is trivial but valid, so the build will succeed and an image will be created
		{"Dockerfile", "FROM alpine\nCMD [\"echo\",\"Hello World\"]"},
	}
	code, err := testBuild(ctx, s.T(), filesWithGoodDockerfile)
	require.Equal(s.T(), 0, code)
	require.NoError(s.T(), err)
}

// testBuild creates a DockerBuildStep and uses its Init() and buildImage() functions
// to execute the docker BuildImage function with the specified tarfile
func testBuild(ctx context.Context, t *testing.T, filesToTar filesToTar) (int, error) {

	ctx = core.NewEmitterContext(ctx)

	_ = DockerOrSkip(ctx, t)

	stepConfig := &core.StepConfig{
		ID: "internal/docker-build",
		Data: map[string]string{
			"dockerfile": "Dockerfile",
			"image-name": "my-image-name",
			//"registry-auth-config": "{\"https://index.docker.io/v1\": {\"Username\": \"wercker\", \"Password\": \"password\"}}",
		},
	}

	env := &util.Environment{}
	pipelineOptions := &core.PipelineOptions{
		GitOptions:    &core.GitOptions{},
		GlobalOptions: &core.GlobalOptions{},
		AWSOptions:    &core.AWSOptions{},
		//WorkingDir:    "./docker_build_test",
	}

	dockerOptions := &Options{}
	guessAndUpdateDockerOptions(ctx, dockerOptions, env)
	step, _ := NewDockerBuildStep(stepConfig, pipelineOptions, dockerOptions)

	step.InitEnv(ctx, env)

	// for key, value := range step.authConfigs {
	// 	require.Equal(t, key, "https://index.docker.io/v1")
	// 	require.Equal(t, value.Username, "wercker")
	// 	require.Equal(t, value.Password, "password")
	// }

	// Create a tarfile for sending to the docker ImageBuild command
	tarfileName := "docker_build_test_data.tar"
	hostPath := step.options.HostPath("")
	os.MkdirAll(hostPath, os.ModePerm)
	defer os.RemoveAll(hostPath)
	createTar(t, step.options.HostPath(tarfileName), filesToTar)

	dockerTransport, err := NewDockerTransport(pipelineOptions, dockerOptions, "dummyContainerID")
	sess := core.NewSession(pipelineOptions, dockerTransport)

	code, err := step.buildImage(ctx, sess, tarfileName)
	return code, err
}

type filesToTar []struct {
	Name, Body string
}

// createTar creates a tarfile with the specified name using the data in the specified filesToTar struct
func createTar(t *testing.T, filename string, files filesToTar) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: 0600,
			Size: int64(len(file.Body)),
		}
		err := tw.WriteHeader(hdr)
		require.NoError(t, err)
		_, err = tw.Write([]byte(file.Body))
		require.NoError(t, err)
	}
	err := tw.Close()
	require.NoError(t, err)

	f, err := os.Create(filename)
	require.NoError(t, err)
	_, err = f.Write(buf.Bytes())
	require.NoError(t, err)
	f.Close()
}
