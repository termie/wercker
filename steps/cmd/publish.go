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

package cmd

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/wercker/wercker/steps"
	"golang.org/x/oauth2"
)

type PublishStepOptions struct {
	Endpoint  string
	AuthToken string
	Owner     string
	Private   bool
	StepDir   string
	TempDir   string
}

// PublishStep publishes the step.
func PublishStep(o *PublishStepOptions) error {
	if err := hasRequiredFiles(o.StepDir); err != nil {
		return errors.Wrap(err, "Not all required files are present")
	}

	manifest, err := parseStepManifest(o.StepDir)
	if err != nil {
		return errors.Wrap(err, "Unable to read or parse step.yml")
	}

	err = steps.ValidateManifest(manifest)
	if err != nil {
		return errors.Wrap(err, "Invalid step.yml")
	}

	path, checksum, err := createTarball(o.TempDir, o.StepDir)
	if err != nil {
		return errors.Wrap(err, "Unable to generate tarball")
	}
	defer os.Remove(path)

	err = publishStep(o, manifest, path, checksum)
	if err != nil {
		return errors.Wrap(err, "Unable to publish step to the registry")
	}

	return nil
}

func hasRequiredFiles(dir string) error {
	files := []string{"step.yml", "run.sh"}
	for _, file := range files {
		_, err := os.Stat(filepath.Join(dir, file))
		if err != nil {
			return errors.New(file + " does not exist")
		}
	}
	return nil
}

func parseStepManifest(stepDir string) (*steps.StepManifest, error) {
	path := filepath.Join(stepDir, "step.yml")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("step.yml does not exist")
		}

		return nil, errors.Wrap(err, "failed to parse step.yml")
	}

	return steps.ParseManifestReader(file)
}

func createTarball(tempDir, stepDir string) (string, string, error) {
	f, err := ioutil.TempFile(tempDir, "step-publish-")
	if err != nil {
		return "", "", errors.Wrap(err, "Unable to create temporary file")
	}
	defer f.Close()

	checksum, err := steps.CreateTarball(stepDir, f)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to create tarball")
	}

	return f.Name(), checksum, nil
}

func publishStep(o *PublishStepOptions, manifest *steps.StepManifest, tarballPath string, checksum string) error {
	file, err := os.Open(tarballPath)
	if err != nil {
		return errors.Wrap(err, "Unable to get open tarball for reading")
	}

	tarballStat, err := file.Stat()
	if err != nil {
		return errors.Wrap(err, "Unable to get stats on the tarball")
	}

	size := tarballStat.Size()

	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: o.AuthToken,
	})
	stepsClient := oauth2.NewClient(oauth2.NoContext, ts)

	ps := steps.NewRESTPublisher(o.Endpoint, http.DefaultClient, stepsClient)

	err = steps.PublishStep(ps, manifest, file, o.Owner, checksum, size, o.Private)
	if err != nil {
		return errors.Wrap(err, "Unable to start publish flow")
	}

	return nil
}
