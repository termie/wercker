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
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/fsnotify/fsnotify.v1"

	"github.com/pborman/uuid"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
)

// test TODO (mh)
// 1. change multiple files simultaneously and show that build only happens
//    once
// 2. what happens when files written while build running. queue build?
//    make sure we don't run multiple builds in parallel

// WatchStep needs to implemenet IStep
type WatchStep struct {
	*core.BaseStep
	Code          string
	reload        bool
	data          map[string]string
	logger        *util.LogEntry
	options       *core.PipelineOptions
	dockerOptions *Options
}

// NewWatchStep is a special step for doing docker pushes
func NewWatchStep(stepConfig *core.StepConfig, options *core.PipelineOptions, dockerOptions *Options) (*WatchStep, error) {
	name := "watch"
	displayName := "watch"
	if stepConfig.Name != "" {
		displayName = stepConfig.Name
	}

	// Add a random number to the name to prevent collisions on disk
	stepSafeID := fmt.Sprintf("%s-%s", name, uuid.NewRandom().String())

	baseStep := core.NewBaseStep(core.BaseStepOptions{
		DisplayName: displayName,
		Env:         util.NewEnvironment(),
		ID:          name,
		Name:        name,
		Owner:       "wercker",
		SafeID:      stepSafeID,
		Version:     util.Version(),
	})

	return &WatchStep{
		BaseStep:      baseStep,
		options:       options,
		dockerOptions: dockerOptions,
		data:          stepConfig.Data,
		logger:        util.RootLogger().WithField("Logger", "WatchStep"),
	}, nil
}

// InitEnv parses our data into our config
func (s *WatchStep) InitEnv(ctx context.Context, env *util.Environment) error {
	if code, ok := s.data["code"]; ok {
		s.Code = code
	}
	if reload, ok := s.data["reload"]; ok && reload != "" {
		if v, err := strconv.ParseBool(reload); err == nil {
			s.reload = v
		} else {
			return fmt.Errorf("%s is an invalid value for reload, error while validating: %s", reload, err.Error())
		}
	}
	return nil
}

// Fetch NOP
func (s *WatchStep) Fetch() (string, error) {
	// nop
	return "", nil
}

// filterGitignore tries to exclude patterns defined in gitignore
func (s *WatchStep) filterGitignore(root string) []string {
	filters := []string{}
	gitignorePath := filepath.Join(root, ".gitignore")
	file, err := os.Open(gitignorePath)
	if err == nil {
		s.logger.Debug("Excluding file patterns in .gitignore")
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			t := strings.Trim(scanner.Text(), " ")
			if t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			filters = append(filters, filepath.Join(root, t))
		}
	}
	return filters
}

func (s *WatchStep) watch(root string) (*fsnotify.Watcher, error) {
	// Set up the filesystem watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	filters := []string{
		fmt.Sprintf("%s*", s.options.StepPath()),
		fmt.Sprintf("%s*", s.options.ProjectDownloadPath()),
		fmt.Sprintf("%s*", s.options.BuildPath()),
		".*",
		"_*",
	}

	watchCount := 0

	// import a .gitignore if it exists
	filters = append(filters, s.filterGitignore(root)...)

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if err != nil {
				return err
			}
			partialPath := filepath.Base(path)

			s.logger.Debugln("check path", path, partialPath)
			for _, pattern := range filters {
				matchFull, err := filepath.Match(pattern, path)
				if err != nil {
					s.logger.Warnln("Bad exclusion pattern: %s", pattern)
				}
				if matchFull {
					s.logger.Debugf("exclude (%s): %s", pattern, path)
					return filepath.SkipDir
				}
				matchPartial, _ := filepath.Match(pattern, partialPath)
				if matchPartial {
					s.logger.Debugf("exclude (%s): %s", pattern, partialPath)
					return filepath.SkipDir
				}
			}
			s.logger.Debugln("Watching:", path)
			watchCount = watchCount + 1
			if err := watcher.Add(path); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Debugf("Watching %d directories", watchCount)
	return watcher, nil
}

// killProcesses sends a signal to all the processes on the machine except
// for PID 1, somewhat naive but seems to work
func (s *WatchStep) killProcesses(containerID string, signal string) error {
	client, err := NewDockerClient(s.dockerOptions)
	if err != nil {
		return err
	}
	details, err := client.InspectContainer(containerID)
	if err != nil {
		return err
	}
	var cmd []string
	if len(details.Args) > 0 && details.Args[len(details.Args)-1] == `if [ -e /bin/bash ]; then /bin/bash; else /bin/sh; fi` {
		cmd = []string{`/bin/sh`, `-c`, fmt.Sprintf(`ps -eaf | grep -v PID | awk "{if (\$2 != 1 && \$3 != 1) print \$2}" | xargs -n 1 kill -s %s`, signal)}
	} else {
		cmd = []string{`/bin/sh`, `-c`, fmt.Sprintf(`ps | grep -v PID | awk "{if (\$1 != 1) print \$1}" | xargs -n 1 kill -s %s`, signal)}
	}

	err = client.ExecOne(containerID, cmd, os.Stdout)
	if err != nil {
		return err
	}
	return nil
}

