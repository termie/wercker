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
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

type FakeTransport struct {
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	cancelFunc context.CancelFunc

	inchan  chan string
	outchan chan string
}

func (t *FakeTransport) Attach(sessionCtx context.Context, stdin io.Reader, stdout, stderr io.Writer) (context.Context, error) {
	fakeContext, cancel := context.WithCancel(sessionCtx)
	t.cancelFunc = cancel
	t.stdin = stdin
	t.stdout = stdout
	t.stderr = stderr

	t.inchan = make(chan string)
	t.outchan = make(chan string)

	go func() {
		for {
			var p []byte
			p = make([]byte, 1024)
			i, err := t.stdin.Read(p)
			s := string(p[:i])
			util.RootLogger().Println(fmt.Sprintf("(test)  stdin: %q", s))
			t.inchan <- s
			if err != nil {
				close(t.inchan)
				return
			}
		}
	}()

	go func() {
		for {
			s := <-t.outchan
			util.RootLogger().Println(fmt.Sprintf("(test) stdout: %q", s))
			_, err := t.stdout.Write([]byte(s))
			if err != nil {
				close(t.outchan)
				return
			}
		}
	}()

	return fakeContext, nil
}

func (t *FakeTransport) Cancel() {
	t.cancelFunc()
}

func (t *FakeTransport) ListenAndRespond(exit int, recv []string) {
	for {
		s := <-t.inchan
		// If this is the last string send our stuff and echo the status code
		if strings.HasPrefix(s, "echo") && strings.HasSuffix(s, "$?\n") {
			parts := strings.Split(s, " ")
			for _, x := range recv {
				t.outchan <- x
			}
			t.outchan <- fmt.Sprintf("%s %d", parts[1], exit)
			return
		}
	}
}

func fakeSessionOptions() *PipelineOptions {
	return &PipelineOptions{
		GlobalOptions:     &GlobalOptions{Debug: true},
		NoResponseTimeout: 100,
		CommandTimeout:    100,
	}
}

func FakeSession(s *util.TestSuite, opts *PipelineOptions) (context.Context, context.CancelFunc, *Session, *FakeTransport) {
	if opts == nil {
		opts = fakeSessionOptions()
	}
	transport := &FakeTransport{}
	topCtx, cancel := context.WithCancel(context.Background())
	topCtx = NewEmitterContext(topCtx)
	session := NewSession(opts, transport)

	sessionCtx, err := session.Attach(topCtx)
	s.Nil(err)
	return sessionCtx, cancel, session, transport
}

func fakeSentinel(s string) func() string {
	return func() string {
		return s
	}
}

type SessionSuite struct {
	*util.TestSuite
}

func TestSessionSuite(t *testing.T) {
	suiteTester := &SessionSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}

func (s *SessionSuite) TestSend() {
	sessionCtx, _, session, transport := FakeSession(s.TestSuite, nil)

	go func() {
		session.Send(sessionCtx, false, "foo")
	}()

	sess := <-transport.inchan
	s.Equal("foo\n", sess)
}

func (s *SessionSuite) TestSendCancelled() {
	sessionCtx, cancel, session, _ := FakeSession(s.TestSuite, nil)
	cancel()

	errchan := make(chan error)
	go func() {
		errchan <- session.Send(sessionCtx, false, "foo")
	}()

	s.NotNil(<-errchan)
}

func (s *SessionSuite) TestSendChecked() {
	sessionCtx, _, session, transport := FakeSession(s.TestSuite, nil)

	stepper := util.NewStepper()
	go func() {
		transport.ListenAndRespond(0, []string{"foo\n"})
		stepper.Wait()
		transport.ListenAndRespond(1, []string{"bar\n"})
	}()

	// Success
	exit, recv, err := session.SendChecked(sessionCtx, "foo")
	s.Nil(err)
	s.Equal(0, exit)
	s.Equal("foo\n", recv[0])

	stepper.Step()
	// Non-zero Exit
	exit, recv, err = session.SendChecked(sessionCtx, "lala")
	s.NotNil(err)
	s.Equal(1, exit)
	s.Equal("bar\n", recv[0])
}

func (s *SessionSuite) TestSendCheckedCommandTimeout() {
	opts := fakeSessionOptions()
	opts.CommandTimeout = 0
	sessionCtx, _, session, transport := FakeSession(s.TestSuite, opts)

	go func() {
		transport.ListenAndRespond(0, []string{"foo\n"})
	}()

	exit, recv, err := session.SendChecked(sessionCtx, "foo")
	s.NotNil(err)
	// We timed out so -1
	s.Equal(-1, exit)
	// We sent some text so we should have gotten that at least
	s.Equal(1, len(recv))
}

func (s *SessionSuite) TestSendCheckedNoResponseTimeout() {
	opts := fakeSessionOptions()
	opts.NoResponseTimeout = 0
	sessionCtx, _, session, transport := FakeSession(s.TestSuite, opts)

	go func() {
		// Just listen and never send anything
		for {
			<-transport.inchan
		}
	}()

	exit, recv, err := session.SendChecked(sessionCtx, "foo")
	s.NotNil(err)
	s.Equal(-1, exit)
	s.Equal(0, len(recv))
}

func (s *SessionSuite) TestSendCheckedEarlyExit() {
	sessionCtx, _, session, transport := FakeSession(s.TestSuite, nil)

	stepper := util.NewStepper()
	randomSentinel = fakeSentinel("test-sentinel")

	go func() {
		for {
			stepper.Wait()
			<-transport.inchan
		}
	}()

	go func() {
		stepper.Step() // "foo"
		// Wait 5 milliseconds because Send has short delay
		stepper.Step(5) // "echo test-sentinel $?"
		transport.outchan <- "foo"
		transport.Cancel()
		transport.outchan <- "bar"
	}()

	exit, recv, err := session.SendChecked(sessionCtx, "foo")
	s.NotNil(err)
	s.Equal(-1, exit)
	s.Equal(2, len(recv), "should have gotten two lines of output")

}

func (s *SessionSuite) TestSmartSplitLines() {
	sentinel := "FOO9000"
	sentinelLine := "FOO9000 1\n"
	testLine := "some garbage\n"

	// Test easy normal return
	simpleLine := sentinelLine
	simpleLines := smartSplitLines(simpleLine, sentinel)
	s.Equal(1, len(simpleLines))
	s.Equal(sentinelLine, simpleLines[0])
	simpleFound, simpleExit := checkLine(simpleLines[0], sentinel)
	s.Equal(true, simpleFound)
	s.Equal(1, simpleExit)

	// Test return on same logical line as other stuff
	mixedLine := fmt.Sprintf("%s%s", testLine, sentinelLine)
	mixedLines := smartSplitLines(mixedLine, sentinel)
	s.Equal(2, len(mixedLines))
	s.Equal(testLine, mixedLines[0])
	s.Equal(sentinelLine, mixedLines[1])
	mixedFound, mixedExit := checkLine(mixedLines[1], sentinel)
	s.Equal(true, mixedFound)
	s.Equal(1, mixedExit)

	// Test no return
	uselessLine := fmt.Sprintf("%s%s", testLine, testLine)
	uselessLines := smartSplitLines(uselessLine, sentinel)
	s.Equal(1, len(uselessLines))
	s.Equal(uselessLine, uselessLines[0])
	uselessFound, _ := checkLine(uselessLines[0], sentinel)
	s.Equal(false, uselessFound)
}
