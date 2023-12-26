//   Copyright 2016 Wercker Holding BV
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

package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/jtacoma/uritemplates"
	"github.com/wercker/wercker/util"
)

// routes is a map containing all UriTemplates indexed by name.
var routes = make(map[string]*uritemplates.UriTemplate)

func init() {
	// Add templates to the route map
	addURITemplate("GetBuilds", "/api/v3/applications{/username,name}/builds{?commit,branch,status,limit,skip,sort,result,stack}")
	addURITemplate("GetDockerRepository", "/api/v2/builds{/buildId}/docker")
	addURITemplate("GetStepVersion", "/api/v2/steps{/owner,name,version}")
}

type APIOptions struct {
	BaseURL   string
	AuthToken string
}

// addURITemplate adds rawTemplate to routes using name as the key. Should only
// be used from init().
func addURITemplate(name, rawTemplate string) {
	uriTemplate, err := uritemplates.Parse(rawTemplate)
	if err != nil {
		panic(err)
	}
	routes[name] = uriTemplate
}

// APIClient is a very dumb client for the wercker API
type APIClient struct {
	baseURL string
	client  *http.Client
	options *APIOptions
	logger  *util.LogEntry
}

// NewAPIClient returns our dumb client
func NewAPIClient(options *APIOptions) *APIClient {
	logger := util.RootLogger().WithFields(util.LogFields{
		"Logger": "API",
	})
	return &APIClient{
		baseURL: options.BaseURL,
		client:  &http.Client{},
		options: options,
		logger:  logger,
	}
}

// URL joins a path with the baseurl. If path doesn't have a leading slash, it
// will be added.
func (c *APIClient) URL(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.baseURL + path
}

// GetBody does a GET request. If the status code is 200, it will return the
// body.
func (c *APIClient) GetBody(path string) ([]byte, error) {
	res, err := c.Get(path)

	if res.StatusCode != 200 {
		body, _ := ioutil.ReadAll(res.Body)
		c.logger.Debugln(string(body))
		return nil, fmt.Errorf("Got non-200 response: %d", res.StatusCode)
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return buf, nil
}

// Get will do a GET http request, it adds the wercker endpoint and will add
// some default headers.
func (c *APIClient) Get(path string) (*http.Response, error) {
	url := c.URL(path)
	c.logger.Debugln("API Get:", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.logger.WithField("Error", err).Debug("Unable to create request to wercker API")
		return nil, err
	}

	AddRequestHeaders(req)
	c.addAuthToken(req)

	return c.client.Do(req)
}

// GetBuildsOptions are the optional parameters associated with GetBuilds
type GetBuildsOptions struct {
	Sort   string `qs:"sort"`
	Limit  int    `qs:"limit"`
	Skip   int    `qs:"skip"`
	Commit string `qs:"commit"`
	Branch string `qs:"branch"`
	Status string `qs:"status"`
	Result string `qs:"result"`
	Stack  int    `qs:"stack"`
}

// APIBuild represents a build from wercker api.
type APIBuild struct {
	ID         string  `json:"id"`
	URL        string  `json:"url"`
	Status     string  `json:"status"`
	Result     string  `json:"result"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
	FinishedAt string  `json:"finishedAt"`
	Progress   float64 `json:"progress"`
}

// GetBuilds will fetch multiple builds for application username/name.
func (c *APIClient) GetBuilds(username, name string, options *GetBuildsOptions) ([]*APIBuild, error) {
	model := util.QueryString(options)
	model["username"] = username
	model["name"] = name

	template := routes["GetBuilds"]
	url, err := template.Expand(model)
	if err != nil {
		return nil, err
	}

	res, err := c.Get(url)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, c.parseError(res)
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var payload []*APIBuild
	err = json.Unmarshal(buf, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

// DockerRepository represents the meta information of a downloadable docker
// repository. This is a tarball compressed using snappy-stream.
type DockerRepository struct {
	// Content is the compressed tarball. It is the caller's responsibility to
	// close Content.
	Content io.ReadCloser

	// Sha256 checksum of the compressed tarball.
	Sha256 string

	// Size of the compressed tarball.
	Size int64
}

// GetDockerRepository will retrieve a snappy-stream compressed tarball.
func (c *APIClient) GetDockerRepository(buildID string) (*DockerRepository, error) {
	model := make(map[string]interface{})
	model["buildId"] = buildID

	template := routes["GetDockerRepository"]
	url, err := template.Expand(model)
	if err != nil {
		return nil, err
	}

	res, err := c.Get(url)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, c.parseError(res)
	}

	return &DockerRepository{
		Content: res.Body,
		Sha256:  res.Header.Get("x-amz-meta-Sha256"),
		Size:    res.ContentLength,
	}, nil
}

// APIStepVersion is the data structure for the JSON returned by the wercker
// API.
type APIStepVersion struct {
	TarballURL  string `json:"tarballUrl"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// GetStepVersion grabs a step at a specific version
func (c *APIClient) GetStepVersion(owner, name, version string) (*APIStepVersion, error) {
	urlModel := make(map[string]interface{})
	urlModel["owner"] = owner
	urlModel["name"] = name
	urlModel["version"] = version

	template := routes["GetStepVersion"]
	url, err := template.Expand(urlModel)
	if err != nil {
		return nil, err
	}

	res, err := c.Get(url)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, c.parseError(res)
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var payload *APIStepVersion
	err = json.Unmarshal(buf, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

// addAuthToken adds the authentication token to the querystring if available.
// TODO(bvdberg): we should migrate to authentication header.
func (c *APIClient) addAuthToken(req *http.Request) {
	authToken := c.options.AuthToken

	if authToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
	}
}

// AddRequestHeaders add a few default headers to req. Currently added: User-
// Agent, X-Wercker-Version, X-Wercker-Git.
func AddRequestHeaders(req *http.Request) {
	userAgent := fmt.Sprintf("wercker %s", util.FullVersion())

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Wercker-Version", util.Version())
	if util.GitCommit != "" {
		req.Header.Set("X-Wercker-Git", util.GitCommit)
	}
}

// APIError represents a wercker error.
type APIError struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
}

// Error returns the message and status code.
func (e *APIError) Error() string {
	return fmt.Sprintf("wercker-api: %s (status code: %d)", e.Message, e.StatusCode)
}

// parseError will check if res.Body contains a wercker generated error and
// return that, otherwise it will return a generic message based on statuscode.
func (c *APIClient) parseError(res *http.Response) error {
	// Check if the Body contains a wercker JSON error.
	if res.ContentLength > 0 {
		contentType := strings.Trim(res.Header.Get("Content-Type"), " ")

		if strings.HasPrefix(contentType, "application/json") {
			buf, err := ioutil.ReadAll(res.Body)
			if err != nil {
				goto generic
			}
			defer res.Body.Close()

			var payload *APIError
			err = json.Unmarshal(buf, &payload)
			if err == nil && payload.Message != "" && payload.StatusCode != 0 {
				return payload
			}
		}
	}

generic:
	var message string
	switch res.StatusCode {
	case 401:
		message = "authentication required"
	case 403:
		message = "not authorized to access this resource"
	case 404:
		message = "resource not found"
	default:
		message = "unknown error"
	}

	return &APIError{
		Message:    message,
		StatusCode: res.StatusCode,
	}
}

func (c *APIClient) GetTarball(tarballURL string) (*http.Response, error) {
	return util.Get(tarballURL, "")
}
