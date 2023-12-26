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
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/pborman/uuid"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// Receiver is for reading from our session
type Receiver struct {
	queue chan string
}

// NewReceiver returns a new channel-based io.Writer
func NewReceiver(queue chan string) *Receiver {
	return &Receiver{queue: queue}
}

// Write writes to a channel
func (r *Receiver) Write(p []byte) (int, error) {
	buf := bytes.NewBuffer(p)
	r.queue <- buf.String()
	return buf.Len(), nil
}

// Sender is for sending to our session
type Sender struct {
	queue chan string
}

// NewSender gives us a new channel-based io.Reader
func NewSender(queue chan string) *Sender {
	return &Sender{queue: queue}
}

// Read reads from a channel
func (s *Sender) Read(p []byte) (int, error) {
	send := <-s.queue
	i := copy(p, []byte(send))
	return i, nil
}

// Transport interface for talking to containervisors
type Transport interface {
	Attach(context.Context, io.Reader, io.Writer, io.Writer) (context.Context, error)
}

// Session is our way to interact with the docker container
type Session struct {
	options    *PipelineOptions
	transport  Transport
	logsHidden bool
	send       chan string
	recv       chan string
	exit       chan int
	logger     *util.LogEntry
}

// NewSession returns a new interactive session to a container.
func NewSession(options *PipelineOptions, transport Transport) *Session {
	logger := util.RootLogger().WithField("Logger", "Session")
	return &Session{
		options:    options,
		transport:  transport,
		logsHidden: false,
		logger:     logger,
	}
}

func (s *Session) Transport() interface{} {
	return s.transport
}

func (s *Session) Recv() chan string {
	return s.recv
}

// Attach us to our container and set up read and write queues.
// Returns a context object for the transport so we can propagate cancels
// on errors and closed connections.
func (s *Session) Attach(runnerCtx context.Context) (context.Context, error) {
	recv := make(chan string)
	outputStream := NewReceiver(recv)
	s.recv = recv

	send := make(chan string)
	inputStream := NewSender(send)
	s.send = send

	// We treat the transport context as the session context everywhere
	return s.transport.Attach(runnerCtx, inputStream, outputStream, outputStream)
}

// HideLogs will emit Logs with args.Hidden set to true
func (s *Session) HideLogs() {
	s.logsHidden = true
}

// ShowLogs will emit Logs with args.Hidden set to false
func (s *Session) ShowLogs() {
	s.logsHidden = false
}

// Send an array of commands.
func (s *Session) Send(sessionCtx context.Context, forceHidden bool, commands ...string) error {
	e, err := EmitterFromContext(sessionCtx)
	if err != nil {
		return err
	}
	// Do a quick initial check whether we have a valid session first
	select {
	case <-sessionCtx.Done():
		s.logger.Errorln("Session finished before sending commands:", commands)
		return sessionCtx.Err()
	// Wait because if both cases are available golang will pick one randomly
	case <-time.After(1 * time.Millisecond):
		// Pass
	}

	for i := range commands {
		command := commands[i] + "\n"
		select {
		case <-sessionCtx.Done():
			s.logger.Errorln("Session finished before sending command:", command)
			return sessionCtx.Err()
		case s.send <- command:
			hidden := s.logsHidden
			if forceHidden {
				hidden = forceHidden
			}

			e.Emit(Logs, &LogsArgs{
				Hidden: hidden,
				Stream: "stdin",
				Logs:   command,
			})
		}
	}
	return nil
}

var randomSentinel = func() string {
	return uuid.NewRandom().String()
}

// CommandResult exists so that we can make a channel of them
type CommandResult struct {
	exit int
	recv []string
	err  error
}

func checkLine(line, sentinel string) (bool, int) {
	if !strings.HasPrefix(line, sentinel) {
		return false, -999
	}
	var rand string
	var exit int
	_, err := fmt.Sscanf(line, "%s %d\n", &rand, &exit)
	if err != nil {
		return false, -999
	}
	return true, exit
}