// Execute runs a command and optionally reloads it
func (s *WatchStep) Execute(ctx context.Context, sess *core.Session) (int, error) {
	e, err := core.EmitterFromContext(ctx)
	if err != nil {
		return -1, err
	}

	// TODO(termie): PACKAGING make this a feature of session and remove
	//               the calls into its struct
	// Start watching our stdout
	stopListening := make(chan struct{})
	defer func() { stopListening <- struct{}{} }()
	go func() {
		for {
			select {
			case line := <-sess.Recv():
				e.Emit(core.Logs, &core.LogsArgs{
					// Hidden: sess.logsHidden,
					Logs: line,
				})
			// We need to make sure we stop eating the stdout from the container
			// promiscuously when we finish out step
			case <-stopListening:
				return
			}
		}
	}()

	// cheating to get containerID
	// TODO(termie): we should deal with this eventually
	dt := sess.Transport().(*DockerTransport)
	containerID := dt.containerID

	// Set up a signal handler to end our step.
	finishedStep := make(chan struct{})
	stopWatchHandler := &util.SignalHandler{
		ID: "stop-watch",
		// Signal our stuff to stop and finish the step, return false to
		// signify that we've handled the signal and don't process further
		F: func() bool {
			s.logger.Println("Keyboard interrupt detected, finishing step")
			finishedStep <- struct{}{}
			return false
		},
	}
	util.GlobalSigint().Add(stopWatchHandler)
	// NOTE(termie): I think the only way to exit this code is via this
	//               signal handler and the signal monkey removes handlers
	//               after it processes them, so this may be superfluous
	defer util.GlobalSigint().Remove(stopWatchHandler)

	// If we're not going to reload just run the thing once, synchronously
	if !s.reload {
		err := sess.Send(ctx, false, "set +e", s.Code)
		if err != nil {
			return 0, err
		}
		<-finishedStep
		// ignoring errors
		s.killProcesses(containerID, "INT")
		return 0, nil
	}
	f := &util.Formatter{ShowColors: s.options.GlobalOptions.ShowColors}
	s.logger.Info(f.Info("Reloading on file changes"))
	doCmd := func() {
		err := sess.Send(ctx, false, "set +e", s.Code)
		if err != nil {
			s.logger.Errorln(err)
			return
		}
		open, err := exposedPortMaps(s.dockerOptions.Host, s.options.PublishPorts)
		if err != nil {
			s.logger.Warnf(f.Info("There was a problem parsing your docker host."), err)
			return
		}
		for _, uri := range open {
			s.logger.Infof(f.Info("Forwarding %s to %s on the container."), uri.HostURI, uri.ContainerPort)
		}
	}

	// Otherwise set up a watcher and do some magic
	watcher, err := s.watch(s.options.ProjectPath)
	if err != nil {
		return -1, err
	}

	debounce := util.NewDebouncer(2 * time.Second)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				s.logger.Debugln("fsnotify event", event.String())
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Remove == fsnotify.Remove {
					if !strings.HasPrefix(filepath.Base(event.Name), ".") {
						s.logger.Debug(f.Info("Modified file", event.Name))
						debounce.Trigger()
					}
				}
			case <-debounce.C:
				err := s.killProcesses(containerID, "INT")
				if err != nil {
					s.logger.Panic(err)
					return
				}
				s.logger.Info(f.Info("Reloading"))
				go doCmd()
			case err := <-watcher.Errors:
				s.logger.Error(err)
				done <- struct{}{}
				return
			case <-finishedStep:
				s.killProcesses(containerID, "INT")
				done <- struct{}{}
				return
			}
		}
	}()

	// Run build on first run
	debounce.Trigger()
	<-done
	return 0, nil
}

// CollectFile NOP
func (s *WatchStep) CollectFile(a, b, c string, dst io.Writer) error {
	return nil
}

// CollectArtifact NOP
func (s *WatchStep) CollectArtifact(context.Context, string) (*core.Artifact, error) {
	return nil, nil
}

// ReportPath getter
func (s *WatchStep) ReportPath(...string) string {
	// for now we just want something that doesn't exist
	return uuid.NewRandom().String()
}

// ShouldSyncEnv before running this step = FALSE
func (s *WatchStep) ShouldSyncEnv() bool {
	return false
}
