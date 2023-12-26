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

package core

import "fmt"

// Store is generic store interface
type Store interface {
	// StoreFromFile copies a file from local disk to the store
	StoreFromFile(*StoreFromFileArgs) error
}

// StoreFromFileArgs are the args for storing a file
type StoreFromFileArgs struct {
	// Path to the local file.
	Path string

	// Key of the file as stored in the store.
	Key string

	// ContentType hints to the content-type of the file (might be ignored)
	ContentType string

	// Meta data associated with the upload (might be ignored)
	Meta map[string]*string

	// MaxTries is the maximum that a store should retry should the store fail.
	MaxTries int
}

// GenerateBaseKey generates the base key based on ApplicationID and either
// DeployID or BuilID
func GenerateBaseKey(options *PipelineOptions) string {
	key := fmt.Sprintf("project-artifacts/%s/%s", options.ApplicationID, options.RunID)

	return key
}
