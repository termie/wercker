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
	"os"
	"os/signal"
	"sync"
)

// SignalHandler is a little struct to hold our signal handling functions
// and an identifier so we can remove it from the list.
type SignalHandler struct {
	ID string
	F  func() bool
}

// SignalMonkey is a LIFO, cascading, singleton for dispatching signal handlers
type SignalMonkey struct {
	signal   os.Signal
	handlers []*SignalHandler
	notify   chan os.Signal
	mutex    *sync.Mutex
}

// NewSignalMonkey constructor
func NewSignalMonkey() *SignalMonkey {
	return &SignalMonkey{handlers: []*SignalHandler{}, mutex: &sync.Mutex{}}
}

// Add a handler to our array
func (s *SignalMonkey) Add(fn *SignalHandler) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.handlers = append(s.handlers, fn)
}

// Remove a handler from our array
func (s *SignalMonkey) Remove(fn *SignalHandler) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// iterate over array, being careful that we may be removing elements as we do so
	i := 0
	for len(s.handlers) > i {
		x := s.handlers[i]
		if x.ID == fn.ID {
			// Delete preserving order from here:
			// https://code.google.com/p/go-wiki/wiki/SliceTricks
			copy(s.handlers[i:], s.handlers[i+1:])
			s.handlers[len(s.handlers)-1] = nil
			s.handlers = s.handlers[:len(s.handlers)-1]
		} else {
			i++
		}
	}
}

// Dispatch calls the handlers LIFO, removing them from the list as it does
// if any returns false, it stops processing further handlers.
func (s *SignalMonkey) Dispatch() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for len(s.handlers) > 0 {
		// Pop from:
		// https://code.google.com/p/go-wiki/wiki/SliceTricks
		fn, a := s.handlers[len(s.handlers)-1], s.handlers[:len(s.handlers)-1]
		s.handlers = a

		result := fn.F()
		if result == false {
			break
		}
	}
}

// Register ourselves to get notifications on a signal
func (s *SignalMonkey) Register(sig os.Signal) {
	s.notify = make(chan os.Signal, 1)
	s.signal = sig
	signal.Notify(s.notify, sig)

	// Start listening on the signal channel forever
	go func() {
		// If we receive another signal before we finish processing the first one
		// assume that the user really really wants to quit and just barf.
		tries := 0
		for _ = range s.notify {
			go func() {
				if tries == 0 && len(s.handlers) > 0 {
					tries++
					s.Dispatch()
					tries--
				} else {
					RootLogger().Fatal("Exiting forcefully, containers and data may not have been cleaned up")
				}
			}()
		}
	}()
}

var globalSigint = NewSignalMonkey()
var globalSigterm = NewSignalMonkey()

// GlobalSigint returns the sigint registry
func GlobalSigint() *SignalMonkey {
	return globalSigint
}

// GlobalSigterm returns the sigterm registry
func GlobalSigterm() *SignalMonkey {
	return globalSigterm
}
