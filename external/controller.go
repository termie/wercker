// Copyright (c) 2018, Oracle and/or its affiliates. All rights reserved.

package external

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
)

// Used to unmarshal Docker json log
type logInfo struct {
	Time           string
	Level          string
	Msg            string
	Source         string
	JobId          string
	RunID          string
	AgentID        string
	ProjectID      string
	ProjectOwnerID string
}

// Detail for each external-runner container that has been created
// and started
type runnerContainer struct {
	containerName   string
	containerID     string
	containerStatus string
}

// RunnerParams are the parameters that drive the control of Docker
// containers where the external runner executes. This structure is
// passed from the Wercker CLI when runner is specified.
type RunnerParams struct {
	BearerToken    string // API Bearer token
	InstanceName   string // Runner name
	GroupName      string // Runner group name
	ImageName      string // Docker image
	OrgID          string // Organization ID
	AppNames       string // Application names
	OrgList        string // Organizations
	Workflows      string // Workflows
	StorePath      string // Local storage location
	LoggerPath     string // Where to write logs
	RunnerCount    int    // Number of runner containers
	ShutdownFlag   bool   // Shutdown if true
	Debug          bool   // debug enabled
	AllOption      bool   // --all option
	NoWait         bool   // --nowait options
	PullRemote     bool   // --pull option
	PollFreq       int    // Polling frequency
	DockerEndpoint string // docker endpoint
	OCIOptions     *core.OCIOptions
	OCIDownload    string // OCI Download endpoint
	// following values are set during processing
	Basename      string // base name for container creation
	Logger        *util.LogEntry
	client        *docker.Client
	containers    []*runnerContainer
	ProdType      bool // Set to true for production
	OverrideImage string
}

// NewDockerController -
func NewDockerController() *RunnerParams {
	return &RunnerParams{
		ImageName: "iad.ocir.io/odx-pipelines/wercker/wercker-runner:latest",
		ProdType:  false,
	}
}

// RunDockerController is commander-in-chief of external runners. It is called from
// Wercker CLI to start or stop external runners. The Wercker CLI builds the RunnParams and
// calls this function.
func (cp *RunnerParams) RunDockerController(statusOnly bool) {
	// When no instance name was supplied, use the hostname
	cp.Basename = cp.InstanceName
	if cp.InstanceName == "" {
		hostName, err := os.Hostname()

		if err != nil {
			cp.Logger.Fatal(fmt.Sprintf("unable to access hostname: %s", err))
			return
		}
		cp.Basename = hostName
	}

	cli, err := docker.NewClient(cp.DockerEndpoint)
	if err != nil {
		cp.Logger.Fatal(fmt.Sprintf("unable to create the Docker client: %s", err))
		return
	}
	cp.client = cli

	// Pickup proper image from local repository to be used for this run. WE are not checking
	// for a newer version from the remote repository.
	if !cp.ShutdownFlag {
		cp.CheckRegistryImages(true)
	}
	image, err := cp.getLocalImage()
	if err != nil {
		cp.Logger.Fatal(fmt.Sprintf("unable to access runner Docker image: %s", err))
		return
	}
	if image == nil {
		cp.Logger.Fatal("No runner image exists in your local Docker repository. Use wercker runner configure command.")
		return
	}

	// Get the list of running containers and determine if there are already
	// any running for the runner instance name.
	clist, err := cp.client.ListContainers(docker.ListContainersOptions{
		All: true,
	})

	// Pick out containers related to this runner instance set.
	var runners []*docker.Container
	lName := fmt.Sprintf("/wercker-external-runner-%s", cp.Basename)
	for _, dockerAPIContainer := range clist {
		for _, label := range dockerAPIContainer.Labels {
			if label == lName {
				dockerContainer, err := cp.client.InspectContainer(dockerAPIContainer.ID)
				if err == nil {
					runners = append(runners, dockerContainer)
					break
				}
			}
		}
	}

	// runners contains the containers running for this external runner
	if cp.ShutdownFlag {
		// Go handle shutdown of our runners.
		cp.shutdownRunners(runners)
		return
	}

	if statusOnly == true {
		if len(runners) > 0 {
			for _, dockerContainer := range runners {
				cname := stripSlashFromName(dockerContainer.Name)
				stats := dockerContainer.State.Status
				if stats != "running" {
					detail := fmt.Sprintf("Inactive runner container %s is being removed.", cname)
					cp.Logger.Print(detail)
					opts := docker.RemoveContainerOptions{
						ID: dockerContainer.ID,
					}
					cp.client.RemoveContainer(opts)
					continue
				}
				detail := fmt.Sprintf("Runner container: %s is active, status=%s", cname, stats)
				cp.Logger.Print(detail)
			}
			return
		}
		cp.Logger.Print("There are no runners active.")
		return
	}

	// OK, we want to start something.
	if len(runners) > 0 {
		detail := fmt.Sprintf("Runner(s) for %s already started.", cp.Basename)
		cp.Logger.Print(detail)
		return
	}

	// check if --all is valid
	if cp.AllOption {
		if cp.OrgList != "" || cp.Workflows != "" || cp.AppNames != "" {
			cp.Logger.Fatal("--all is not valid with --orgs, --apps, or --workflows")
		}
	} else {
		if strings.IndexByte(cp.GroupName, '@') == -1 && cp.OrgList == "" && cp.Workflows == "" && cp.AppNames == "" {
			cp.Logger.Fatal("--all must be specified when no other selection criteria")
		}
	}

	// Validate the group name. If omitted entirely then default to the base name. When present
	// check for an org specification and disallow spps/workspaces parameters. A simple group name
	// without the organization is passed along and does not invoke server-side filtering.
	if cp.GroupName == "" {
		cp.GroupName = cp.Basename
	} else {
		if strings.IndexByte(cp.GroupName, '@') != -1 {
			if cp.AppNames != "" || cp.Workflows != "" || cp.OrgList != "" || cp.AllOption {
				cp.Logger.Fatal("--all, --orgs, --apps or --workflows not allowed with groupname@organization")
			}
		}
	}

	cp.startTheRunners()
	if cp.StorePath != "" {
		message := fmt.Sprintf("Output is written to the %s directory", cp.StorePath)
		cp.Logger.Info(message)
	}

	cp.Logger.Debug("Running with ProdType: ", cp.ProdType)

	if !cp.NoWait {
		// Foreground processing. The Wercker command continues to run while
		// there are runner containers active.
		cp.waitForExternalRunners()
	} else {
		// Background processing, all the containers are started but logs are not
		// written because the Wecker command is ending and we cannot spawn
		// loggers to output the logs from the containers. Log information must
		// be obtained using the docker log command.
		cp.Logger.Info("Use the Wercker runner stop command with the same name to terminate the started runner(s).")
	}
}

