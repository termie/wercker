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
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/mgo.v2/bson"

	"github.com/docker/docker/api/types"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

type DockerSuite struct {
	*util.TestSuite
}

func TestDockerSuite(t *testing.T) {
	suiteTester := &DockerSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *DockerSuite) TestPing() {
	ctx := context.Background()
	client := DockerOrSkip(ctx, s.T())
	_, err := client.Ping(ctx)
	s.Nil(err)
}

func (s *DockerSuite) TestGenerateDockerID() {
	id, err := GenerateDockerID()
	s.Require().NoError(err, "Unable to generate Docker ID")

	// The ID needs to be a valid hex value
	b, err := hex.DecodeString(id)
	s.Require().NoError(err, "Generated Docker ID was not a hex value")

	// The ID needs to be 256 bits
	s.Equal(256, len(b)*8)
}

//TestCreateContainerWithRetries - Verifies that CreateContainerWithRetries is able to create the container
// even after few "no such image" errors due to image not being available
func (s *DockerSuite) TestCreateContainerWithRetries() {
	repoName := "alpine"
	originalTag := "3.1"
	testTag := bson.NewObjectId().Hex()
	testContainerName := bson.NewObjectId().Hex()

	// Check of docker is available
	client, err := NewDockerClient(MinimalDockerOptions())
	err = client.Ping()
	if err != nil {
		s.Skip("Docker not available, skipping test")
		return
	}

	// Check if alpine base image is available, if not pull the image
	_, err = client.InspectImage(repoName + ":" + originalTag)
	if err == docker.ErrNoSuchImage {
		options := docker.PullImageOptions{
			Repository: repoName,
			Tag:        originalTag,
		}
		err := client.PullImage(options, docker.AuthConfiguration{})
		s.NoError(err, "Unable to pull image")
		_, err = client.InspectImage(repoName + ":" + originalTag)
		s.NoError(err, "Unable to verify pulled image")
		// Cleanup image we just pulled
		defer func() {
			client.RemoveImage(repoName + ":" + originalTag)
		}()
	}

	// Check if image is already tagged to our test tag, if yes remove
	// We create a separate test tag so that we do not touch the original image
	_, err = client.InspectImage(repoName + ":" + testTag)
	if err == nil {
		client.RemoveImage(repoName + ":" + testTag)
	}

	// Check if there is a container with name same as test container, if yes remove
	existingContainerID := getContainerIDByContainerName(client, testContainerName)
	if existingContainerID != nil {
		client.RemoveContainer(docker.RemoveContainerOptions{Force: true, ID: *existingContainerID})
	}

	finished := make(chan int, 1)
	// Fire off CreateContainerWithRetries in a separate thread
	conf := &docker.Config{
		Image: repoName + ":" + testTag,
	}
	go func() {
		defer func() {
			finished <- 0
		}()
		container, err := client.CreateContainerWithRetries(docker.CreateContainerOptions{Name: testContainerName, Config: conf})

		s.NoError(err, "Error while creating container")
		s.NotNil(container, "Container is nil")
		s.Equal(testContainerName, container.Name, "Container created with a different name")

		//cleanup
		client.RemoveContainer(docker.RemoveContainerOptions{Force: true, ID: container.ID})
	}()

	// Wait for some time before making the image tag available
	// docker should respond with "no such image" during this time
	time.Sleep(4 * time.Second)

	// Now tag the image to make it available to CreateContainerWithRetries
	err = client.TagImage(repoName+":"+originalTag, docker.TagImageOptions{Repo: repoName, Tag: testTag})
	s.NoError(err, "Unable to tag image for testing")
	<-finished

	// Cleanup image we are testing
	client.RemoveImage(repoName + ":" + testTag)

}

// TestFixForGCR_UsernameNotJsonKey verifies that fixForGCR does not change the authEncodedJSON
// when username is not _json_key
func (s *DockerSuite) TestFixForGCR_UsernameNotJsonKey() {
	username := "anyuser"
	password := "password"
	repository := "docker.io/test"
	email := "test@abc.net"
	authConfig := types.AuthConfig{
		Username: username,
		Password: password,
		Email:    email,
	}
	authEncodedJSON, _ := json.Marshal(authConfig)
	authEncodedJSONAfterFix, err := fixForGCR(authEncodedJSON, authConfig.Username, repository)
	s.NoError(err, "No error should be returned")
	s.Equal(authEncodedJSON, authEncodedJSONAfterFix, "fixForGCR should not change the json")
}

