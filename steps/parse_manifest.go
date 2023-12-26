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
	"io"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

// ParseManifest parse b as a StepManifest.
func ParseManifest(b []byte) (*StepManifest, error) {
	var manifest StepManifest
	err := yaml.Unmarshal(b, &manifest)
	if err != nil {
		return nil, err
	}

	return &manifest, nil
}

// ParseManifestReader first reads all of r into memory before using
// ParseManifest to unmarshall the content.
func ParseManifestReader(r io.Reader) (*StepManifest, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return ParseManifest(b)
}
