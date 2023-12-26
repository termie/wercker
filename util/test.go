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
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
)

// TestSuite is our base class for test suites
type TestSuite struct {
	suite.Suite
	workingDir string
}

// SetupTest mostly just configures logging now
func (s *TestSuite) SetupTest() {
	setupTestLogging(s.T())
}

// TearDownTest cleans up our working dir if we made one
func (s *TestSuite) TearDownTest() {
	if s.workingDir != "" {
		err := os.RemoveAll(s.WorkingDir())
		s.workingDir = ""
		if err != nil {
			s.T().Error(err.Error())
		}
	}
}

// WorkingDir makes a new temp dir to run tests in
func (s *TestSuite) WorkingDir() string {
	if s.workingDir == "" {
		s.workingDir, _ = ioutil.TempDir("", "wercker-")
	}
	return s.workingDir
}

// func (s *TestSuite) Error(err error) {
//   s.T().Error(err)
//   // s.FailNow()
// }

// FailNow just proxies to testing.T.FailNow
func (s *TestSuite) FailNow() {
	s.T().FailNow()
}

// Skip just proxies to testing.T.Skip
func (s *TestSuite) Skip(msg string) {
	s.T().Skip(msg)
}

// TestLogWriter writes our logs to the test output
type TestLogWriter struct {
	t *testing.T
}

// NewTestLogWriter constructor
func NewTestLogWriter(t *testing.T) *TestLogWriter {
	return &TestLogWriter{t: t}
}

// Write for io.Writer
func (l *TestLogWriter) Write(p []byte) (int, error) {
	l.t.Log(string(p))
	return len(p), nil
}

// TestLogFormatter removes the last newline character
type TestLogFormatter struct {
	*logrus.TextFormatter
}

// NewTestLogFormatter constructor
func NewTestLogFormatter() *TestLogFormatter {
	return &TestLogFormatter{&logrus.TextFormatter{}}
}

// Format like a text log but strip the last newline
func (f *TestLogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b, err := f.TextFormatter.Format(entry)
	if err == nil {
		b = b[:len(b)-1]
	}
	return b, err
}

func setupTestLogging(t *testing.T) {
	writer := NewTestLogWriter(t)
	rootLogger.SetLevel("debug")
	rootLogger.Out = writer
	rootLogger.Formatter = NewTestLogFormatter()
}

// Stepper lets use step and sync goroutines
type Stepper struct {
	stepper chan struct{}
}

// NewStepper constructor
func NewStepper() *Stepper {
	return &Stepper{stepper: make(chan struct{})}
}

// Wait until Step has been called
func (s *Stepper) Wait() {
	s.stepper <- struct{}{}
	<-s.stepper
}

// Step through a waiting goroutine with optional delay
func (s *Stepper) Step(delay ...int) {
	<-s.stepper
	for _, d := range delay {
		time.Sleep(time.Duration(d) * time.Millisecond)
	}
	s.stepper <- struct{}{}
}