// TestFixForGCR_PushNotForGCR verifies that fixForGCR does not change the authEncodedJSON
// when repository is not a gcr repository
func (s *DockerSuite) TestFixForGCR_PushNotForGCR() {
	username := "_json_key"
	password := `{\n  "type": "service_account",\n  "project_id": "XXXXXXXXXXXXXXXXXX",\n  "private_key_id": "a4c5223051e9624e730116c50939eeecf1bf6c10",\n  "private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC4ryM288pcOQj1\nYh5DQamqoTab5icUKBIK3B0G5xg2yZ2sg9yanWvYsxYtBmLwdhsV0NAZPr3e1KB7\nmb5oYVvpnUBchYJXL76IbnQDteoYT4OyXz58GXsPnfm2a6bHpQC4d5dQJkcHH732\ncHY9ViMnljLVlpcf51xOerMQ4VAC9wQydFEMeNzL48ewnqFb9krhmqIHEWXdyGrW\ndSruQ8WsDOEAs7JpR+iLNnGeuLrKrDVj2OFPH+nTQ8ap3+ppjJz0uYKcC00lKwE6\nlb8GymFDETRowedz7WreNjqWp8PQQ/fWbxz3YCrN5A7G8HV39td5Tt6X5QRGLkvs\nBV0AWtQxAgMBAAECggEAUUui3C8vbi4bD+0Ldj6iwX3qjHCg1iIXYxlmW6IBSiiw\n0/5NbvAJx595jQNJLSFIJe+/ksVIDh0ZsZ7JLqhgrbKvYKrSZ6+YFvVL81AyBlaG\nGdAMMNOElKjNAaxcg3hSG2FlRX47+NpTo/X4TmKq4eOfZ968kmok+1TOmwkbT9oN\nxwFVvDYvrLHhzdC+19/TmvkReoGBKtIy3ve9ZuQm+0clAscrUf36yZyjZiOHDBkK\ncx1hLgKnUBlKjcSJ9/407+qxPVYMRWrX3b+IMyiEDwn1OCJvATy+U5VP6rXE9ESB\nl3C0iPDhPdysgosDHlRriMD8wOR+ZrVvRisTuVDP8wKBgQDf+09ZaxdHhsQeogGd\nQBva53PPmTFvRaQu62LtTc5/o4FkrJVhyn7PyyQVLX6Zd46ZPIkHKEPcQSOuIKUm\nEmmHPPRiDO3l6BfxWSEqkYkSljll7TR6cFEc/z5hIwtjg8G/s9polcGvmHG3EING\n1S7/HjWujmQFPCjVamgWOADw3wKBgQDTFbg92Xldw7kqlFWaAIG3RIycatKGzUjL\nAlZ2+3yORttJVu8uGoaNjF9uZE6ojJZEwdOZ20phQRnUWR+CL1FvHysHnTeqLfKj\neKjRHTuHSTgxbSGEdkTDHFJGYQt+hxKXOo9hZlc8NeL8NP2t+5mAjjUqyOmmBLPv\nBuFRhECM7wKBgBZM0iin9ehkLZCTNq/uWxefZbNsoDRg7ajSPMY9seqZX9+jIzha\nTefoZM5K+kjTU3pEQaxZwO/j+GZ0z5yLxr/1PKuqd+ElC4U3B4tSdCBKnqpcRJZQ\nKnNFonNPZungi2DHyl4RUvhlqCS+2yMpRIWX/2ZCvQicZcBh2L0llEpnAoGBAIBF\nNZWYHxFki5QdWbtgzXKh3FR88XvrKW37+LEK99C5rC3v/x5kDhncEG3T1JzF+dbE\ndiKLyLI6zkhk9Cm3OWQua4aP+jCXBVhjTSrt+aunSdd3OqP0/qoV/sU32bVEvX5a\nnqCQgThcgpfCV9mvB8PAJvzd5GX3e6Qn6SoRFOzPAoGATJWS0k7+pTZZ3qMptjfd\nicVa8R53plZPD90M1io/RkVfY7+zzb3008kWM/TVEGynFsKJY8jVB7gtPU407vgL\nG0GjijtgqyR6NjMcML9ZDGIqusBXCbWqhwG8TAkE8Rjt8nCfRCn7iJiGUPGnKPrZ\nlAyUAwQXlQ6o0bmI/ixYyGA=\n-----END PRIVATE KEY-----\n",\n  "client_email": "457010517207-compute@developer.gserviceaccount.com",\n  "client_id": "100527145131329464627",\n  "auth_uri": "https://accounts.google.com/o/oauth2/auth",\n  "token_uri": "https://oauth2.googleapis.com/token",\n  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",\n  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/457010517207-compute%40developer.gserviceaccount.com"\n}`
	repository := "eu.mycr.io/hopeful-theorem-220606/test"
	email := "test@abc.net"
	authConfig := types.AuthConfig{
		Username: username,
		Password: password,
		Email:    email,
	}
	authEncodedJSON, _ := json.Marshal(authConfig)
	authEncodedJSONAfterFix, err := fixForGCR(authEncodedJSON, authConfig.Username, repository)
	s.NoError(err, "No error should be returned")
	s.Equal(authEncodedJSON, authEncodedJSONAfterFix, "fixForGCR should not change the json")
}

