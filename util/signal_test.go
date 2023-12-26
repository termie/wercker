//   Copyright © 2019, Oracle and/or its affiliates.  All rights reserved.
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

type SignalSuite struct {
	*TestSuite
}

func TestSignalSuite(t *testing.T) {
	suiteTester := &SignalSuite{&TestSuite{}}
	suite.Run(t, suiteTester)
}

// TestSignalMonkeyAddRemove tests the adding and removing of handlers in LIFO order
func (s *SignalSuite) TestSignalMonkeyAddRemoveLIFO() {

	handler1 := &SignalHandler{
		ID: "ID1",
		F: func() bool {
			return true
		},
	}
	handler2 := &SignalHandler{
		ID: "ID2",
		F: func() bool {
			return true
		},
	}
	handler3 := &SignalHandler{
		ID: "ID3",
		F: func() bool {
			return true
		},
	}

	sm := NewSignalMonkey()

	s.Equal(0, len(sm.handlers))

	sm.Add(handler1)
	s.Equal(1, len(sm.handlers))
	s.Equal(handler1.ID, sm.handlers[0].ID)

	sm.Add(handler2)
	s.Equal(2, len(sm.handlers))
	s.Equal(handler1.ID, sm.handlers[0].ID)
	s.Equal(handler2.ID, sm.handlers[1].ID)

	sm.Add(handler3)
	s.Equal(3, len(sm.handlers))
	s.Equal(handler1.ID, sm.handlers[0].ID)
	s.Equal(handler2.ID, sm.handlers[1].ID)
	s.Equal(handler3.ID, sm.handlers[2].ID)

	sm.Remove(handler3)
	s.Equal(2, len(sm.handlers))
	s.Equal(handler1.ID, sm.handlers[0].ID)
	s.Equal(handler2.ID, sm.handlers[1].ID)

	sm.Remove(handler2)
	s.Equal(1, len(sm.handlers))
	s.Equal(handler1.ID, sm.handlers[0].ID)

	sm.Remove(handler1)
	s.Equal(0, len(sm.handlers))
}

// TestSignalMonkeyAddRemove tests the adding and removing of handlers in non-LIFO order
func (s *SignalSuite) TestSignalMonkeyAddRemoveNonLIFO() {

	handler1 := &SignalHandler{
		ID: "ID1",
		F: func() bool {
			return true
		},
	}
	handler2 := &SignalHandler{
		ID: "ID2",
		F: func() bool {
			return true
		},
	}
	handler3 := &SignalHandler{
		ID: "ID3",
		F: func() bool {
			return true
		},
	}

	sm := NewSignalMonkey()

	s.Equal(0, len(sm.handlers))

	sm.Add(handler1)
	s.Equal(1, len(sm.handlers))
	s.Equal(handler1.ID, sm.handlers[0].ID)

	sm.Add(handler2)
	s.Equal(2, len(sm.handlers))
	s.Equal(handler1.ID, sm.handlers[0].ID)
	s.Equal(handler2.ID, sm.handlers[1].ID)

	sm.Add(handler3)
	s.Equal(3, len(sm.handlers))
	s.Equal(handler1.ID, sm.handlers[0].ID)
	s.Equal(handler2.ID, sm.handlers[1].ID)
	s.Equal(handler3.ID, sm.handlers[2].ID)

	sm.Remove(handler1)
	s.Equal(2, len(sm.handlers))
	s.Equal(handler2.ID, sm.handlers[0].ID)
	s.Equal(handler3.ID, sm.handlers[1].ID)

	sm.Remove(handler2)
	s.Equal(1, len(sm.handlers))
	s.Equal(handler3.ID, sm.handlers[0].ID)

	sm.Remove(handler3)
	s.Equal(0, len(sm.handlers))

}

// TestSignalMonkeyAddRemove tests the adding and removing of handlers with the same ID
func (s *SignalSuite) TestSignalMonkeyAddRemoveDupIDs() {

	handler1 := &SignalHandler{
		ID: "IDSame",
		F: func() bool {
			return true
		},
	}
	handler2 := &SignalHandler{
		ID: "IDSame",
		F: func() bool {
			return true
		},
	}

	sm := NewSignalMonkey()

	s.Equal(0, len(sm.handlers))

	sm.Add(handler1)
	s.Equal(1, len(sm.handlers))
	s.Equal(handler1.ID, sm.handlers[0].ID)

	sm.Add(handler2)
	s.Equal(2, len(sm.handlers))
	s.Equal(handler1.ID, sm.handlers[0].ID)
	s.Equal(handler2.ID, sm.handlers[1].ID)

	sm.Remove(handler1)
	s.Equal(0, len(sm.handlers))
}
