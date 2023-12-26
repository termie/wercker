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

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// BoxOptions are box options, duh
type BoxOptions struct {
	NetworkDisabled bool
}

type Box interface {
	GetName() string
	GetTag() string
	Repository() string
	Clean() error
	Stop()
	Commit(string, string, string, bool) (*docker.Image, error)
	Restart() (*docker.Container, error)
	AddService(ServiceBox)
	Fetch(context.Context, *util.Environment) (*docker.Image, error)
	Run(context.Context, *util.Environment, string) (*docker.Container, error)
	RecoverInteractive(string, Pipeline, Step) error
}
