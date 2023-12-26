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

import "time"

// Debouncer silences repeated triggers for settlePeriod
// and sends the current time on first trigger to C
// C is the public read only channel, c is the private r/w chan
type Debouncer struct {
	C            <-chan time.Time
	c            chan time.Time
	settlePeriod time.Duration
	settling     bool
}

// NewDebouncer constructor
func NewDebouncer(d time.Duration) *Debouncer {
	c := make(chan time.Time, 1)
	return &Debouncer{
		C:            c,
		c:            c,
		settlePeriod: d,
		settling:     false,
	}
}

// Trigger tells us we should do the thing we're waiting on
func (d *Debouncer) Trigger() {
	if d.settling {
		return
	}
	d.settling = true
	time.AfterFunc(d.settlePeriod, func() {
		d.settling = false
	})
	// Non-blocking send of time on c.
	select {
	case d.c <- time.Now():
	default:
	}
}
