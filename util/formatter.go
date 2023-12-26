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
	"strings"
)

const (
	successColor = "\x1b[32m"
	failColor    = "\x1b[31m"
	varColor     = "\x1b[33m"
	reset        = "\x1b[m"
)

// Formatter formats the messages, and optionally disabling colors. See
// FormatMessage for the structure of messages.
type Formatter struct {
	ShowColors bool
}

// Info uses no color.
func (f *Formatter) Info(messages ...string) string {
	return FormatMessage("", f.ShowColors, messages...)
}

// Success uses successColor (green) as color.
func (f *Formatter) Success(messages ...string) string {
	return FormatMessage(successColor, f.ShowColors, messages...)
}

// Fail uses failColor (red) as color.
func (f *Formatter) Fail(messages ...string) string {
	return FormatMessage(failColor, f.ShowColors, messages...)
}

// FormatMessage handles one or two messages. If more messages are used, those
// are ignore. If no messages are used, than it will return an empty string.
// 1 message : --> message[0]
// 2 messages: --> message[0]: message[1]
// color will be applied to the first message, varColor will be used for the
// second message. If useColors is false, than color will be ignored.
func FormatMessage(color string, useColors bool, messages ...string) string {
	segments := []string{}

	l := len(messages)

	if l > 0 {
		segments = append(segments, "-->")
	}

	if l >= 1 {
		if useColors {
			segments = append(segments, fmt.Sprintf(" %s%s%s", color, messages[0], reset))
		} else {
			segments = append(segments, fmt.Sprintf(" %s", messages[0]))
		}
	}

	if l >= 2 {
		if useColors {
			segments = append(segments, fmt.Sprintf(": %s%s%s", varColor, messages[1], reset))
		} else {
			segments = append(segments, fmt.Sprintf(": %s", messages[1]))
		}
	}

	if l > 2 {
		for _, m := range messages[2:] {
			segments = append(segments, fmt.Sprintf(" %s", m))
		}
	}

	return strings.Join(segments, "")
}
