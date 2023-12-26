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
	"reflect"
	"sort"
	"strings"

	"github.com/chuckpreslar/emission"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

const (
	// Logs is the event when wercker generate logs
	Logs = "Logs"

	// BuildStarted is the event when wercker has started a build.
	BuildStarted = "BuildStarted"

	// BuildFinished occures when a pipeline finishes the main phase. It is
	// possible that after-steps are run after this event.
	BuildFinished = "BuildFinished"

	// BuildStepsAdded is the event when wercker has parsed the wercker.yml and
	// has valdiated that the steps exist.
	BuildStepsAdded = "BuildStepsAdded"

	// BuildStepStarted is the event when wercker has started a new buildstep.
	BuildStepStarted = "BuildStepStarted"

	// BuildStepFinished is the event when wercker has finished a buildstep.
	BuildStepFinished = "BuildStepFinished"

	// FullPipelineFinished occurs when a pipeline finishes all it's steps,
	// included after-steps.
	FullPipelineFinished = "FullPipelineFinished"
)

// BuildStartedArgs contains the args associated with the "BuildStarted" event.
type BuildStartedArgs struct {
	Options *PipelineOptions
}

// BuildFinishedArgs contains the args associated with the "BuildFinished"
// event.
type BuildFinishedArgs struct {
	Box     Box
	Options *PipelineOptions
	Result  string
}

// LogsArgs contains the args associated with the "Logs" event.
type LogsArgs struct {
	Build   Pipeline
	Options *PipelineOptions
	Order   int
	Step    Step
	Logs    string
	Stream  string
	Hidden  bool
}

// BuildStepsAddedArgs contains the args associated with the
// "BuildStepsAdded" event.
type BuildStepsAddedArgs struct {
	Build      Pipeline
	Options    *PipelineOptions
	Steps      []Step
	StoreStep  Step
	AfterSteps []Step
}

// BuildStepStartedArgs contains the args associated with the
// "BuildStepStarted" event.
type BuildStepStartedArgs struct {
	Options *PipelineOptions
	Box     Box
	Build   Pipeline
	Order   int
	Step    Step
}

// BuildStepFinishedArgs contains the args associated with the
// "BuildStepFinished" event.
type BuildStepFinishedArgs struct {
	Options     *PipelineOptions
	Box         Box
	Build       Pipeline
	Order       int
	Step        Step
	Successful  bool
	Message     string
	ArtifactURL string
	// Only applicable to the store step
	PackageURL string
	// Only applicable to the setup environment step
	WerckerYamlContents string
}

// FullPipelineFinishedArgs contains the args associated with the
// "FullPipelineFinished" event.
type FullPipelineFinishedArgs struct {
	Options             *PipelineOptions
	MainSuccessful      bool
	RanAfterSteps       bool
	AfterStepSuccessful bool
}

// DebugHandler dumps events
type DebugHandler struct {
	logger *util.LogEntry
}

// NewDebugHandler constructor
func NewDebugHandler() *DebugHandler {
	logger := util.RootLogger().WithField("Logger", "Events")
	return &DebugHandler{logger: logger}
}

// dumpEvent prints out some debug info about an event
func (h *DebugHandler) dumpEvent(event interface{}, indent ...string) {
	indent = append(indent, "  ")
	s := reflect.ValueOf(event).Elem()

	typeOfT := s.Type()
	names := []string{}
	for i := 0; i < s.NumField(); i++ {
		// f := s.Field(i)
		fieldName := typeOfT.Field(i).Name
		if fieldName != "Env" {
			names = append(names, fieldName)
		}
	}
	sort.Strings(names)

	for _, name := range names {

		r := reflect.ValueOf(event)
		f := reflect.Indirect(r).FieldByName(name)
		if name == "Options" {
			continue
		}
		if name[:1] == strings.ToLower(name[:1]) {
			// Not exported, skip it
			h.logger.Debugln(fmt.Sprintf("%s%s %s = %v", strings.Join(indent, ""), name, f.Type(), "<not exported>"))
			continue
		}
		if name == "Box" || name == "Step" {
			h.logger.Debugln(fmt.Sprintf("%s%s %s", strings.Join(indent, ""), name, f.Type()))
			if !f.IsNil() {
				h.dumpEvent(f.Interface(), indent...)
			}
		} else {
			h.logger.Debugln(fmt.Sprintf("%s%s %s = %v", strings.Join(indent, ""), name, f.Type(), f.Interface()))
		}
	}
}

