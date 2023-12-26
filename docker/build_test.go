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

package dockerlocal

import (
	"testing"

	"github.com/wercker/wercker/util"
)

func TestBuildEnvironment(t *testing.T) {
	env := util.NewEnvironment("X_FOO=bar", "BAZ=fizz")
	passthru := env.GetPassthru().Ordered()
	if len(passthru) != 1 {
		t.Fatal("Expected only one variable in passthru")
	}
	if passthru[0][0] != "FOO" {
		t.Fatal("Expected to find key 'FOO'")
	}
	if passthru[0][1] != "bar" {
		t.Fatal("Expected to find value 'bar'")
	}
}
