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

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/pkg/term"
	"github.com/wercker/wercker/api"
	"github.com/wercker/wercker/util"
)

// Request contains the name needed to generate a token.
type Request struct {
	Name string `json:"name"`
}

// Response from authentication endpoint
type Response struct {
	Token string `json:"token"`
}

var authLogger = util.RootLogger().WithField("Logger", "Authentication")

func readUsername() string {
	print("Username: ")
	var input string
	_, err := fmt.Scanln(&input)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to read username")
	}
	return input
}

// readSessionName read the session name that will be associated with the token.
func readSessionName() string {
	component, err := os.Hostname()
	if err != nil {
		component = fmt.Sprintf("%d", time.Now().Unix())
	}
	sessionName := fmt.Sprintf("werckercli-%s", component)
	print(fmt.Sprintf("Sessions name [default: %s]: ", sessionName))

	var input string
	_, err = fmt.Scanln(&input)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to read the session name")
	}
	if input == "" {
		return sessionName
	}
	return input
}

func readPassword() string {
	var oldState *term.State
	var input string
	oldState, err := term.SetRawTerminal(os.Stdin.Fd())
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to Set Raw Terminal")
	}

	print("Password: ")

	term.DisableEcho(os.Stdin.Fd(), oldState)
	defer term.RestoreTerminal(os.Stdin.Fd(), oldState)

	_, err = fmt.Scanln(&input)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to read password")
	}

	if input == "" {
		authLogger.Println("Password required")
		os.Exit(1)
	}
	print("\n")
	return input
}

// retrieves a basic access token from the wercker API
func getAccessToken(username, password, sessionName, url string) (string, error) {
	tokenRequest := &Request{
		Name: sessionName,
	}

	b, _ := json.Marshal(tokenRequest)
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to post request to wercker API")
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(username, password)
	api.AddRequestHeaders(req)

	client := http.DefaultClient
	client.Timeout = 30 * time.Second

	resp, err := client.Do(req)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable read from wercker API")
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to read response")
		return "", err
	}

	var response = &Response{}
	err = json.Unmarshal(body, response)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to serialize response")
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		err := errors.New("Invalid credentials")
		authLogger.WithField("Error", err).Debug("Authentication failed")
		return "", err
	}

	return strings.TrimSpace(response.Token), nil
}

// creates directory when needed, overwrites file when it already exists
func saveToken(path, token string) error {
	err := os.MkdirAll(filepath.Dir(path), 0700)
	if err != nil {
		authLogger.WithField("Error", err).Debug("Unable to create auth store folder")
		return err
	}

	return ioutil.WriteFile(path, []byte(token), 0600)
}

func removeToken(tokenStore string) error {
	err := os.Remove(tokenStore)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
