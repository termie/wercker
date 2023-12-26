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

package util

import (
	"fmt"
	"strconv"
	"time"
)

var (
	// GitCommit is the git commit hash associated with this build.
	GitCommit = ""

	// MajorVersion is the semver major version.
	MajorVersion = "1"

	// MinorVersion is the semver minor version.
	MinorVersion = "0"

	// PatchVersion is the semver patch version. (use 0 for dev, build process
	// will inject a build number)
	PatchVersion = "0"

	// Compiled is the unix timestamp when this binary got compiled.
	Compiled = ""
)

func init() {
	if Compiled == "" {
		Compiled = strconv.FormatInt(time.Now().Unix(), 10)
	}
}

// Version returns a semver compatible version for this build.
func Version() string {
	return fmt.Sprintf("%s.%s.%s", MajorVersion, MinorVersion, PatchVersion)
}

// FullVersion returns the semver version and the git version if available.
func FullVersion() string {
	semver := Version()
	gitCommit := ""
	if GitCommit != "" {
		gitCommit = fmt.Sprintf(", Git commit: %s", GitCommit)
	}
	return fmt.Sprintf("%s (Compiled at: %s%s)", semver, CompiledAt().Format(time.RFC3339), gitCommit)
}

// CompiledAt converts the Unix time Compiled to a time.Time using UTC timezone.
func CompiledAt() time.Time {
	i, err := strconv.ParseInt(Compiled, 10, 64)
	if err != nil {
		panic(err)
	}

	return time.Unix(i, 0).UTC()
}

// GetVersions returns a Versions struct filled with the current values.
func GetVersions() *Versions {
	return &Versions{
		CompiledAt: CompiledAt(),
		GitCommit:  GitCommit,
		Version:    Version(),
	}
}

// Versions contains GitCommit and Version as a JSON marshall friendly struct.
type Versions struct {
	CompiledAt time.Time `json:"compiledAt,omitempty"`
	GitCommit  string    `json:"gitCommit,omitempty"`
	Version    string    `json:"version,omitempty"`
}

// FullVersion returns the semver version and the git version if available.
// TODO(mh): I'd like to make the above methods of Versions
// 	Because I'd like to reuse them on `Versions` objects used in updating
func (v *Versions) FullVersion() string {
	gitCommit := ""
	if v.GitCommit != "" {
		gitCommit = fmt.Sprintf(", Git commit: %s", v.GitCommit)
	}
	return fmt.Sprintf("%s (Compiled at: %s%s)", v.Version, v.CompiledAt.Format(time.RFC3339), gitCommit)
}
