package core

import (
	"io/ioutil"
	"os"
)

// WerckerInitStep needs to implemenet IStep
type WerckerInitStep struct {
	*ExternalStep
}

var initSh = `#!/bin/sh +xe
WARN_COLOR="[33m[1m"
SUCCESS_COLOR="[32m[1m"
ERROR_COLOR="[31m[1m"
INFO_COLOR="[37m[1m"
DEBUG_COLOR="[38m[1m"
RESET_COLOR="[m"

_message() {
    msg=$1
    color=$2
    printf "%b%b%b\n" "${color}" "${msg}" "${RESET_COLOR}"
}

success() {
    _message "${1}" "$SUCCESS_COLOR"
}

info() {
    _message "${1}" "$INFO_COLOR"
}

debug() {
    _message "${1}" "$DEBUG_COLOR"
}

warn() {
    _message "${1}" "$WARN_COLOR"
}

error() {
    _message "error: ${1}" "$ERROR_COLOR"
}

fail() {
    _message "failed: ${1}" "$ERROR_COLOR"
    echo "${1}" > "$WERCKER_REPORT_MESSAGE_FILE"
    exit 1
}

setMessage() {
  echo "${1}" > "$WERCKER_REPORT_MESSAGE_FILE"
}

if command -v shopt >/dev/null 2>&1; then
  # Because we aren't in an interactive shell, we need to set expand_aliases
  # manually, to override sudo in scripts.
  shopt -s expand_aliases
fi

# NOTE(termie): We're overriding sudo because when using Docker the
#               containers usually don't have it installed. This may prove
#               to be a bigger issue in the future but for now it seems to
#               be working.
alias sudo=""

# Make sure we fail on all errors
set -e
`

// NewWerckerInitStep is a special step.
func NewWerckerInitStep(options *PipelineOptions) (*WerckerInitStep, error) {
	werckerInit := "wercker/wercker-init@2.1.0"
	stepConfig := &StepConfig{ID: werckerInit, Data: make(map[string]string)}
	initStep, err := NewStep(stepConfig, options)
	if err != nil {
		return nil, err
	}

	return &WerckerInitStep{
		ExternalStep: initStep,
	}, nil
}

// Fetch saves predefined `init.sh`.
func (s *WerckerInitStep) Fetch() (string, error) {
	hostStepPath := s.options.HostPath(s.safeID)
	scriptPath := s.options.HostPath(s.safeID, "init.sh")

	err := os.MkdirAll(hostStepPath, 0755)
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(scriptPath, []byte(initSh), 0755)
	if err != nil {
		return "", err
	}

	return hostStepPath, nil
}