// Starting runner(s).  Initiate a container to run the external runner for as many times as
// specified by the user.
func (cp *RunnerParams) startTheRunners() {
	if cp.BearerToken == "" {
		// Check if token is supplied in the environment and pick it up from
		// there.
		token := os.Getenv("WERCKER_RUNNER_TOKEN")
		if token == "" {
			cp.Logger.Fatal("Unable to start runner(s) because runner bearer token was not supplied.")
			return
		}
		cp.BearerToken = token
	}

	// Add sanity checks to make sure storepath and logpath actually exist. Without this
	// check a path with a type will result in a silent mount error when the container
	// is started. It will appear that the external runner is hung.
	if !checkPathExists(cp.StorePath) {
		cp.Logger.Fatal(fmt.Sprintf("Local storage path %s does not exist", cp.StorePath))
		return
	}
	if !checkPathExists(cp.LoggerPath) {
		cp.Logger.Fatal(fmt.Sprintf("Log output path %s does not exist", cp.LoggerPath))
		return
	}

	ct := 1
	for i := cp.RunnerCount; i > 0; i-- {
		runnerName := fmt.Sprintf("%s_%d", cp.Basename, ct)
		cmd, err := cp.createTheRunnerCommand(runnerName)
		if err == nil {
			cp.startTheContainer(runnerName, cmd)
			ct++
		}
	}
}

