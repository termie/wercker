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
	"testing"

	"github.com/stretchr/testify/suite"
)

type EnvironmentSuite struct {
	TestSuite
}

func (s *EnvironmentSuite) SetupTest() {
	s.TestSuite.SetupTest()
}

func TestEnvironmentSuite(t *testing.T) {
	suiteTester := new(EnvironmentSuite)
	suite.Run(t, suiteTester)
}

func (s *EnvironmentSuite) TestAddIfMissing() {
	env := NewEnvironment()
	env.AddIfMissing("foo", "bar")

	s.Equal(0, len(env.GetPassthru().Ordered()))
	s.Equal(0, len(env.GetHiddenPassthru().Ordered()))
	s.Equal(1, len(env.Ordered()))
	s.Equal("bar", env.Get("foo"))
}

func (s *EnvironmentSuite) TestAddIfMissingDoesNotClobber() {
	env := NewEnvironment("existing=original")
	env.AddIfMissing("existing", "overridden")

	s.Equal(0, len(env.GetPassthru().Ordered()))
	s.Equal(0, len(env.GetHiddenPassthru().Ordered()))
	s.Equal(1, len(env.Ordered()))
	s.Equal("original", env.Get("existing"))
}

func (s *EnvironmentSuite) TestPassThruProxyConfig() {
	env := NewEnvironment("http_proxy=proxy", "https_proxy=proxy", "no_proxy=noproxy", "HTTP_PROXY=proxy", "HTTPS_PROXY=proxy", "NO_PROXY=noproxy")
	env.PassThruProxyConfig()

	s.Equal(6, len(env.GetPassthru().Ordered()))
	s.Equal(0, len(env.GetHiddenPassthru().Ordered()))
	s.Equal(12, len(env.Ordered()))
	s.Equal("proxy", env.Get("http_proxy"))
	s.Equal("proxy", env.Get("https_proxy"))
	s.Equal("noproxy", env.Get("no_proxy"))
	s.Equal("proxy", env.Get("HTTP_PROXY"))
	s.Equal("proxy", env.Get("HTTPS_PROXY"))
	s.Equal("noproxy", env.Get("NO_PROXY"))
}

func (s *EnvironmentSuite) TestPassThruProxyConfigDoesNotClobber() {
	env := NewEnvironment("http_proxy=proxy", "https_proxy=proxy", "no_proxy=noproxy", "HTTP_PROXY=proxy", "HTTPS_PROXY=proxy", "NO_PROXY=noproxy",
		"X_http_proxy=xproxy", "X_https_proxy=xproxy", "X_no_proxy=xnoproxy", "X_HTTP_PROXY=xproxy", "X_HTTPS_PROXY=xproxy", "X_NO_PROXY=xnoproxy")
	env.PassThruProxyConfig()

	s.Equal(6, len(env.GetPassthru().Ordered()))
	s.Equal(0, len(env.GetHiddenPassthru().Ordered()))
	s.Equal(12, len(env.Ordered()))

	s.Equal("proxy", env.Get("http_proxy"))
	s.Equal("proxy", env.Get("https_proxy"))
	s.Equal("noproxy", env.Get("no_proxy"))
	s.Equal("proxy", env.Get("HTTP_PROXY"))
	s.Equal("proxy", env.Get("HTTPS_PROXY"))
	s.Equal("noproxy", env.Get("NO_PROXY"))

	s.Equal("xproxy", env.Get("X_http_proxy"))
	s.Equal("xproxy", env.Get("X_https_proxy"))
	s.Equal("xnoproxy", env.Get("X_no_proxy"))
	s.Equal("xproxy", env.Get("X_HTTP_PROXY"))
	s.Equal("xproxy", env.Get("X_HTTPS_PROXY"))
	s.Equal("xnoproxy", env.Get("X_NO_PROXY"))
}

func (s *EnvironmentSuite) TestPassthru() {
	env := NewEnvironment("X_PUBLIC=foo", "XXX_PRIVATE=bar", "NOT=included")
	s.Equal(1, len(env.GetPassthru().Ordered()))
	s.Equal(1, len(env.GetHiddenPassthru().Ordered()))
}

