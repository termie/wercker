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

type Step struct {
	// owner is the user owning the step
	Owner string `json:"owner,omitempty"`
	// name of the step
	Name string `json:"name,omitempty"`
	// version of the step
	Version *StepVersion `json:"version,omitempty"`
	// summary of the step. Should never exceed 140 characters
	Summary string `json:"summary,omitempty"`
	// tags of the step
	Tags []string `json:"tags,omitempty"`
	// checksum of the tarball containing the step
	Checksum string `json:"checksum,omitempty"`
	// size of the tarball containing the step
	Size int64 `json:"size,omitempty"`
	// license of the step
	License string `json:"license,omitempty"`
	// properties of the step
	Properties []*StepProperty `json:"properties,omitempty"`
	// repository of the step. URL where to find the source of the step
	Repository string `json:"repository,omitempty"`
	// tarballUrl points to the step download link
	TarballURL string `json:"tarballUrl,omitempty"`
	// readmeUrl point to the step README
	ReadmeURL string `json:"readmeUrl,omitempty"`
}

type StepVersion struct {
	// number is the semantic version representation of the step
	Number string `json:"number,omitempty"`
	// published is the ISO8601 string representation of the publish date
	Published string `json:"published,omitempty"`
}