// Handler returns a per-event dumpEvent
func (h *DebugHandler) Handler(name string) func(interface{}) {
	return func(event interface{}) {
		h.logger.Debugln(name)
		h.dumpEvent(event)
	}
}

// ListenTo attaches to the emitter
func (h *DebugHandler) ListenTo(e *NormalizedEmitter) {
	e.AddListener(BuildStarted, h.Handler("BuildStarted"))
	e.AddListener(BuildFinished, h.Handler("BuildFinished"))
	e.AddListener(BuildStepsAdded, h.Handler("BuildStepsAdded"))
	e.AddListener(BuildStepStarted, h.Handler("BuildStepStarted"))
	e.AddListener(BuildStepFinished, h.Handler("BuildStepFinished"))
	e.AddListener(FullPipelineFinished, h.Handler("FullPipelineFinished"))
}

// NormalizedEmitter wraps the emission.Emitter and is smart enough about
// our events to fill in details as needed so that we don't need so many args
type NormalizedEmitter struct {
	*emission.Emitter

	// All these are initially unset
	options      *PipelineOptions // Set by BuildStarted
	build        Pipeline         // Set by BuildStepsAdded
	currentOrder int              // Set by BuildStepStarted
	currentStep  Step             // Set by BuildStepStarted
}

// NewNormalizedEmitter constructor
func NewNormalizedEmitter() *NormalizedEmitter {
	return &NormalizedEmitter{Emitter: emission.NewEmitter()}
}

// Emit normalizes our events by storing some state
func (e *NormalizedEmitter) Emit(event interface{}, args interface{}) {
	switch event {
	// store the options for later
	case BuildStarted:
		a := args.(*BuildStartedArgs)
		e.options = a.Options
		e.Emitter.Emit(event, a)
	// Store the build, add the options
	case BuildStepsAdded:
		a := args.(*BuildStepsAddedArgs)
		if a.Options == nil {
			a.Options = e.options
		}
		e.build = a.Build
		e.Emitter.Emit(event, a)
	// Store step and order, add options, build
	case BuildStepStarted:
		a := args.(*BuildStepStartedArgs)
		if a.Options == nil {
			a.Options = e.options
		}
		if a.Build == nil {
			a.Build = e.build
		}
		e.currentStep = a.Step
		e.currentOrder = a.Order
		e.Emitter.Emit(event, a)
	// Add options, build, step, order, default stream
	case Logs:
		a := args.(*LogsArgs)
		if a.Options == nil {
			a.Options = e.options
		}
		if a.Build == nil {
			a.Build = e.build
		}
		if a.Step == nil {
			a.Step = e.currentStep
		}
		if a.Order == 0 {
			a.Order = e.currentOrder
		}
		if a.Stream == "" {
			a.Stream = "stdout"
		}
		e.Emitter.Emit(event, a)
	// Add options, build, step, order, reset step and order after
	case BuildStepFinished:
		a := args.(*BuildStepFinishedArgs)
		if a.Options == nil {
			a.Options = e.options
		}
		if a.Build == nil {
			a.Build = e.build
		}
		if a.Step == nil {
			a.Step = e.currentStep
		}
		if a.Order == 0 {
			a.Order = e.currentOrder
		}
		e.Emitter.Emit(event, a)
		e.currentStep = nil
		e.currentOrder = -1
	// Just add the options
	case BuildFinished:
		a := args.(*BuildFinishedArgs)
		if a.Options == nil {
			a.Options = e.options
		}
		e.Emitter.Emit(event, a)
	// Just add the options
	case FullPipelineFinished:
		a := args.(*FullPipelineFinishedArgs)
		if a.Options == nil {
			a.Options = e.options
		}
		e.Emitter.Emit(event, a)
	}
}

// NewEmitterContext gives us a new context with an emitter
func NewEmitterContext(ctx context.Context) context.Context {
	e := NewNormalizedEmitter()
	return context.WithValue(ctx, "Emitter", e)
}

// EmitterFromContext gives us the emitter attached to the context
func EmitterFromContext(ctx context.Context) (e *NormalizedEmitter, err error) {
	e, ok := ctx.Value("Emitter").(*NormalizedEmitter)
	if !ok {
		err = fmt.Errorf("Cannot get emitter from context.")
	}
	return e, err
}
