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
	"errors"

	"github.com/blang/semver"
	"github.com/wercker/wercker/util"
)

// isSemVer checks if the version adheres to the SemVer specification:
// http://semver.org/
func isSemVer(version string) bool {
	_, err := semver.Make(version)
	return err == nil
}

// ValidateManifest checks for some common issues, before sending the manifest
// to the Wercker steps server.
func ValidateManifest(manifest *StepManifest) error {
	var e []error

	if manifest.Name == "" {
		e = append(e, errors.New("Name cannot be empty"))
	}

	if manifest.Summary == "" {
		e = append(e, errors.New("Summary cannot be empty"))
	}

	if !isSemVer(manifest.Version) {
		e = append(e, errors.New("Version does not appear to be valid semver"))
	}

	return util.SqaushErrors(e)
}
