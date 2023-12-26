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
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cheggaaa/pb"
	"github.com/wercker/wercker/util"
)

// Updater data structure for versions
type Updater struct {
	CurrentVersion *util.Versions
	ServerVersion  *util.Versions
	channel        string
	l              *util.LogEntry
}

// NewUpdater constructor
func NewUpdater(channel string) (*Updater, error) {
	serverVersion, err := getServerVersion(channel)
	if err != nil {
		return nil, err
	}
	return &Updater{
		CurrentVersion: util.GetVersions(),
		ServerVersion:  serverVersion,
		channel:        channel,
		l:              util.RootLogger().WithField("Logger", "Updater"),
	}, nil
}

// DownloadURL returns the url to download the latest version
func (u *Updater) DownloadURL() string {
	return fmt.Sprintf("https://s3.amazonaws.com/downloads.wercker.com/cli/%s/%s_%s/wercker", u.channel, runtime.GOOS, runtime.GOARCH)
}

// DownloadVersionURL returns the url to download the specified version
func (u *Updater) DownloadVersionURL(version string) string {
	return fmt.Sprintf("https://s3.amazonaws.com/downloads.wercker.com/cli/versions/%s/%s_%s/wercker", version, runtime.GOOS, runtime.GOARCH)
}

// UpdateAvailable returns true if there's an update available
func (u *Updater) UpdateAvailable() bool {
	return u.ServerVersion.CompiledAt.After(u.CurrentVersion.CompiledAt)
}

// Update replaces the inode of the current executable with the latest version
// n.b. this won't work on Windows
func (u *Updater) Update() error {
	u.l.Infoln("Downloading version", u.ServerVersion.Version)
	werckerPath, err := filepath.Abs(os.Args[0])
	if err != nil {
		return err
	}

	// Put new version in tempfile in parent directory.
	temp, err := ioutil.TempFile(filepath.Dir(werckerPath), fmt.Sprintf(".%s-", u.ServerVersion.Version))
	if err != nil {
		return err
	}
	defer temp.Close()

	newVersion, err := http.Get(u.DownloadURL())
	if err != nil {
		return err
	}
	defer newVersion.Body.Close()

	bar := pb.New(int(newVersion.ContentLength)).SetUnits(pb.U_BYTES)
	bar.Start()
	writer := io.MultiWriter(temp, bar)

	_, err = io.Copy(writer, newVersion.Body)
	if err != nil {
		return err
	}

	temp.Chmod(0755)

	return os.Rename(temp.Name(), werckerPath)
}

func getServerVersion(channel string) (*util.Versions, error) {
	logger := util.RootLogger().WithField("Logger", "getServerVersion")

	url := fmt.Sprintf("https://s3.amazonaws.com/downloads.wercker.com/cli/%s/version.json", channel)

	nv := &util.Versions{}
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.WithField("Error", err).Debug("Unable to create request to version endpoint")
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		logger.WithField("Error", err).Debug("Unable to execute HTTP request to version endpoint")
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logger.WithField("Error", err).Debug("Unable to read response body")
		return nil, err
	}

	err = json.Unmarshal(body, nv)
	if err != nil {
		logger.WithField("Error", err).Debug("Unable to unmarshal versions")
		return nil, err
	}
	return nv, nil
}

// AskForUpdate asks users if they want to update and returns the answer
func AskForUpdate() bool {
	fmt.Println("Would you like update? [yN]")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		util.RootLogger().Errorln("Problem reading answer", err)
		return false
	}
	return strings.HasPrefix(strings.ToLower(line), "y")
}
