//   Copyright (c) 2018, Oracle and/or its affiliates.  All rights reserved.
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
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/util"
)

type MainSuite struct {
	*util.TestSuite
}

func TestMainSuite(t *testing.T) {
	suiteTester := &MainSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

//TestGetApp_HelpCommands - tests all commands and subcommnads of wercker cli
//for the --help option
func (s *MainSuite) TestGetApp_HelpCommands() {
	app := GetApp()
	for _, cmd := range app.Commands {
		args := []string{" ", cmd.Name, "--help"}
		err := app.Run(args)
		s.NoError(err, "Error executing wercker %s", cmd.Name)
		for _, subCmd := range cmd.Subcommands {
			args := []string{" ", cmd.Name, subCmd.Name, "--help"}
			err := app.Run(args)
			s.NoError(err, "Error executing wercker %s %s", cmd.Name, subCmd.Name)
		}
	}
}
