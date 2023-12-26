//   Copyright © 2016, 2019, Oracle and/or its affiliates.  All rights reserved.
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

package util

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var (
	protectedPrefix = "XXX_"
	publicPrefix    = "X_"
)

// Environment represents a shell environment and is implemented as something
// like an OrderedMap
type Environment struct {
	Hidden *Environment
	Map    map[string]string
	Order  []string
}

// DefaultEnvironment uses the default strategy of using the current host's
// environment, loading the environment variables defined in the file located at
// envfile (if it doesn't exist, it will be silently ignored). Finally any proxy
// environment variables that are defined on the host, will be passed through to
// the build (by prefixing these with the publicPrefix).
func DefaultEnvironment(envfile string) *Environment {
	env := NewEnvironment(os.Environ()...)
	env.LoadFile(envfile)
	env.PassThruProxyConfig()

	return env
}

// NewEnvironment fills up an Environment from a []string
// Usually called like: env := NewEnvironment(os.Environ())
func NewEnvironment(env ...string) *Environment {
	e := Environment{
		Hidden: &Environment{},
	}
	for _, keyvalue := range env {
		pair := strings.SplitN(keyvalue, "=", 2)
		e.Add(pair[0], pair[1])
	}

	return &e
}

// Update adds new elements to the Environment data structure.
func (e *Environment) Update(a [][]string) {
	for _, keyvalue := range a {
		e.Add(keyvalue[0], keyvalue[1])
	}
}

// Add an individual record.
func (e *Environment) Add(key, value string) {
	if e.Map == nil {
		e.Map = make(map[string]string)
	}
	if _, ok := e.Map[key]; !ok {
		e.Order = append(e.Order, key)
	}
	e.Map[key] = value
}

// AddIfMissing an individual record.
func (e *Environment) AddIfMissing(key, value string) {
	if e.Map == nil {
		e.Add(key, value)
	} else if _, ok := e.Map[key]; !ok {
		e.Add(key, value)
	}
}

// PassThruProxyConfig prefixes any proxy environment variables defined on the
// host with publicPrefix. So they will be available during a build.
func (e *Environment) PassThruProxyConfig() {
	if e.Map == nil {
		return
	}

	for _, key := range proxyEnv {
		value, ok := e.Map[key]
		if ok {
			e.AddIfMissing(fmt.Sprintf("%s%s", publicPrefix, key), value)
		}
	}
}

// Get an individual record.
func (e *Environment) Get(key string) string {
	if e.Map != nil {
		if val, ok := e.Map[key]; ok {
			return val
		}
	}
	return ""
}

// Export the environment as shell commands for use with Session.Send*
func (e *Environment) Export() []string {
	s := []string{}
	for _, key := range e.Order {
		export := fmt.Sprintf(`export %s=%q`, key, e.Map[key])
		export = strings.Replace(export, "`", "\\`", -1)
		s = append(s, export)
	}
	return s
}

// Ordered returns a [][]string of the items in the env.
// Used only for debugging
func (e *Environment) Ordered() [][]string {
	a := [][]string{}
	for _, k := range e.Order {
		a = append(a, []string{k, e.Map[k]})
	}
	return a
}

// Interpolate is a naive interpolator that attempts to replace variables
// identified by $VAR with the value of the VAR pipeline environment variable
// NOTE(termie): This will check the hidden env, too.
func (e *Environment) Interpolate(s string) string {
	return os.Expand(s, e.GetInclHidden)
}

var mirroredEnv = [...]string{
	"WERCKER_STARTED_BY",
	"WERCKER_MAIN_PIPELINE_STARTED",
}

var proxyEnv = [...]string{
	"http_proxy",
	"https_proxy",
	"no_proxy",
	"HTTP_PROXY",
	"HTTPS_PROXY",
	"NO_PROXY",
}

// GetPassthru gets the environment variables from e which should be exposed to
// the build as normal environment variables.
func (e *Environment) GetPassthru() (env *Environment) {
	return e.passthru(publicPrefix)
}

// GetHiddenPassthru gets the environment variables from e which should be
// exposed to the build as hidden environment variables.
func (e *Environment) GetHiddenPassthru() (env *Environment) {
	return e.passthru(protectedPrefix)
}

func (e *Environment) passthru(prefix string) (env *Environment) {
	a := [][]string{}
	for _, key := range e.Order {
		if strings.HasPrefix(key, prefix) {
			a = append(a, []string{strings.TrimPrefix(key, prefix), e.Map[key]})
		}
	}
	env = &Environment{}
	env.Update(a)
	return env

}

// GetMirror retrieves all the environment variables.
func (e *Environment) GetMirror() [][]string {
	a := [][]string{}
	for _, key := range mirroredEnv {
		value, ok := e.Map[key]
		if ok {
			a = append(a, []string{key, value})
		}
	}
	return a
}

// GetInclHidden gets an individual record either from this environment or the
// hidden environment.
func (e *Environment) GetInclHidden(key string) string {
	if e.Map != nil {
		if val, ok := e.Map[key]; ok {
			return val
		}
	}

	if e.Hidden != nil && e.Hidden.Map != nil {
		if val, ok := e.Hidden.Map[key]; ok {
			return val
		}
	}

	return ""
}

// LoadFile imports key,val pairs from the provided file path. File entries
// should be 1 per line in the form key=value. Blank lines and lines begining
// with # are ignored.
func (e *Environment) LoadFile(f string) error {
	file, err := os.Open(f)
	if err != nil {
		return err
	}
	defer file.Close()

	s := bufio.NewScanner(file)
	for ok := s.Scan(); ok; ok = s.Scan() {
		// Ignore comments
		if strings.HasPrefix(s.Text(), "#") {
			continue
		}
		parts := strings.SplitN(s.Text(), "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		// Don't override existing environment
		if e.Get(key) != "" {
			continue
		}

		val = trim(val)

		e.Add(key, val)
	}

	return nil
}

func trim(s string) string {
	s = strings.TrimSpace(s)

	if len(s) > 1 {
		f := string(s[0:1])
		l := string(s[len(s)-1:])
		if f == l && strings.ContainsAny(f, `"'`) {
			// strip surrounding quotes
			s = string(s[1 : len(s)-1])

			// now expand escaped double quotes and newlines
			s = strings.Replace(s, `\"`, `"`, -1)
			s = strings.Replace(s, `\n`, "\n", -1)
		}
	}

	return s
}
