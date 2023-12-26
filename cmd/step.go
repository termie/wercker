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
	"github.com/wercker/wercker/core"
	stepscmd "github.com/wercker/wercker/steps/cmd"
)

func cmdStepPublish(opts *core.WerckerStepOptions) error {
	stepDir := "."
	if opts.StepDir != "" {
		stepDir = opts.StepDir
	}

	publishOpts := &stepscmd.PublishStepOptions{
		Endpoint:  opts.StepRegistryURL,
		AuthToken: opts.AuthToken,
		Owner:     opts.Owner,
		StepDir:   stepDir,
		TempDir:   "",
		Private:   opts.Private,
	}
	return stepscmd.PublishStep(publishOpts)
}