func (s *EnvironmentSuite) TestPassthruKeepsOrder() {
	env := NewEnvironment("X_fake1=val1", "X_fake2=val2", "X_fake3=$fake2")
	actual := env.GetPassthru()
	expected := []string{"fake1", "fake2", "fake3"}
	s.Equal(expected, actual.Order)
}
func (s *EnvironmentSuite) TestPassthruHiddenKeepsOrder() {
	env := NewEnvironment("XXX_fake1=val1", "XXX_fake2=val2", "XXX_fake3=$fake2")
	actual := env.GetHiddenPassthru()
	expected := []string{"fake1", "fake2", "fake3"}
	s.Equal(expected, actual.Order)
}

func (s *EnvironmentSuite) TestInterpolate() {
	env := NewEnvironment("PUBLIC=foo", "X_PRIVATE=zed", "XXX_OTHER=otter")
	env.Update(env.GetPassthru().Ordered())
	env.Hidden.Update(env.GetHiddenPassthru().Ordered())

	// this is impossible to set because the order the variables are applied is
	// defined by the caller
	//env.Update([][]string{[]string{"X_PUBLIC", "bar"}})
	//tt.Equal(env.Interpolate("$PUBLIC"), "foo", "Non-prefixed should alias any X_ prefixed vars.")
	s.Equal(env.Interpolate("${PUBLIC}"), "foo", "Alternate shell style vars should work.")

	// NB: stipping only works because we cann Update with the passthru
	// function above
	s.Equal(env.Interpolate("$PRIVATE"), "zed", "Xs should be stripped.")
	s.Equal(env.Interpolate("$OTHER"), "otter", "XXXs should be stripped.")
	s.Equal(env.Interpolate("one two $PUBLIC bar"), "one two foo bar", "interpolation should work in middle of string.")
}

func (s *EnvironmentSuite) TestOrdered() {
	env := NewEnvironment("PUBLIC=foo", "X_PRIVATE=zed")
	expected := [][]string{[]string{"PUBLIC", "foo"}, []string{"X_PRIVATE", "zed"}}
	s.Equal(env.Ordered(), expected)
}

func (s *EnvironmentSuite) TestExport() {
	env := NewEnvironment("PUBLIC=foo", "X_PRIVATE=zed")
	expected := []string{`export PUBLIC="foo"`, `export X_PRIVATE="zed"`}
	s.Equal(expected, env.Export())
}

func (s *EnvironmentSuite) TestExportBacktick() {
	env := NewEnvironment("BT1=`backtick", "BT2=back`tick", "BT3=backtick`")
	expected := []string{`export BT1="\` + "`" + `backtick"`, `export BT2="back\` + "`" + `tick"`, `export BT3="backtick\` + "`" + `"`}
	s.Equal(expected, env.Export())
}

func (s *EnvironmentSuite) TestExportBackslash() {
	env := NewEnvironment(`BS1=\backslash`, `BS2=back\slash`, `BS3=backslash\`)
	expected := []string{`export BS1="\\backslash"`, `export BS2="back\\slash"`, `export BS3="backslash\\"`}
	s.Equal(expected, env.Export())
}

func (s *EnvironmentSuite) TestLoadFile() {
	env := NewEnvironment("PUBLIC=foo")
	expected := [][]string{
		[]string{"PUBLIC", "foo"},
		[]string{"A", "1"},
		[]string{"B", "2"},
		[]string{"C", "3"},
		[]string{"D", "4"},
		[]string{"E", "5"},
		[]string{"F", "6"},
		[]string{"G", "7"},
		[]string{"H", "8"},
		[]string{"I", "9"},
		[]string{"J", "Hello \"quotes\""},
		[]string{"K", ""},
		[]string{"L", "\n"},
		[]string{"M", `\"`},
		[]string{"N", `"`},
	}
	env.LoadFile("../tests/environment-test-load-file.env")
	s.Equal(15, len(env.Ordered()), "Should only load 8 valid lines.")
	s.Equal("foo", env.Get("PUBLIC"), "LoadFile should ignore keys already set in env.")
	s.Equal(expected, env.Ordered(), "LoadFile should maintain order.")
	s.Equal([]string{"PUBLIC", "A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"}, env.Order, "LoadFile should maintain ordered keys.")
}
