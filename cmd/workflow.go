//   Copyright © 2018, 2019, Oracle and/or its affiliates.  All rights reserved.
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

package cmd

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/wercker/wercker/core"
	dockerlocal "github.com/wercker/wercker/docker"
	"github.com/wercker/wercker/util"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2/bson"
)

// cmdWorkflow is the main driver for running a workflow.
func cmdWorkflow(ctx context.Context, opts *core.WorkflowOptions, dockerOptions *dockerlocal.Options) error {
	// PipelineName->RunId map to keep track of which pipelines have ran
	// and their runIDs.
	pipelineRunMap := map[string]string{}

	config, err := getConfig(opts)
	if err != nil {
		return errors.Wrap(err, "failed to read the yml file")
	}

	workflow := config.GetWorkflow(opts.WorkflowName)
	if workflow == nil {
		return errors.Errorf("%s does not contain workflow %s", opts.PipelineOptions.WerckerYml, opts.WorkflowName)
	}

	err = workflow.Validate(config)
	if err != nil {
		return errors.Wrap(err, "invalid workflow")
	}

	for {
		pipeline, sourceRunIDs := nextPipeline(workflow, pipelineRunMap)
		if pipeline == nil {
			// Done with the workflow, no more pipelines to execute.
			break
		}

		pipelineOpts, err := getPipelineOptions(opts, pipeline, sourceRunIDs, pipelineRunMap)
		if err != nil {
			return err
		}

		runID, err := runPipeline(ctx, pipelineOpts, dockerOptions)
		if err != nil {
			return err
		}

		pipelineRunMap[pipeline.Name] = runID
	}

	return nil
}

func getConfig(opts *core.WorkflowOptions) (*core.Config, error) {
	var werckerYaml []byte
	var err error

	if opts.PipelineOptions.WerckerYml != "" {
		werckerYaml, err = ioutil.ReadFile(opts.PipelineOptions.WerckerYml)
	} else {
		werckerYaml, err = core.ReadWerckerYaml([]string{"."}, false)
		if err != nil {
			return nil, err
		}
		opts.PipelineOptions.WerckerYml, _ = filepath.Abs("./wercker.yml")
	}

	if err != nil {
		return nil, err
	}

	config, err := core.ConfigFromYaml(werckerYaml)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// nextPipeline returns the next pipeline to execute along with the list of IDs
// of the pipeline's source runs. nil pipeline means end of the workflow.
//
// One source runID means either sequential execution or fan-in with the source
// artifact pipeline set. Multiple runIDs means fan-in with all source pipelines
// acting as artifact sources.
func nextPipeline(workflow *core.WorkflowConfig, pipelineRunMap map[string]string) (*core.WorkflowPipelineConfig, []string) {
	for _, pipeline := range workflow.Pipelines {
		if _, ok := pipelineRunMap[pipeline.Name]; ok {
			continue
		}

		sourceRunIDs := []string{}
		artifactPipelineRunID := ""

		for _, requiredPipeline := range pipeline.Requires {
			runID, ok := pipelineRunMap[requiredPipeline]
			if ok {
				sourceRunIDs = append(sourceRunIDs, runID)
			}

			if pipeline.ArtifactPipeline == requiredPipeline {
				artifactPipelineRunID = runID
			}
		}

		// Check if all of the required pipelines have been executed.
		if len(sourceRunIDs) != len(pipeline.Requires) {
			continue
		}

		// If the `artifactPipeline` parameter is set, return only that
		// pipeline's runID.
		if artifactPipelineRunID != "" {
			sourceRunIDs = []string{artifactPipelineRunID}
		}

		return &pipeline, sourceRunIDs
	}

	return nil, nil
}

func getPipelineOptions(opts *core.WorkflowOptions, pipeline *core.WorkflowPipelineConfig, sourceRunIDs []string, pipelineRunMap map[string]string) (*core.PipelineOptions, error) {
	// Use the initial PipelineOptions as the basis.
	po := opts.PipelineOptions

	// Here, the pipeline name must be the one defined
	// in the pipelines' section of yml file.
	po.Pipeline = pipeline.GetYAMLPipelineName()

	po.RunID = bson.NewObjectId().Hex()
	po.ShouldArtifacts = true

	if pipeline.EnvFile != "" {
		po.HostEnv = util.DefaultEnvironment(pipeline.EnvFile)
	}

	err := configureProjectPath(&po, pipeline.Name, sourceRunIDs, pipelineRunMap)
	if err != nil {
		return nil, err
	}

	return &po, nil
}

func configureProjectPath(opts *core.PipelineOptions, name string, sourceRunIDs []string, pipelineStatus map[string]string) error {
	if len(sourceRunIDs) == 1 {
		latestPath, err := outputPath(opts, sourceRunIDs[0])
		if err != nil {
			return err
		}

		opts.ProjectPath = latestPath
	} else if len(sourceRunIDs) > 1 {
		// This is the fan-in case where all source pipelines act as
		// artifacts sources.
		opts.ProjectPathsByPipeline = make(map[string]string)

		for _, sourceRunID := range sourceRunIDs {
			sourcePipelineName := ""
			for name, runID := range pipelineStatus {
				if sourceRunID == runID {
					sourcePipelineName = name
					break
				}
			}

			if sourcePipelineName == "" {
				return errors.Errorf("no pipeline for source runID %s", sourceRunID)
			}

			latestPath, err := outputPath(opts, sourceRunID)
			if err != nil {
				return err
			}

			opts.ProjectPathsByPipeline[sourcePipelineName] = latestPath
		}
	}

	return nil
}

func outputPath(opts *core.PipelineOptions, runID string) (string, error) {
	latestPath := opts.WorkingPath("builds", runID, "output")

	found, err := util.Exists(latestPath)
	if err != nil {
		return "", errors.Wrapf(err, "unable to locate output from the run %s", opts.RunID)
	}

	if !found {
		return "", errors.Errorf("no output from the run %s", opts.RunID)
	}

	return filepath.Abs(latestPath)
}

func runPipeline(ctx context.Context, opts *core.PipelineOptions, dockerOptions *dockerlocal.Options) (string, error) {
	_, err := cmdDeploy(ctx, opts, dockerOptions)
	if err != nil {
		return "", errors.Wrapf(err, "unable to run pipeline %s", opts.Pipeline)
	}

	return opts.RunID, nil
}
