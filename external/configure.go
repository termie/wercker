// Copyright (c) 2018, Oracle and/or its affiliates. All rights reserved.

package external

import (
	"fmt"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	context "golang.org/x/net/context"
)

// Get the Docker client
func (cp *RunnerParams) getDockerClient() error {
	context.Background()
	cli, err := docker.NewClient(cp.DockerEndpoint)
	if err != nil {
		cp.Logger.Fatal(fmt.Sprintf("unable to create the Docker client: %s", err))
		return err
	}
	cp.client = cli
	return nil
}

// Describe the local image and return the Image structure
func (cp *RunnerParams) getLocalImage() (*docker.Image, error) {
	// Handle image override when development and an overriding image name supplied.
	if !cp.ProdType && cp.OverrideImage != "" {
		cp.ImageName = cp.OverrideImage
		image, err := cp.client.InspectImage(cp.ImageName)
		if err != nil {
			return nil, err
		}
		return image, err
	}

	opts := docker.ListImagesOptions{
		All: true,
	}

	// Find the image containing 'wercker/wercker-runner:external-runner"
	images, err := cp.client.ListImages(opts)
	if err != nil {
		return nil, err
	}

	// Dynamically figure out the image name based on a known static string embedded in
	// the repository tag. This allows different repository prefixs and version information
	// in the tail end of the tag. When more than one instance is found then take the
	// most recent image.

	taggedLatest := "" // To remember a latest image
	latestMaster := "" // To remember latest master.
	var imageName string
	var latest int64 = 0
	for _, image := range images {
		for _, slice := range image.RepoTags {
			if !strings.Contains(slice, "wercker/wercker-runner:") {
				continue
			}

			if cp.Debug {
				tokens := strings.Split(slice, ":")
				ima, err := cp.client.InspectImage(slice)
				if err == nil {
					message := fmt.Sprintf("Local: %s --> %s", ima.Created, tokens[1])
					cp.Logger.Debugln(message)
				}
			}

			if cp.ProdType && strings.HasSuffix(slice, ":latest") {
				// Remember latest present for production
				taggedLatest = slice
			}

			if latest < image.Created {
				latest = image.Created
				imageName = slice
				// Remember latest from master branch
				if strings.Contains(slice, ":master") {
					latestMaster = slice
				}
			}
		}
	}
	// Nothing was found.
	if imageName == "" {
		return nil, nil
	}

	// Decide what image is to be used according to development or production.
	if cp.ProdType {
		// Production must be either latest or the most recent master
		if taggedLatest != "" {
			// When tagged as latest foind then use it.
			imageName = taggedLatest
		} else {
			if latestMaster != "" {
				// Using latest master branch
				imageName = latestMaster
			} else {
				// Nothing there for production.
				return nil, nil
			}
		}
	}
	cp.ImageName = imageName

	image, err := cp.client.InspectImage(cp.ImageName)
	if err != nil {
		return nil, err
	}
	return image, err
}

// Check the external runner images between local and remote repositories.
// If local exists but remote does not then do nothing
// If local exists and is the same as the remote then do nothing
// If local is older than remote then give user the option to download the remote
// If neither exists then fail immediately
func (cp *RunnerParams) CheckRegistryImages(isInternal bool) error {

	cp.Logger.Debug("CheckRegisterImages Running with ProdType: ", cp.ProdType)

	err := cp.getDockerClient()
	if err != nil {
		cp.Logger.Fatal(err)
	}

	// Get the local image for the runner
	localImage, err := cp.getLocalImage()
	if err != nil {
		cp.Logger.Fatal(err)
		return err
	}

	// Get the latest image from the OCIR repository
	remoteImage, err := cp.getRemoteImage()
	if err != nil {
		if isInternal {
			return nil
		}
		cp.Logger.Fatalln("Unable to access remote repository", err)
		return err
	}

	// See if there is a remote image available to check against.
	if remoteImage.ImageName != "" {
		// See if remote image is newer
		if localImage == nil && cp.PullRemote {
			return cp.pullNewerImage(remoteImage.ImageName)
		}

		if localImage != nil && remoteImage.Created.After(localImage.Created) &&
			remoteImage.ImageName != cp.ImageName {

			// Remote has an image that is newer
			if cp.PullRemote {
				return cp.pullNewerImage(remoteImage.ImageName)
			} else {
				message := "There is a newer runner image available from Oracle"
				cp.Logger.Info(message)
				if !isInternal {
					cp.Logger.Info(fmt.Sprintf("Image: %s", remoteImage.ImageName))
					cp.Logger.Info(fmt.Sprintf("Created: %s", remoteImage.Created))
					cp.Logger.Infoln("Execute \"wercker runner configure --pull\" to update your system.")
				}
				return nil
			}
		}
	}

	if localImage == nil {
		cp.Logger.Infoln("No Docker runner image exists in the local repository.")
		cp.Logger.Fatal("Execute \"wercker runner configure --pull\" to pull the required image.")
	} else {
		message := "Local Docker repository runner image is up-to-date."
		cp.Logger.Infoln(message)
		if !isInternal {
			cp.Logger.Infoln(fmt.Sprintf("Image: %s", cp.ImageName))
			cp.Logger.Infoln(fmt.Sprintf("Created: %s", localImage.Created))
		}
	}
	return nil
}