// Create the command to run the external runner in a container.
func (cp *RunnerParams) createTheRunnerCommand(name string) ([]string, error) {
	var cmd []string
	cmd = append(cmd, "/externalRunner.sh")
	//cmd = append(cmd, "--external-runner")
	cmd = append(cmd, fmt.Sprintf("--runner-image=%s", cp.ImageName))
	cmd = append(cmd, fmt.Sprintf("--runner-name=%s", name))
	cmd = append(cmd, fmt.Sprintf("--runner-api-token=%s", cp.BearerToken))
	if cp.GroupName != "" {
		cmd = append(cmd, fmt.Sprintf("--runner-group=%s", cp.GroupName))
	}
	if cp.OrgList != "" {
		cmd = append(cmd, fmt.Sprintf("--runner-orgs=%s", cp.OrgList))
	}
	if cp.AppNames != "" {
		cmd = append(cmd, fmt.Sprintf("--runner-apps=%s", cp.AppNames))
	}
	if cp.Workflows != "" {
		cmd = append(cmd, fmt.Sprintf("--runner-workflows=%s", cp.Workflows))
	}
	if cp.StorePath != "" {
		cmd = append(cmd, fmt.Sprintf("--runner-store-path=%s", cp.StorePath))
	}
	if cp.OCIDownload != "" {
		cmd = append(cmd, fmt.Sprintf("--runner-operator-url=%s", cp.OCIDownload))
	}
	if cp.OCIOptions != nil {
		if cp.OCIOptions.Namespace != "" {
			cmd = append(cmd, fmt.Sprintf("--runner-obj-store-namespace=%s", cp.OCIOptions.Namespace))
		}
		if cp.OCIOptions.Bucket != "" {
			cmd = append(cmd, fmt.Sprintf("--bucket-result=%s", cp.OCIOptions.Bucket))
		}
	}
	if cp.Debug == true {
		cmd = append(cmd, "-d")
		cmd = append(cmd, "--showlogs")
	}
	if cp.AllOption == true {
		cmd = append(cmd, "--runner-all")
	}
	if cp.PollFreq > 0 {
		cmd = append(cmd, fmt.Sprintf("--poll-frequency=%d", cp.PollFreq))
	}
	return cmd, nil
}

// Start the runner container(s). The command and arguments are supplied so
// create the container, then start it.
func (cp *RunnerParams) startTheContainer(name string, cmd []string) error {
	var args []string
	var labels []string
	var volumes []string

	labels = append(labels, fmt.Sprintf("runner=/wercker-external-runner-%s", cp.Basename))
	if cp.GroupName != "" {
		labels = append(labels, fmt.Sprintf("runnergroup=%s", cp.GroupName))
	}

	volumes = append(volumes, "/var/lib/wercker:/var/lib/wercker:rw")
	volumes = append(volumes, "/var/run/docker.sock:/var/run/docker.sock")
	if cp.LoggerPath != "" {
		volumes = append(volumes, fmt.Sprintf("%s:%s:rw", cp.LoggerPath, cp.LoggerPath))
	}
	if cp.StorePath != "" {
		volumes = append(volumes, fmt.Sprintf("%s:%s:rw", cp.StorePath, cp.StorePath))
	}

	var myenv []string
	myenv = append(myenv, fmt.Sprintf("WERCKER_RUNNER_TOKEN=%s", cp.BearerToken))

	// Using object storage?
	if cp.OCIOptions != nil {
		opts := cp.OCIOptions
		if opts.TenancyOCID == "" || opts.UserOCID == "" || opts.Region == "" || opts.PrivateKeyPath == "" || opts.Fingerprint == "" {
			cp.Logger.Fatal("Missing OCI object store access credentials")
		}
		myenv = append(myenv, fmt.Sprintf("WERCKER_OCI_TENANCY_OCID=%s", opts.TenancyOCID))
		myenv = append(myenv, fmt.Sprintf("WERCKER_OCI_USER_OCID=%s", opts.UserOCID))
		myenv = append(myenv, fmt.Sprintf("WERCKER_OCI_REGION=%s", opts.Region))
		myenv = append(myenv, fmt.Sprintf("WERCKER_OCI_PRIVATE_KEY_PATH=%s", opts.PrivateKeyPath))
		myenv = append(myenv, fmt.Sprintf("WERCKER_OCI_FINGERPRINT=%s", opts.Fingerprint))
		// optional value
		if opts.PrivateKeyPassphrase != "" {
			myenv = append(myenv, fmt.Sprintf("WERCKER_OCI_PRIVATE_KEY_PASSPHRASE=%s", opts.PrivateKeyPassphrase))
		}
	} else if cp.StorePath == "" {
		// This is deprecated and should be change to Oracle cloud eventually
		awskey1 := os.Getenv("AWS_ACCESS_KEY_ID")
		awskey2 := os.Getenv("AWS_SECRET_ACCESS_KEY")
		if awskey1 == "" || awskey2 == "" {
			cp.Logger.Fatal("Missing AWS S3 access credentials")
		}
		awsfullkey1 := fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", awskey1)
		awsfullkey2 := fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", awskey2)
		myenv = append(myenv, awsfullkey1)
		myenv = append(myenv, awsfullkey2)
	}

	if cp.ProdType {
		// Switch for production. Forces external runner to access app.wercker.com
		myenv = append(myenv, "WERCKER_SYSTYPE=PROD")
	}

	// Pickup proxies...
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "http_proxy") || strings.HasPrefix(env, "HTTP_PROXY") ||
			strings.HasPrefix(env, "https_proxy") || strings.HasPrefix(env, "HTTPS_PROXY") ||
			strings.HasPrefix(env, "no_proxy") || strings.HasPrefix(env, "NO_PROXY") {
			// Don't set unless there is really significant data
			tokens := strings.Split(env, "=")
			if len(tokens[1]) > 4 {
				myenv = append(myenv, env)
			}
		}
	}

	// This is a super Kludge until go-dockerclient is updated to support mounts.

	args = append(args, "run")
	args = append(args, "--detach")
	args = append(args, "--name")
	args = append(args, name)
	for _, envvar := range myenv {
		args = append(args, "-e")
		args = append(args, envvar)
	}
	for _, label := range labels {
		args = append(args, "--label")
		args = append(args, label)
	}
	for _, volume := range volumes {
		args = append(args, "--volume")
		args = append(args, volume)
	}
	args = append(args, cp.ImageName)
	// Add the command arguments
	for _, cmdarg := range cmd {
		args = append(args, cmdarg)
	}
	err := runDocker(args)
	if err != nil {
		cp.Logger.Fatal(err)
		return err
	}

	message := fmt.Sprintf("Runner %s has started.", name)
	cp.Logger.Print(message)
	cp.Logger.Debug(fmt.Sprintf("Docker image: %s", cp.ImageName))

	// Remember the container
	// Wait a second because the docker api doesn't set the container id immediately
	time.Sleep(time.Second)
	theDockerContainer, err := cp.client.InspectContainer(name)
	if err != nil {
		cp.Logger.Fatal(err)
	}
	for theDockerContainer == nil {
	}

	newContainer := &runnerContainer{
		containerName:   name,
		containerID:     theDockerContainer.ID,
		containerStatus: theDockerContainer.State.Status,
	}

	cp.containers = append(cp.containers, newContainer)

	return nil
}