// smartSplitLines tries really hard to make sure our sentinel string
// ends up on its own line
func smartSplitLines(line, sentinel string) []string {
	// NOTE(termie): we have to do some string mangling here to find the
	//               sentinel when stuff manages to squeeze it on to the
	//               same logical output line, it isn't pretty and makes
	//               me sad
	lines := []string{}
	splitLines := strings.Split(line, "\n")
	// If the line at least ends with a newline
	if len(splitLines) > 1 {
		// Check the second to last element
		// (the newline at the end makes an empty final element)
		possibleSentinel := splitLines[len(splitLines)-2]
		// And we expect a newline at the end
		possibleSentinel = fmt.Sprintf("%s\n", possibleSentinel)

		// does this string contain the sentinel?
		sentPos := strings.Index(possibleSentinel, sentinel)

		// If we found the sentinel, make sure it gets read as a separate line to anything that preceded it
		if sentPos >= 0 {
			// If we weren't the only line to begin with, add the rest
			if len(splitLines) > 2 {
				otherLines := strings.Join(splitLines[:len(splitLines)-2], "\n")
				otherLines = fmt.Sprintf("%s\n", otherLines)
				lines = append(lines, otherLines)
			}
			if sentPos > 0 {
				// Add the characters before the sentinel on its own line
				lines = append(lines, possibleSentinel[0:sentPos])
			}
			// add the sentinel (and whatever follows) on its own line
			lines = append(lines, possibleSentinel[sentPos:])
		} else {
			// Otherwise a sentinel was not found so just return the whole thing
			lines = append(lines, line)
		}
	} else {
		lines = append(lines, line)
	}
	return lines
}

// SendChecked sends commands, waits for them to complete and returns the
// exit status and output
// Ways to know a command is done:
//  [x] We received the sentinel echo
//  [x] The container has exited and we've exhausted the incoming data
//  [x] The session has closed and we've exhaused the incoming data
//  [x] The command has timed out
// Ways for a command to be successful:
//  [x] We received the sentinel echo with exit code 0
func (s *Session) SendChecked(sessionCtx context.Context, commands ...string) (int, []string, error) {
	e, err := EmitterFromContext(sessionCtx)
	if err != nil {
		return -1, []string{}, err
	}
	recv := []string{}
	sentinel := randomSentinel()

	sendCtx, _ := context.WithTimeout(sessionCtx, time.Duration(s.options.CommandTimeout)*time.Millisecond)

	commandComplete := make(chan CommandResult)

	// Signal channel to tell the reader to stop reading, this lets us
	// keep it reading for a small amount of time after we know something
	// has gone wrong, otherwise it misses some error messages.
	stopReading := make(chan struct{}, 1)

	// This is our main waiter, it will get an exit code, an error or a timeout
	// and then complete the command, anything
	exitChan := make(chan int)
	errChan := make(chan error)
	go func() {
		select {
		// We got an exit code because we got our sentinel, let's skiddaddle
		case exit := <-exitChan:
			err = nil
			if exit != 0 {
				err = fmt.Errorf("Command exited with exit code: %d", exit)
			}
			commandComplete <- CommandResult{exit: exit, recv: recv, err: err}
		case err = <-errChan:
			commandComplete <- CommandResult{exit: -1, recv: recv, err: err}
		case <-sendCtx.Done():
			// We timed out or something closed, try to read in the rest of the data
			// over the next 100 milliseconds and then return
			<-time.After(time.Duration(100) * time.Millisecond)
			// close(stopReading)
			stopReading <- struct{}{}
			commandComplete <- CommandResult{exit: -1, recv: recv, err: sendCtx.Err()}
		}
	}()

	// If we don't get a response in a certain amount of time, timeout
	noResponseTimeout := make(chan struct{})
	go func() {
		for {
			select {
			case <-noResponseTimeout:
				continue
			case <-time.After(time.Duration(s.options.NoResponseTimeout) * time.Millisecond):
				stopReading <- struct{}{}
				errChan <- fmt.Errorf("Command timed out after no response")
				return
			}
		}
	}()

	// Read in data until we get our sentinel or are asked to stop
	go func() {
		for {
			select {
			case line := <-s.recv:
				// If we found a line reset the NoResponseTimeout timer
				noResponseTimeout <- struct{}{}
				lines := smartSplitLines(line, sentinel)
				for _, subline := range lines {
					// subline = fmt.Sprintf("%s\n", subline)
					// If we found the exit code, we're done
					foundExit, exit := checkLine(subline, sentinel)
					if foundExit {
						e.Emit(Logs, &LogsArgs{
							Hidden: true,
							Logs:   subline,
						})
						exitChan <- exit
						return
					}
					e.Emit(Logs, &LogsArgs{
						Hidden: s.logsHidden,
						Logs:   subline,
					})
					recv = append(recv, subline)
				}
			case <-stopReading:
				return
			}
		}
	}()

	err = s.Send(sessionCtx, false, commands...)
	if err != nil {
		return -1, []string{}, err
	}
	err = s.Send(sessionCtx, true, fmt.Sprintf("echo %s $?", sentinel))
	if err != nil {
		return -1, []string{}, err
	}

	r := <-commandComplete
	// Pretty up the error messages
	if r.err == context.DeadlineExceeded {
		r.err = fmt.Errorf("Command timed out")
	} else if r.err == context.Canceled {
		r.err = fmt.Errorf("Command cancelled due to error")
	}
	return r.exit, r.recv, r.err
}