// TestFixForGCR_PushNotForGCR verifies that fixForGCR correctly replaces "\\n" with "\n" in authEncodedJSON
// i.e. for cases when the environment variable containing the json password already contains literal \n for newlines
// before being passed to json.Marshal (e.g. for cases where variable value is updated after SyncEnvironment)
func (s *DockerSuite) TestFixForGCR_PushForGCR() {
	username := "_json_key"
	password := `{\n  "type": "service_account",\n  "project_id": "XXXXXXXXXXXXXXXXXX",\n  "private_key_id": "a4c5223051e9624e730116c50939eeecf1bf6c10",\n  "private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC4ryM288pcOQj1\nYh5DQamqoTab5icUKBIK3B0G5xg2yZ2sg9yanWvYsxYtBmLwdhsV0NAZPr3e1KB7\nmb5oYVvpnUBchYJXL76IbnQDteoYT4OyXz58GXsPnfm2a6bHpQC4d5dQJkcHH732\ncHY9ViMnljLVlpcf51xOerMQ4VAC9wQydFEMeNzL48ewnqFb9krhmqIHEWXdyGrW\ndSruQ8WsDOEAs7JpR+iLNnGeuLrKrDVj2OFPH+nTQ8ap3+ppjJz0uYKcC00lKwE6\nlb8GymFDETRowedz7WreNjqWp8PQQ/fWbxz3YCrN5A7G8HV39td5Tt6X5QRGLkvs\nBV0AWtQxAgMBAAECggEAUUui3C8vbi4bD+0Ldj6iwX3qjHCg1iIXYxlmW6IBSiiw\n0/5NbvAJx595jQNJLSFIJe+/ksVIDh0ZsZ7JLqhgrbKvYKrSZ6+YFvVL81AyBlaG\nGdAMMNOElKjNAaxcg3hSG2FlRX47+NpTo/X4TmKq4eOfZ968kmok+1TOmwkbT9oN\nxwFVvDYvrLHhzdC+19/TmvkReoGBKtIy3ve9ZuQm+0clAscrUf36yZyjZiOHDBkK\ncx1hLgKnUBlKjcSJ9/407+qxPVYMRWrX3b+IMyiEDwn1OCJvATy+U5VP6rXE9ESB\nl3C0iPDhPdysgosDHlRriMD8wOR+ZrVvRisTuVDP8wKBgQDf+09ZaxdHhsQeogGd\nQBva53PPmTFvRaQu62LtTc5/o4FkrJVhyn7PyyQVLX6Zd46ZPIkHKEPcQSOuIKUm\nEmmHPPRiDO3l6BfxWSEqkYkSljll7TR6cFEc/z5hIwtjg8G/s9polcGvmHG3EING\n1S7/HjWujmQFPCjVamgWOADw3wKBgQDTFbg92Xldw7kqlFWaAIG3RIycatKGzUjL\nAlZ2+3yORttJVu8uGoaNjF9uZE6ojJZEwdOZ20phQRnUWR+CL1FvHysHnTeqLfKj\neKjRHTuHSTgxbSGEdkTDHFJGYQt+hxKXOo9hZlc8NeL8NP2t+5mAjjUqyOmmBLPv\nBuFRhECM7wKBgBZM0iin9ehkLZCTNq/uWxefZbNsoDRg7ajSPMY9seqZX9+jIzha\nTefoZM5K+kjTU3pEQaxZwO/j+GZ0z5yLxr/1PKuqd+ElC4U3B4tSdCBKnqpcRJZQ\nKnNFonNPZungi2DHyl4RUvhlqCS+2yMpRIWX/2ZCvQicZcBh2L0llEpnAoGBAIBF\nNZWYHxFki5QdWbtgzXKh3FR88XvrKW37+LEK99C5rC3v/x5kDhncEG3T1JzF+dbE\ndiKLyLI6zkhk9Cm3OWQua4aP+jCXBVhjTSrt+aunSdd3OqP0/qoV/sU32bVEvX5a\nnqCQgThcgpfCV9mvB8PAJvzd5GX3e6Qn6SoRFOzPAoGATJWS0k7+pTZZ3qMptjfd\nicVa8R53plZPD90M1io/RkVfY7+zzb3008kWM/TVEGynFsKJY8jVB7gtPU407vgL\nG0GjijtgqyR6NjMcML9ZDGIqusBXCbWqhwG8TAkE8Rjt8nCfRCn7iJiGUPGnKPrZ\nlAyUAwQXlQ6o0bmI/ixYyGA=\n-----END PRIVATE KEY-----\n",\n  "client_email": "457010517207-compute@developer.gserviceaccount.com",\n  "client_id": "100527145131329464627",\n  "auth_uri": "https://accounts.google.com/o/oauth2/auth",\n  "token_uri": "https://oauth2.googleapis.com/token",\n  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",\n  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/457010517207-compute%40developer.gserviceaccount.com"\n}`
	repository := "eu.gcr.io/hopeful-theorem-220606/test"
	email := "test@abc.net"
	authConfig := types.AuthConfig{
		Username: username,
		Password: password,
		Email:    email,
	}
	authEncodedJSON, _ := json.Marshal(authConfig)
	num := bytes.Count(authEncodedJSON, []byte("\\\\n"))
	s.NotZero(num, "There should be non-zero instances of \\\\n in encoded json")
	authEncodedJSONAfterFix, err := fixForGCR(authEncodedJSON, authConfig.Username, repository)
	s.NoError(err, "No error should be returned")
	s.NotEqual(authEncodedJSON, authEncodedJSONAfterFix, "fixForGCR should change the json")
	num = bytes.Count(authEncodedJSONAfterFix, []byte("\\\\n"))
	s.Zero(num, "There should be zero instances of \\\\n in encoded json after fix")
}

