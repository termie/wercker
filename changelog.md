## unreleased

## 1.0.1555 (2019-04-05)

- Revert #566

## 1.0.1554 (2019-04-04)

- Allow wercker CLI to be run locally with a web proxy set (#566) 

## 1.0.1546 (2019-03-27)

- Fix bug in local steps (#564)

## 1.0.1539 (2019-03-20)

- Add ability to reference environment files from workflows (#562)

## 1.0.1530 (2019-03-11)

- Turn on enable-dev-steps flag by default (#560)
- Make wercker-init to be an embedded step (#558)
- Fix bug which meant that when a run was aborted the step was sometimes still shown as running (#559)

## v1.0.1467 (2019-01-07)

- Gracefully handle failure when docker-push has an invalid label specification (#553)

## v1.0.1442 (2018-12-13)

- Add retry mechanism for fetching step version and tarball (#550)

## v1.0.1436 (2018-12-07)

- Improve error message when a step cannot be fetched (#545)

## v1.0.1435 (2018-12-06)

- Fix interpolation and splitting order of tags for internal/docker-push (#546)

## v1.0.1432 (2018-12-03)

- Fix for gcr push to encode newlines correctly (#542)

## v1.0.1429 (2018-11-30)

- Add new property "context" to internal/docker-build (#537)
- Switch logs to json format(#535)

## v1.0.1421 (2018-11-22)

- Fix problems with OCI environment settings passed to kiddie-pool for runners.
- Issue error message when store-oci and storepath are both specified in the wercker command
- Fix issue with docker:true and after-steps (#538)

## v1.0.1401 (2018-11-02)

- Fix SyncEnvironment for alpine images (#531)

## v1.0.1399 (2018-10-31)

- Remove redundant flag --allow-rdd (#530)

## v1.0.1393 (2018-10-25)

- Make direct docker access available to everyone (#528)

## v1.0.1380 (2018-10-12)

- suppress build logs(#522)

## v1.0.1378 (2018-10-10)

- updated image in integration test(#524)
- Created new env var WERCKER_GIT_TAG (#523)

## v1.0.1359 (2018-09-21)

- Fix env vars WERCKER_DEPLOY_URL, WERCKER_BUILD_URL and WERCKER_RUN_URL (#519)

## v1.0.1355 (2018-09-17)

- Inject checkout pipeline for workflows in yml (#517)

## v1.0.1351 (2018-09-13)

- Show error when publishing a step fails (#515)

## v1.0.1350 (2018-09-12)

- Validate tag format in internal/docker-push and internal/docker-scratch-push (#513)

## v1.0.1345 (2018-09-07)

- Add workflows validation to check-config (#511)

## v1.0.1343 (2018-09-05)

- Fix for workflows in yml (#507)
- Escape special characters in bash exports (#499)
- Fix option check for runner start to allow a server-side group name (#504)

## v1.0.1337 (2018-08-30)

- Display on screen repository:tag being pushed with internal/docker-push (#500)
- When docker:true is set, run the pipeline container in privileged mode (#480)

## v1.0.1336 (2018-08-29)

- Workflow validation (#497)

## v1.0.1335 (2018-08-28)

- Increase value of retry counter for CreateContainer call (#495)

## v1.0.1334 (2018-08-27)

- Add support for runners to use OCI object storage (#492)
- Fix runner configure access to OCIR caused by OCI API change. Show runner docker image status on runner start (#493)

## v1.0.1328 (2018-08-21)

- Retry CreateContainer to avoid intermittent no such image issue (#479)

## v1.0.1327 (2018-08-20)

- Display message to users while pushing images built using internal/docker-build (#481)

## v1.0.1323 (2018-08-16)

- Wrap errors in runner (#487)

## v1.0.1321 (2018-08-14)

- Add OCI object store support (#465)
- Add InternalBuildFlags to workflow command (#478)

## v1.0.1316 (2018-08-09)

- Experimental support for running workflows in yml locally (#471)

## v1.0.1315 (2018-08-08)

- Do not crash if the requested pipeline is not defined in the yaml file (#473)

## v1.0.1313  (2018-08-06)

- internal/docker-run should fail pileline if image does not exist (#468)

## v1.0.1309 (2018-08-02)

- Better error message when failing to create docker conatainer (#469)

## v1.0.1308 (2018-08-01)

- Clean the docker box option when cleaning the network (#462)
- RDD Cleanup for errors/aborts (#461)
- Introduced access control for RDD (#463) 

## v1.0.1302 (2018-07-26)

- Fix panic when no docker daemon is running (#457)
- Remove unused journal logging (#431)
- Authentication added for docker build (#449)
- Authentication added for internal/docker-run step (#451)

## v1.0.1301 (2018-07-25)

- Wrap error messages for docker failure triage (pull 454)
- Remove external from runner messages
- Support server side configuration group name <name@group-name>
- Fix runner help to only show related options
- Add --using=prod|dev to switch between app/dev servers
- Update runner configure to get latest runner image when production. 
- Add runner image override for development use

## v1.0.1296 (2018-07-20)

- Add support for direct docker daemon access (#442)

## v1.0.1281 (2018-07-05)

- Send auth token with steps request to allow access to private steps (#445)
- Execute docker-build in the same network as the normal build (#446)

## v1.0.1271 (2018-06-25)

- Honor proxy environment variables, if set (#440)
- Do not manually check for write access to remote docker repository, since docker does it anyway (#436)

## v1.0.1267 (2018-06-21)

- docker-push no longer defaults to wcr.io, and displays info messages in certain cases (#423)
- docker-build bugfix: if the dockerfile build fails then the error is displayed and the step fails (#424)
- Add retry mechanism to RDD verify method (#434)
- Fix wercker --help for subcommands (#432)

## v1.0.1264 (2018-06-18)

- Collect report artifacts from a step even if it failed (#428)

## v1.0.1260 (2018-06-14)

- Changes to access RDD API Service and inject RDD in build pipelines (#421)
- Fix wercker --help (#426)

## v1.0.1244 (2018-05-29)

- Changes for WRKR-347 Allow switching between app/dev sites (#419)

## v1.0.1238 (2018-05-23)

- Wercker runner config (#417)
- Docker file integration (#415)

## v1.0.1230 (2018-05-15)

- Support for publishing private steps (#409)
- Wercker CLI changes for external runner (#406)
    Changes for WRKR-76 and WRKR-207
    Adding code to format json log entries, write log to disk
    Clean up logging to disk
    Cleanup logging when JobId encountered.
    Fix --nowait option not working
    Fix to use proper logger
    Allow store path as env variable WERCKER_RUNNER_STOREPATH. Make sure directory is created if doesn't exist
    Add informational message telling user where local output is stored.
    Runner configure without remote repository pull
    * Changes for WRKR-76, WRKR-77, and WRKR-207
    * Fix typo in flags definition

## v1.0.1226 (2018-05-11)

- Reverted Cosmetic changes for docker file integration (#407)
- Reverted Docker file integration, docker-networks, docker-run, docker-kill (#405)

## v1.0.1223 (2018-05-08)

- External runner changes removed by prior revert (merge pull request #384) 
- Cosmetic changes for docker file integration (#407)
- Docker file integration, docker-networks, docker-run, docker-kill (#405)

## v1.0.1216 (2018-05-01)

- Add Oracle Contributor Agreement (#400)
- Convert docker-push to use the official docker client (#399)

## v1.0.1210 (2018-04-25)

- Revert docker links to docker networks change, which was causing build issues (#397)

## v1.0.1205 (2018-04-20)

- Changes for robust error handling and reporting in docker-push (#387)
- Replace docker links with docker network (#382)
- Change some docker API calls to use the official Docker client (#385) 

## v1.0.1201 (2018-04-16)

- Update azure client to allow docker-push in all regions (#381)

## v1.0.1196(2018-04-11)

- Fixes and additional properties for internal/docker-build step (#372)

## v1.0.1195(2018-04-10)

- Fix for correctly inferring regsitry and repoistory from step inputs (#375) 
- Fix "go build" and "wercker build" on golang 1.10 (#374)

## v1.0.1189(2018-04-04)

- Fix status reporting for docker push (#371)

## v1.0.1183 (2018-03-29)

- New docker-build step and enhanded docker-push step (#362)

## v1.0.1153 (2018-02-27)

- Remove Keen dependencies (#354)

## v1.0.1062 (2017-11-28)

- Default docker hub push to registry V2 (#348)

## v1.0.1049 (2017-11-15)

- Update dependencies, as a result of `Sirupsen/logrus` -> `sirupsen/logrus` (#333)
- Add a Docker subcommand (#335)
- Ensure repository names are always lowercase (#338)
- Support for the new step manifest format (#343)

## v1.0.965 (2017-08-23)

- Change compilation in separate wercker steps (#331)
- Add retry and exponential backoff for fetching step metadata and step tarball
  (#330)
- Add flag to delete Docker image after pushing it to a registry (#327)
- Use wercker registry for wercker-init (#334)

## v1.0.938 (2017-07-27)

- Some nice additions to the way we do the docker push and test (#320)
- Fix env var loading order (#314, #315, #317)
- Fix internal/watchstep (#312)
- Add env option to docker-scratch-push (#295)
- Allow relative paths for file:// targets in dev mode (#296)
- Better control limiting memory on run containers, when using
  services gives the services a 25% of the total memory to split
  amongst themselves, defaults to no limits (#299)
- Automatically detect bash or sh for containers by default,
  defaulting to bash if it is there (#301)
- Fix a small bug when doing local deploys and using a working-dir other
  than .wercker (#301)

## v1.0.758 (2017-01-27)

- Add Azure Registry support (#275)
- Explicitly chmods the basepath / source path to a+rx
- Removes the explicit clear after launching a shell (#257)
- Fix `wercker doc` and update `./Documentation/*` (#260)

## v1.0.643 (2016-10-05)

- Remove google as default container DNS (#245)
- Update to compiling with go 1.7

## v1.0.629 (2016-09-21)

- Add additional output when storing artifacts (#207)
- Fix longer (2+) chains of runs that have source-dir specified (#151)
- Output more descriptive error message when setup environment fails (#230)
- Allow use of an "ignore-file" yaml directive that parse the gitignore syntax
  (#240)

## v1.0.560 (2016-07-14)

- Fix internal/docker-scratch-push for Docker 1.10+

## v1.0.547 (2016-07-01)

- Add checkpointing and base-path (#123)
- Support for registry v2 (#131)
- Mount volumes in the container from different local paths (#134)
- Only push tags that were defined in the wercker.yml (#142)
- wercker is now using govendor (#146)
- Display raw config, before parsing it (#149)
- Allow multiple services with the same images (#159)
- Add exposed-ports (#161)
- Fix run, build and deploy urls (#163)

## 2016.03.11

### Features

- Moves the working path to default to `.wercker` and removes the flags
  for configuring the other paths
- Adds a symlink `.wercker/latest` for referring to your latest build, and
  a `.wercker/latest_deploy` for referring to your latest deploy
- Make the --artifacts work better locally, making your build's artifacts
  easily available under .wercker/latest/output
- Automatically use the contents of `.wercker/latest/output` when running a
  `wercker deploy` without specifying a target
- When running `wercker deploy` if the specified target does not container a
  wercker.yml file, attempt to use the one in the current directory.
- Allow settings multiple tags at a time when doing `internal/docker-push`
- Check for and allow unix:///var/run/docker.sock on non-linux hosts


### Bug Fixes

- Deal with symlinks significantly better
- Respect --docker-local when using `internal/docker-push` (don't push)
- Allow images to be pulled by nested local services (removes
  implicit --docker-local)
- Workaround a docker issue related to not fully consuming the result of a
  CopyFromContainer API call (when we exported a cache that was more than our
  limit of 1GB we'd just drop it, and docker would hang)
- Remove pipeline ID tag set by `internal/docker-push`


## 2016.02.10

### Features

- Allow users to mount local volumes to their wercker build containers, specified by a list of `volumes` underneath box in the werker.yml file. Must have `--enable-volumes` flag set in order to run.
- Check to see if config from wercker.yml is empty
- Adds changelog

### Bug fixes

- Fixes to the shellstep implementation