// Execute the docker command
func runDocker(args []string) error {
	dockerCmd := exec.Command("docker", args...)
	// run using a pseudo-terminal so that we get the nice docker output :)
	err := dockerCmd.Start()

	if err != nil {
		return err
	}
	return nil
}

// Shutdown all the external runners that have been started for this instance. Each
// container is killed, then waited for it to exit. Then delete the container.
func (cp *RunnerParams) shutdownRunners(runners []*docker.Container) {
	if len(runners) == 0 {
		cp.Logger.Fatal("There are no runners to terminate")
		return
	}

	// For each runner, kill it and wait for it exited before destorying the container.
	for _, dockerContainer := range runners {

		containerName := stripSlashFromName(dockerContainer.Name)
		stats := dockerContainer.State.Status
		// If container is not in a running state then remove it
		if stats != "running" {
			detail := fmt.Sprintf("Inactive runner container %s is removed.", containerName)
			cp.Logger.Print(detail)
			opts := docker.RemoveContainerOptions{
				ID: dockerContainer.ID,
			}
			cp.client.RemoveContainer(opts)
			continue
		}

		err := cp.client.KillContainer(docker.KillContainerOptions{
			ID: dockerContainer.ID,
		})
		if err != nil {
			message := fmt.Sprintf("failed to stop runner container: %s, err=%s", containerName, err)
			cp.Logger.Print(message)
			continue
		}
		// Container was killed, now wait for it to exit.
		for {
			time.Sleep(1000 * time.Millisecond)
			container, err := cp.client.InspectContainer(dockerContainer.ID)

			if err != nil {
				// Assume that an error is because container terminated
				break
			}
			if container.State.Status == "exited" {
				opts := docker.RemoveContainerOptions{
					ID: container.ID,
				}
				cp.client.RemoveContainer(opts)
				message := fmt.Sprintf("Runner %s has terminated.", containerName)
				cp.Logger.Print(message)
				break
			}
		}
	}
	var finalMessage = fmt.Sprintf("Runner(s) for %s stopped.", cp.Basename)
	cp.Logger.Print(finalMessage)
}

// Remove the slash from the beginning of the name
func stripSlashFromName(name string) string {
	return strings.TrimPrefix(name, "/")
}