// TestFixForGCR_PushForGCRNoExtraSlash verifies that fixForGCR retains "\n" in authEncodedJSON
// i.e. for cases when the environment variable containing the json password does not already contains literal \n for newlines
// before being passed to json.Marshal (e.g. for cases where variable value is sourced as is from envvars)
func (s *DockerSuite) TestFixForGCR_PushForGCRNoExtraSlash() {
	username := "_json_key"
	password := `{
		"type": "service_account",
		"project_id": "XXXXXXXXXXXXXXXXXX",
		"private_key_id": "a4c5223051e9624e730116c50939eeecf1bf6c10",
		"private_key": "-----BEGIN PRIVATE KEY-----
	  MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC4ryM288pcOQj1
	  Yh5DQamqoTab5icUKBIK3B0G5xg2yZ2sg9yanWvYsxYtBmLwdhsV0NAZPr3e1KB7
	  mb5oYVvpnUBchYJXL76IbnQDteoYT4OyXz58GXsPnfm2a6bHpQC4d5dQJkcHH732
	  cHY9ViMnljLVlpcf51xOerMQ4VAC9wQydFEMeNzL48ewnqFb9krhmqIHEWXdyGrW
	  dSruQ8WsDOEAs7JpR+iLNnGeuLrKrDVj2OFPH+nTQ8ap3+ppjJz0uYKcC00lKwE6
	  lb8GymFDETRowedz7WreNjqWp8PQQ/fWbxz3YCrN5A7G8HV39td5Tt6X5QRGLkvs
	  BV0AWtQxAgMBAAECggEAUUui3C8vbi4bD+0Ldj6iwX3qjHCg1iIXYxlmW6IBSiiw
	  0/5NbvAJx595jQNJLSFIJe+/ksVIDh0ZsZ7JLqhgrbKvYKrSZ6+YFvVL81AyBlaG
	  GdAMMNOElKjNAaxcg3hSG2FlRX47+NpTo/X4TmKq4eOfZ968kmok+1TOmwkbT9oN
	  xwFVvDYvrLHhzdC+19/TmvkReoGBKtIy3ve9ZuQm+0clAscrUf36yZyjZiOHDBkK
	  cx1hLgKnUBlKjcSJ9/407+qxPVYMRWrX3b+IMyiEDwn1OCJvATy+U5VP6rXE9ESB
	  l3C0iPDhPdysgosDHlRriMD8wOR+ZrVvRisTuVDP8wKBgQDf+09ZaxdHhsQeogGd
	  QBva53PPmTFvRaQu62LtTc5/o4FkrJVhyn7PyyQVLX6Zd46ZPIkHKEPcQSOuIKUm
	  EmmHPPRiDO3l6BfxWSEqkYkSljll7TR6cFEc/z5hIwtjg8G/s9polcGvmHG3EING
	  1S7/HjWujmQFPCjVamgWOADw3wKBgQDTFbg92Xldw7kqlFWaAIG3RIycatKGzUjL
	  AlZ2+3yORttJVu8uGoaNjF9uZE6ojJZEwdOZ20phQRnUWR+CL1FvHysHnTeqLfKj
	  eKjRHTuHSTgxbSGEdkTDHFJGYQt+hxKXOo9hZlc8NeL8NP2t+5mAjjUqyOmmBLPv
	  BuFRhECM7wKBgBZM0iin9ehkLZCTNq/uWxefZbNsoDRg7ajSPMY9seqZX9+jIzha
	  TefoZM5K+kjTU3pEQaxZwO/j+GZ0z5yLxr/1PKuqd+ElC4U3B4tSdCBKnqpcRJZQ
	  KnNFonNPZungi2DHyl4RUvhlqCS+2yMpRIWX/2ZCvQicZcBh2L0llEpnAoGBAIBF
	  NZWYHxFki5QdWbtgzXKh3FR88XvrKW37+LEK99C5rC3v/x5kDhncEG3T1JzF+dbE
	  diKLyLI6zkhk9Cm3OWQua4aP+jCXBVhjTSrt+aunSdd3OqP0/qoV/sU32bVEvX5a
	  nqCQgThcgpfCV9mvB8PAJvzd5GX3e6Qn6SoRFOzPAoGATJWS0k7+pTZZ3qMptjfd
	  icVa8R53plZPD90M1io/RkVfY7+zzb3008kWM/TVEGynFsKJY8jVB7gtPU407vgL
	  G0GjijtgqyR6NjMcML9ZDGIqusBXCbWqhwG8TAkE8Rjt8nCfRCn7iJiGUPGnKPrZ
	  lAyUAwQXlQ6o0bmI/ixYyGA=
	  -----END PRIVATE KEY-----
	  ",
		"client_email": "457010517207-compute@developer.gserviceaccount.com",
		"client_id": "100527145131329464627",
		"auth_uri": "https://accounts.google.com/o/oauth2/auth",
		"token_uri": "https://oauth2.googleapis.com/token",
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/457010517207-compute%40developer.gserviceaccount.com"
	  }`
	repository := "eu.gcr.io/hopeful-theorem-220606/test"
	email := "test@abc.net"
	authConfig := types.AuthConfig{
		Username: username,
		Password: password,
		Email:    email,
	}
	authEncodedJSON, _ := json.Marshal(authConfig)
	num1 := bytes.Count(authEncodedJSON, []byte("\\n"))
	s.NotZero(num1, "There should be non zero instances of \\n in encoded json")
	num2 := bytes.Count(authEncodedJSON, []byte("\\\\n"))
	s.Zero(num2, "There should be zero instances of \\\\n in encoded json")
	authEncodedJSONAfterFix, err := fixForGCR(authEncodedJSON, authConfig.Username, repository)
	s.NoError(err, "No error should be returned")
	s.Equal(authEncodedJSON, authEncodedJSONAfterFix, "fixForGCR should not change the json")
}

func getContainerIDByContainerName(client *DockerClient, containerName string) *string {
	listContainerFilter := make(map[string][]string)
	names := make([]string, 1)
	names[0] = containerName
	listContainerFilter["name"] = names

	containers, err := client.ListContainers(docker.ListContainersOptions{All: true, Filters: listContainerFilter})
	if err != nil {
		return nil
	}

	if containers != nil && len(containers) != 0 {
		for _, container := range containers {
			if container.Names != nil && len(container.Names) == 0 && container.Names[0] == containerName {
				return &container.ID
			}
		}
	}
	return nil
}
