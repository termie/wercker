//   Copyright © 2016,2018, Oracle and/or its affiliates.  All rights reserved.
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
	"encoding/json"
	"io"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
)

// EmitStatus emits the json message on r
func EmitStatus(e *core.NormalizedEmitter, r io.Reader, options *core.PipelineOptions) error {
	s := NewJSONMessageProcessor()
	dec := json.NewDecoder(r)
	for {
		var m jsonmessage.JSONMessage
		if err := dec.Decode(&m); err == io.EOF {
			// Once the EOF is reached the function will stop
			break
		} else if err != nil {
			util.RootLogger().Panic(err)
		}

		line, err := s.ProcessJSONMessage(&m)
		if err != nil {
			e.Emit(core.Logs, &core.LogsArgs{
				Logs:   err.Error() + "\n",
				Stream: "docker",
			})
			return err
		}

		e.Emit(core.Logs, &core.LogsArgs{
			Logs:   line,
			Stream: "docker",
		})

	}
	return nil
}
