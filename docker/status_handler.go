//   Copyright © 2016,2018, Oracle and/or its affiliates.  All rights reserved.
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

package dockerlocal

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/jsonmessage"
)

// NewJSONMessageProcessor will create a new JSONMessageProcessor and
// initialize it.
func NewJSONMessageProcessor() *JSONMessageProcessor {
	s := &JSONMessageProcessor{}
	s.progressMessages = make(map[string]*jsonmessage.JSONMessage)
	return s
}

// A JSONMessageProcessor will process JSONMessages and generate logs.
type JSONMessageProcessor struct {
	lastProgressLength int
	message            *jsonmessage.JSONMessage
	progressMessages   map[string]*jsonmessage.JSONMessage
}

// ProcessJSONMessage will take JSONMessage m and generate logs based on the
// message and previous messages.
func (s *JSONMessageProcessor) ProcessJSONMessage(m *jsonmessage.JSONMessage) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}

	if m.Stream != "" {
		return m.Stream, nil
	}

	switch m.Status {
	case "Extracting":
		fallthrough
	case "Pushing":
		fallthrough
	case "Downloading":
		fallthrough
	case "Buffering to disk":
		s.progressMessages[m.ID] = m

	case "Pull complete":
		fallthrough
	case "Download complete":
		fallthrough
	case "Image already pushed, skipping":
		fallthrough
	case "Image successfully pushed":
		delete(s.progressMessages, m.ID)
		s.message = m
	default:
		s.message = m
	}

	return s.getOutput(), nil
}

// generateFilling will generate spaces based on s.lastProgressLength and
// length. This is to overwrite previous written lines that are bigger than the
// current line.
func (s *JSONMessageProcessor) generateFilling(length int) string {
	filling := ""
	if s.lastProgressLength > 0 {
		if length < s.lastProgressLength {
			filling = strings.Repeat(" ", s.lastProgressLength-length)
		}

		// We've generated filling so reset the lastProgressLength
		s.lastProgressLength = 0
	}
	return filling
}

// getOutput will take the current s.message and s.progressMessages and generate
// a line. This will remove s.message.
func (s *JSONMessageProcessor) getOutput() string {
	output := ""

	if s.lastProgressLength > 0 {
		output = fmt.Sprintf("\r%s", output)
	}

	if s.message != nil {
		messageOutput := formatCompleteOutput(s.message)
		filling := s.generateFilling(len(messageOutput))

		output = fmt.Sprintf("%s%s%s\n", output, messageOutput, filling)
		s.message = nil
	}

	pointer := 0
	keys := make([]string, len(s.progressMessages))
	for key := range s.progressMessages {
		keys[pointer] = key
		pointer++
	}

	sort.Strings(keys)

	buffer := make([]string, len(s.progressMessages))
	for i, key := range keys {
		buffer[i] = formatProgressOutput(s.progressMessages[key])
	}

	// Create progress message and optionally fill it to match previous message
	// length
	progressMessage := strings.Join(buffer, ", ")
	progressFilling := s.generateFilling(len(progressMessage))

	// Update with the current line
	s.lastProgressLength = len(progressMessage)

	output = fmt.Sprintf("%s%s%s", output, progressMessage, progressFilling)

	return output
}

// formatCompleteOutput will format the message m as an completed message.
func formatCompleteOutput(m *jsonmessage.JSONMessage) string {
	if strings.HasPrefix(m.Status, "The push refers to a repository") {
		return "Pushing to registry"
	}

	if strings.HasPrefix(m.Status, "Pushing repository") &&
		strings.HasSuffix(m.Status, "tags)") {
		tags := 0
		registry := ""
		fmt.Sscanf(m.Status, "Pushing repository %s (%d tags)", &registry, &tags)
		return fmt.Sprintf("Pushing %d tag(s)", tags)
	}

	if strings.HasPrefix(m.Status, "Pushing tag for rev [") &&
		strings.HasSuffix(m.Status, "};") {
		image := ""
		registry := ""
		fmt.Sscanf(m.Status, "Pushing tag for rev [%s] on {%s};", &image, &registry)
		image = strings.TrimSuffix(image, "]")
		return fmt.Sprintf("Pushing tag for image: %s", image)
	}

	if m.ID != "" {
		return fmt.Sprintf("%s: %s", m.Status, m.ID)
	}

	return m.Status
}

// formatProgressOutput will format the message m as an progress message.
func formatProgressOutput(m *jsonmessage.JSONMessage) string {
	if m.Status == "Buffering to disk" {
		progress := formatDiskUnit(int64(m.Progress.Current))
		return fmt.Sprintf("%s: %s (%s)", m.Status, m.ID, progress)
	}

	progress := ""
	if m.Progress != nil && m.Progress.Total != 0 {
		progress = fmt.Sprintf(" (%d%%)", calculateProgress(m.Progress))
	}
	return fmt.Sprintf("%s: %s%s", m.Status, m.ID, progress)
}

// round will round the value val.
func round(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}

// formatDiskUnit will format b (amount of bytes) to include a postfix. It will
// try to fit b in the biggest unit.
func formatDiskUnit(b int64) string {
	units := []string{"B", "KB", "MB", "GB"}
	totalUnits := len(units)
	value := float64(b)
	pointer := 0

	for {
		if pointer+1 == totalUnits {
			break
		}

		if value >= 1024 {
			value = value / 1024
			pointer++
		} else {
			break
		}
	}

	// Always round down and round at 1 point precision
	value = round(value, 1, 1)

	// Use -1 precision which will result in no point or 1 point precision
	v := strconv.FormatFloat(value, 'f', -1, 64)

	return fmt.Sprintf("%s %s", v, units[pointer])
}

// calculateProgress will calculate the percentage based on p. It will return 0
// if p.Total equals 0.
func calculateProgress(p *jsonmessage.JSONProgress) int {
	if p.Total == 0 {
		return 0
	}

	return int((100 * p.Current) / p.Total)
}
