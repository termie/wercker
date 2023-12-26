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
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ParseManifest_Valid(t *testing.T) {
	tests := []string{
		"valid.yml",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			f, err := os.Open(path.Join("testdata", "step_manifests", test))
			require.NoError(t, err, "Unable to open step manifest")

			manifest, err := ParseManifestReader(f)
			assert.NoError(t, err)
			assert.NotNil(t, manifest)
		})
	}
}

func Test_ParseManifest_Invalid(t *testing.T) {
	tests := []string{
		"invalid.yml",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			f, err := os.Open(path.Join("testdata", "step_manifests", test))
			require.NoError(t, err, "Unable to open step manifest")

			manifest, err := ParseManifestReader(f)
			assert.Error(t, err)
			assert.Nil(t, manifest)
		})
	}
}
