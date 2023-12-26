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

type StepManifest struct {
	// name of the step
	Name string `json:"name,omitempty"`
	// version of the step
	Version string `json:"version,omitempty"`
	// summary of the step
	Summary string `json:"summary,omitempty"`
	// tags of the step
	Tags []string `json:"tags,omitempty"`
	// properties of the step
	Properties []*StepProperty `json:"properties,omitempty"`
}

type StepProperty struct {
	// name of the property
	Name string `json:"name,omitempty"`
	// type of the property
	Type string `json:"type,omitempty"`
	// required whether the property is required or not
	Required bool `json:"required,omitempty"`
	// default property
	Default string `json:"default,omitempty"`
}
