package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types"
	"github.com/kr/pty"
	"github.com/wercker/wercker/core"
)

var errNoWerckerToken = errors.New("wercker auth token could not be found, please run wercker login")

func ensureWerckerCredentials(opts *core.WerckerDockerOptions) error {
	if opts.AuthToken == "" {
		return errNoWerckerToken
	}
	dockerConfig := config.LoadDefaultConfigFile(os.Stderr)
	_, hasAuth := dockerConfig.AuthConfigs[opts.WerckerContainerRegistry.String()]
	if !hasAuth {
		dockerConfig.AuthConfigs[opts.WerckerContainerRegistry.String()] = types.AuthConfig{
			Username: "token",
			Password: opts.AuthToken,
		}
		err := dockerConfig.Save()
		if err != nil {
			return fmt.Errorf("Could not inject wercker token into docker config: %v", err)
		}
	}
	return nil
}

func runDocker(args []string) error {
	dockerCmd := exec.Command("docker", args...)
	// run using a pseudo-terminal so that we get the nice docker output :)
	outFile, err := pty.Start(dockerCmd)
	if err != nil {
		return err
	}
	// Stream output of the command to stdout
	io.Copy(os.Stdout, outFile)
	return nil
}
