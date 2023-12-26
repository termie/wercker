//   Copyright 2017 Wercker Holding BV
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

package steps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// Publisher contains the steps to publish a step.
type Publisher interface {
	// CreateDraft implements the first step of the publication flow. It requires
	// a manifest, the checksum and size of the tarball.
	CreateDraft(req *PublishStepRequest) (*PublishStepResponse, error)

	// UploadTarball uploads the step tarball using the response from the
	// CreateDraft endpoint.
	UploadTarball(uploadURL string, body io.Reader, size int64) error

	// FinishPublish post to the publish endpoint and the tarball is uploaded to
	// indicate that the step can be published.
	FinishPublish(token string) error
}

type PublishStepRequest struct {
	// checksum of the tarball containing the step
	Checksum string `json:"checksum,omitempty"`
	// size of the tarball containing the step
	Size int64 `json:"size,omitempty"`
	// manifest contains the manifest of the step
	Manifest *StepManifest `json:"manifest,omitempty"`
	// username
	Username string `json:"username,omitempty"`
	// specifies whether the step is private or public
	Private bool `json:"private,omitempty"`
}

type PublishStepResponse struct {
	// uploadUrl is the URL the client has to post the tarball to
	UploadUrl string `json:"uploadUrl,omitempty"`
	// token is the token to send to the done endpoint to notify the upload has
	// been finished
	Token string `json:"token,omitempty"`
	// expires is the expiration date of the uploadUrl
	Expires string `json:"expires,omitempty"`
}

// PublishStep uses ps to create a new step using manifest, tarball.
func PublishStep(ps Publisher, manifest *StepManifest, tarball io.Reader, username, checksum string, size int64, private bool) error {
	createDraftRequest := &PublishStepRequest{
		Username: username,
		Manifest: manifest,
		Checksum: checksum,
		Size:     size,
		Private:  private,
	}

	resp, err := ps.CreateDraft(createDraftRequest)
	if err != nil {
		return err
	}

	err = ps.UploadTarball(resp.UploadUrl, tarball, size)
	if err != nil {
		return err
	}

	err = ps.FinishPublish(resp.Token)
	if err != nil {
		return err
	}

	return nil
}

// NewRESTPublisher creates a publisher that uses the REST API.
func NewRESTPublisher(endpoint string, client *http.Client, stepsClient *http.Client) *RESTPublisher {
	ps := &RESTPublisher{
		endpoint:    endpoint,
		client:      client,
		stepsClient: stepsClient,
	}

	return ps
}

// RESTPublisher contains the steps to publish a step.
type RESTPublisher struct {
	endpoint    string
	client      *http.Client
	stepsClient *http.Client
}

var _ Publisher = (*RESTPublisher)(nil)

// generateURL takes s.endpoint and appends slugs to it.
func (s *RESTPublisher) generateURL(slugs ...string) string {
	path := strings.Join(slugs, "/")
	return fmt.Sprintf("%s/%s", s.endpoint, path)
}

// CreateDraft implements the first step of the publication flow. It requires a
// manifest, the checksum and size of the tarball.
func (s *RESTPublisher) CreateDraft(createDraftRequest *PublishStepRequest) (*PublishStepResponse, error) {
	log.Debug("Creating draft")

	b, err := json.Marshal(createDraftRequest)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to marshal create draft request")
	}

	req, err := http.NewRequest("POST", s.generateURL("api", "publish"), bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.stepsClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to make request")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "Unable to read response body")
		}
		defer resp.Body.Close()

		e := fmt.Sprintf("Received error: %d: %s", resp.StatusCode, string(respBody))
		log.Errorf(e)
		return nil, errors.New(e)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to read response body")
	}
	defer resp.Body.Close()

	var createDraftResponse PublishStepResponse
	err = json.Unmarshal(respBody, &createDraftResponse)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to unmarshal response body")
	}

	return &createDraftResponse, nil
}

// UploadTarball uploads the step tarball using the response from the
// CreateDraft endpoint.
func (s *RESTPublisher) UploadTarball(uploadURL string, body io.Reader, size int64) error {
	log.WithField("url", uploadURL).Debug("Uploading tarball to UploadUrl")

	req, err := http.NewRequest("PUT", uploadURL, body)
	if err != nil {
		return errors.Wrap(err, "Unable to create request")
	}
	req.ContentLength = size

	resp, err := s.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "Unable to make request")
	}

	log.Debugf("Received status code: %d", resp.StatusCode)
	if resp.StatusCode != 200 {
		respBody, _ := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()

		if len(respBody) > 0 {
			log.Errorf("Error body: %s", respBody)
		}

		return errors.New("Did not receive expected status code")
	}

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	return nil
}

// FinishPublish post to the publish endpoint and the tarball is uploaded to
// indicate that the step can be published.
func (s *RESTPublisher) FinishPublish(token string) error {
	u := s.generateURL("api", "publish", "done")
	log.WithField("url", u).Debug("Finishing publication of step")

	payload := bytes.NewBuffer([]byte(fmt.Sprintf(`{"token": "%s"}`, token)))
	req, err := http.NewRequest("POST", u, payload)
	if err != nil {
		return errors.Wrap(err, "Unable to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.stepsClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "Unable to make request")
	}

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	log.Debugf("Received status code: %d", resp.StatusCode)

	if resp.StatusCode != 200 {
		return errors.New("Did not receive expected status code")
	}

	return nil
}
