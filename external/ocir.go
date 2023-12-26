// Copyright (c) 2018, Oracle and/or its affiliates. All rights reserved.

package external

// This module is used to access the remote Docker image repository on ocir.io

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
)

// LatestImage for output
type LatestImage struct {
	ImageName string
	Created   time.Time
}

// Request token for authenticated request
type requestToken struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
}

// CurrentImage item
type CurrentImage struct {
	URL   string `json:"url"`
	Start string `json:"start"`
	Limit int    `json:"limit"`
}

// RemoteImage item
type RemoteImage struct {
	Repo      string `json:"repo"`
	Tag       string `json:"tag"`
	Digest    string `json:"digest"`
	Timestamp string `json:"timestamp"`
}

// List wrapper for remote response payload
type listWrapper struct {
	Imgs []RemoteImage `json:"tags"`
}

// Get the list of remote images from ocir.io and return information about the
// most recently found image.
func (cp *RunnerParams) getRemoteImage() (*LatestImage, error) {

	resultToken, err := cp.getBearerToken()

	if err != nil {
		return nil, err
	}

	url := "https://iad.ocir.io/20180419/docker/images/odx-pipelines?repo=wercker%2Fwercker-runner"

	var client http.Client

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer "+resultToken)
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	latestImageName := ""
	var latestImageTime time.Time
	latestImageDigest := ""

	// I hope this never changes...
	basis := "iad.ocir.io/odx-pipelines/wercker/wercker-runner"

	if resp.StatusCode == 200 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		bodyString := string(bodyBytes)
		theWrapper := listWrapper{}
		json.Unmarshal([]byte(bodyString), &theWrapper)

		for _, imageItem := range theWrapper.Imgs {
			tm, err := time.Parse(time.RFC3339, imageItem.Timestamp)
			if err != nil {
				cp.Logger.Error(err)
				continue
			}

			if cp.Debug {
				message := fmt.Sprintf("Repos: %s --> %s", tm, imageItem.Tag)
				cp.Logger.Debugln(message)
			}

			// For production only ignore any tag that isn't latest or master
			if cp.ProdType {
				if imageItem.Tag != "latest" && !strings.HasPrefix(imageItem.Tag, "master") {
					continue
				}
			}

			// Match the digest from latest with the proper master entry. That will be the
			// image name returned to the caller.
			if cp.ProdType {
				if imageItem.Tag == "latest" {
					latestImageDigest = imageItem.Digest
					cp.Logger.Debugln("Remote latest digest is " + imageItem.Digest)
					continue
				} else {
					// Compare the digest to the latest, when the same we have the image name with commit-id
					// for whatever was tagged as latest.
					if imageItem.Digest == latestImageDigest {
						latestImageTime = tm
						latestImageName = fmt.Sprintf("%s:%s", basis, imageItem.Tag)
						if cp.Debug {
							message := fmt.Sprintf("Selecting %s as doopleganger for discovered latest tag", latestImageName)
							cp.Logger.Debugln(message)
						}
						break
					}
				}
			}

			if tm.After(latestImageTime) {
				latestImageTime = tm
				latestImageName = fmt.Sprintf("%s:%s", basis, imageItem.Tag)
			}
		}
	}

	if latestImageName == "" {
		return nil, fmt.Errorf("no runner image exists in the remote repository")
	}

	return &LatestImage{
		ImageName: latestImageName,
		Created:   latestImageTime,
	}, nil
}

// Obtain the bearer token that is necessary to fetch the image list or pull
// from the remote image repository. This function will return an anonymous
// public token
func (cp *RunnerParams) getBearerToken() (string, error) {

	url := "https://iad.ocir.io/20180419/docker/token"

	var client http.Client

	req, err := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	var resultToken string

	if resp.StatusCode == 200 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		bodyString := string(bodyBytes)
		theToken := requestToken{}
		json.Unmarshal([]byte(bodyString), &theToken)
		resultToken = theToken.Token
	}
	return resultToken, nil
}

// Pull the newer image from OCIR. The older image is left so if there
// is a problem with the newer image it can be removed from the local
// repository as a manual rollback.
func (cp *RunnerParams) pullNewerImage(imageName string) error {

	imageTokens := strings.Split(imageName, ":")

	opts := docker.PullImageOptions{
		Repository: imageTokens[0],
		Tag:        imageTokens[1],
	}
	auth := docker.AuthConfiguration{
		Username: "",
		Password: "",
	}
	cp.Logger.Info("Pulling latest runner Docker image, Please wait...")
	err := cp.client.PullImage(opts, auth)

	if err != nil {
		message := fmt.Sprintf("Failed to update runner Docker image: %s", err)
		cp.Logger.Error(message)
	} else {
		message := fmt.Sprintf("Pulled newer runner Docker image")
		cp.Logger.Infoln(message)
		message = fmt.Sprintf("Image: %s", imageName)
		cp.Logger.Infoln(message)
	}
	return err
}