// Called to wait for all external runners to terminate. While waiting, the logs are accessed and
// either dumped to stdout or written to a specified log file location. If the Wercker command is
// cancelled, whatever runners that are active will continue running.
func (cp *RunnerParams) waitForExternalRunners() {

	// Start the loggers
	for _, p := range cp.containers {
		go cp.logFromContainer(p)
	}

	// Setup interrupt handler
	runnerCleanupHandler := &util.SignalHandler{
		ID: "runner-cleanup",
		F: func() bool {
			cp.Logger.Warnln("Interrupt detected, cleaning up runner containers and shutting down")

			// Kill each container and gracefully terminate
			for _, p := range cp.containers {
				cp.client.KillContainer(docker.KillContainerOptions{
					ID: p.containerID,
				})
				continue
			}
			return true
		},
	}
	util.GlobalSigint().Add(runnerCleanupHandler)
	util.GlobalSigterm().Add(runnerCleanupHandler)

	// Wait until all containers have exited.
	for len(cp.containers) > 0 {

		// Wait an arbitrary amount of time.
		time.Sleep(5 * time.Second)

		for i, rc := range cp.containers {

			// Clear out containers that have exited. Make sure they get
			// removed from our list and from docker.
			dockerContainer, err := cp.client.InspectContainer(rc.containerID)
			if err != nil {
				cp.containers = append(cp.containers[:i], cp.containers[i+1:]...)
				break
			}
			status := dockerContainer.State.Status
			if status == "exited" {
				opts := docker.RemoveContainerOptions{
					ID: dockerContainer.ID,
				}
				cp.client.RemoveContainer(opts)
				message := fmt.Sprintf("Runner %s has been stopped.", rc.containerName)
				cp.Logger.Print(message)
				cp.containers = append(cp.containers[:i], cp.containers[i+1:]...)
				break
			}
		}
	}
}

// Get the log stream for this container and output to either console (defailt) or
// specified logger output path.
func (cp *RunnerParams) logFromContainer(rc *runnerContainer) {

	if cp.LoggerPath != "" {
		os.MkdirAll(cp.LoggerPath, 0666)
	}

	pr, pw := io.Pipe()

	go func() {
		// Read-side of pipe. Get log entries and output to either stdout or
		// append to a log file.
		rd := bufio.NewReader(pr)
		for {
			str, err := rd.ReadString('\n')
			if err != nil {
				log.Print(err)
				return
			}

			// Do any necessary formatting to make str conform to pretty output
			str = strings.TrimSuffix(str, "\n")

			if strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}") {
				// json output so deal appropriately
				ls := logInfo{}
				err = json.Unmarshal([]byte(str), &ls)
				if err == nil {
					str1 := fmt.Sprintf("time=%s level=%s msg=%s", ls.Time, ls.Level, ls.Msg)
					if ls.AgentID != "" {
						str1 = fmt.Sprintf("%s AgentID=%s", str1, ls.AgentID)
					}
					if ls.JobId != "" {
						str1 = fmt.Sprintf("%s JobId=%s", str1, ls.JobId)
					}
					if ls.RunID != "" {
						str1 = fmt.Sprintf("%s RunID=%s", str1, ls.RunID)
					}
					if ls.ProjectID != "" {
						str1 = fmt.Sprintf("%s ProjectID=%s", str1, ls.ProjectID)
					}
					if ls.ProjectOwnerID != "" {
						str1 = fmt.Sprintf("%s ProjectOwnerID=%s", str1, ls.ProjectOwnerID)
					}
					if ls.Source != "" {
						str1 = fmt.Sprintf("%s Source=%s", str1, ls.Source)
					}
					str = str1
				}
			}

			if cp.LoggerPath != "" {
				filename := fmt.Sprintf("%s/%s.log", cp.LoggerPath, rc.containerName)
				f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
				if err == nil {
					f.WriteString(str)
					f.WriteString("\n")
					f.Close()
				}
				continue
			}
			// No output path for logger so just write to stdout
			outline := fmt.Sprintf("%s: %s", rc.containerName, str)
			cp.Logger.Printf(outline)
		}
	}()

	// Setup options to call logger. Follow is set to true so Docker will send
	// log output continuously by writing into a pipe.
	opts := docker.LogsOptions{
		Container:    rc.containerID,
		OutputStream: pw,
		ErrorStream:  pw,
		Stdout:       true,
		Stderr:       true,
		Follow:       true,
	}
	err := cp.client.Logs(opts)
	if err != nil {
		log.Print(err)
	}
}

func checkPathExists(path string) bool {

	if path == "" {
		return true
	}
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}
