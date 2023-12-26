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

package event

import (
	log "github.com/sirupsen/logrus"
	"github.com/wercker/reporter-client"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
)

// NewLiteralLogHandler will create a new LiteralLogHandler.
func NewLiteralLogHandler(options *core.PipelineOptions) (*LiteralLogHandler, error) {
	var logger *util.Logger

	if options.Debug {
		logger = util.RootLogger()
	} else {
		logger = util.NewLogger()
		logger.Formatter = &reporter.LiteralFormatter{}
		logger.Level = log.InfoLevel
	}

	return &LiteralLogHandler{l: logger, options: options}, nil
}

// A LiteralLogHandler logs all events using Logrus.
type LiteralLogHandler struct {
	l       *util.Logger
	options *core.PipelineOptions
}

// Logs will handle the Logs event.
func (h *LiteralLogHandler) Logs(args *core.LogsArgs) {
	if args.Stream == "" {
		args.Stream = "stdout"
	}
	if h.options.Debug {
		shown := "[x]"
		if args.Hidden {
			shown = "[ ]"
		}
		h.l.WithFields(util.LogFields{
			"Logger": "Literal",
			"Hidden": args.Hidden,
			"Stream": args.Stream,
		}).Printf("%s %6s %q", shown, args.Stream, args.Logs)
	} else if h.shouldPrintLog(args) {
		h.l.Print(args.Logs)
	}
}

func (h *LiteralLogHandler) shouldPrintLog(args *core.LogsArgs) bool {
	if args.Hidden {
		return false
	}

	// Do not show stdin stream is verbose is false
	if args.Stream == "stdin" && !h.options.Verbose {
		return false
	}

	return true
}

// ListenTo will add eventhandlers to e.
func (h *LiteralLogHandler) ListenTo(e *core.NormalizedEmitter) {
	e.AddListener(core.Logs, h.Logs)
}
