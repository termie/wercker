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
	"github.com/wercker/reporter-client"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// NewReportHandler will create a new ReportHandler.
func NewReportHandler(werckerHost, token string) (*ReportHandler, error) {
	logger := util.RootLogger().WithField("Logger", "Reporter")

	r, err := reporter.NewClient(werckerHost, token)
	if err != nil {
		return nil, err
	}

	writers := make(map[string]*reporter.LogWriter)
	h := &ReportHandler{
		reporter: r,
		writers:  writers,
		logger:   logger,
	}
	return h, nil
}

func mapSteps(phase string, steps ...core.Step) []reporter.NewStep {
	buffer := make([]reporter.NewStep, len(steps))
	for i, s := range steps {
		buffer[i] = reporter.NewStep{
			DisplayName: s.DisplayName(),
			Name:        s.Name(),
			Phase:       phase,
			StepSafeID:  s.SafeID(),
		}
	}
	return buffer
}

// A ReportHandler reports all events to the wercker-api.
type ReportHandler struct {
	reporter *reporter.ReportingClient
	writers  map[string]*reporter.LogWriter
	logger   *util.LogEntry
}

// BuildStepStarted will handle the BuildStepStarted event.
func (h *ReportHandler) StepStarted(args *core.BuildStepStartedArgs) {
	opts := reporter.RunStepStartedArgs{
		RunID:      args.Options.RunID,
		StepSafeID: args.Step.SafeID(),
	}

	h.reporter.RunStepStarted(context.TODO(), opts)
}

func (h *ReportHandler) flushLogs(safeID string) error {
	if writer, ok := h.writers[safeID]; ok {
		return writer.Flush()
	}

	return nil
}

// BuildStepFinished will handle the BuildStepFinished event.
func (h *ReportHandler) StepFinished(args *core.BuildStepFinishedArgs) {
	h.flushLogs(args.Step.SafeID())

	result := "failed"
	if args.Successful {
		result = "passed"
	}

	opts := reporter.RunStepFinishedArgs{
		RunID:               args.Options.RunID,
		StepSafeID:          args.Step.SafeID(),
		Result:              result,
		ArtifactURL:         args.ArtifactURL,
		PackageURL:          args.PackageURL,
		Message:             args.Message,
		WerckerYamlContents: args.WerckerYamlContents,
	}

	h.reporter.RunStepFinished(context.TODO(), opts)
}

// BuildStepsAdded will handle the BuildStepsAdded event.
func (h *ReportHandler) StepsAdded(args *core.BuildStepsAddedArgs) {
	steps := mapSteps("mainSteps", args.Steps...)

	if args.StoreStep != nil {
		storeStep := mapSteps("mainSteps", args.StoreStep)
		steps = append(steps, storeStep...)
	}

	afterSteps := mapSteps("finalSteps", args.AfterSteps...)
	steps = append(steps, afterSteps...)

	opts := reporter.RunStepsAddedArgs{
		RunID: args.Options.RunID,
		Steps: steps,
	}

	h.reporter.RunStepsAdded(context.TODO(), opts)
}

// getStepOutputWriter will check h.writers for a writer for the step, otherwise
// it will create a new one.
func (h *ReportHandler) getStepOutputWriter(args *core.LogsArgs) (*reporter.LogWriter, error) {
	key := args.Step.SafeID()

	writer, ok := h.writers[key]
	if !ok {
		w, err := reporter.NewLogWriter(h.reporter, args.Options.RunID, args.Step.SafeID(), args.Stream)
		if err != nil {
			return nil, err
		}
		h.writers[key] = w
		writer = w
	}

	return writer, nil
}

// Logs will handle the Logs event.
func (h *ReportHandler) Logs(args *core.LogsArgs) {
	if args.Hidden {
		return
	}

	if args.Step == nil {
		return
	}

	w, err := h.getStepOutputWriter(args)
	if err != nil {
		h.logger.WithField("Error", err).Error("Unable to create step output writer")
		return
	}
	w.Write([]byte(args.Logs))
}

// BuildFinished will handle the BuildFinished event.
func (h *ReportHandler) PipelineFinished(args *core.BuildFinishedArgs) {
	opts := reporter.RunFinishedArgs{
		RunID:  args.Options.RunID,
		Result: args.Result,
	}
	h.reporter.RunFinished(context.TODO(), opts)
}

// FullPipelineFinished closes current writers, making sure they have flushed
// their logs.
func (h *ReportHandler) FullPipelineFinished(args *core.FullPipelineFinishedArgs) {
	h.Close()
}

// Close will call close on any log writers that have been created.
func (h *ReportHandler) Close() error {
	for _, w := range h.writers {
		w.Close()
	}

	return nil
}

// ListenTo will add eventhandlers to e.
func (h *ReportHandler) ListenTo(e *core.NormalizedEmitter) {
	e.AddListener(core.BuildFinished, h.PipelineFinished)
	e.AddListener(core.BuildStepFinished, h.StepFinished)
	e.AddListener(core.BuildStepsAdded, h.StepsAdded)
	e.AddListener(core.BuildStepStarted, h.StepStarted)
	e.AddListener(core.FullPipelineFinished, h.FullPipelineFinished)
	e.AddListener(core.Logs, h.Logs)
}
