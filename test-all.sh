#!/bin/bash

# This is a shell script to run a bunch of regression tests that require
# running sentcli in a fully docker-enabled environment. They'll eventually
# be moved into a golang test package.
# 
# These tests use the --docker-local parameter, which means that if they need an image
# that is not already in the docker daemon these tests will fail with "image not found"
# The function pullImages below pulls a list of specified images before running the tests. Update it if needed.
#
# This script can be run either from the command line or in a Wercker pipeline.
# Those tests that cannot be run in a Wercker pipeline are skipped automatically.
#
# Before running these tests in a Wercker pipeline the pipeline must start a docker daemon running in the pipeline container, 
# and set DOCKER_HOST accordingly. This means the pipeline must have docker: true specified because that causes the pipeline 
# container to run in privileged mode, and a docker container is not allowed to start a docker daemon unless the container 
# is started in privileged mode.
#
# Note that you can't simply set docker:true (to enable direct docker access) and use the default setting for DOCKER_HOST,
# which is to point to the daemon in which the pipeline container is running. This is because these tests use the Wercker CLI,
# which will start containers in the daemon specified by DOCKER_HOST, and Wercker CLI requires that any such containers 
# are able access the same file system that the Wercker CLI uses.
# 
# To run the tests:
#
#  cd $GOPATH//src/github.com/wercker/wercker
#  ./test-all.sh
#
wercker=$PWD/wercker
workingDir=$PWD/.werckertests
testsDir=$PWD/tests/projects
rootDir=$PWD

# Make sure we have a working directory
mkdir -p "$workingDir"
if [ ! -e "$wercker" ]; then
  go build
fi

pullIfNeeded () {
  ## check whether an image exists locally with the specified repository
  ## TODO extend to allow a tag to be specified 
  docker images | awk '{print $1}' | grep -q $1
  if [ $? -ne 0 ]; then
    echo pulling $1
    docker pull $1
  fi
}

# Since most tests run with the --docker-local parameter we need to make sure that the required base images are pulled into the daemon
pullImages () {
  pullIfNeeded "busybox"
  pullIfNeeded "node"
  pullIfNeeded "alpine"
  pullIfNeeded "ubuntu"
  pullIfNeeded "golang"
  pullIfNeeded "postgres:9.6"
  pullIfNeeded "nginx"
  pullIfNeeded "interactivesolutions/eatmydata-mysql-server"
}

basicTest() {
  testName=$1
  shift
  printf "testing %s... " "$testName"
  $wercker --debug $@ --working-dir "$workingDir" &> "${workingDir}/${testName}.log"
  if [ $? -ne 0 ]; then
    printf "failed\n"
    cat "${workingDir}/${testName}.log"
    return 1
  else
    printf "passed\n"
  fi
  return 0
}

basicTestFail() {
  testName=$1
  shift
  printf "testing %s... " "$testName"
  $wercker $@ --working-dir "$workingDir" &> "${workingDir}/${testName}.log"
  if [ $? -ne 1 ]; then
    printf "failed\n"
    cat "${workingDir}/${testName}.log"
    return 1
  else
    printf "passed\n"
  fi
  return 0
}

testScratchPush () {
  echo -n "testing scratch-n-push.."
  testDir=$testsDir/scratch-n-push
  logFile="${workingDir}/scratch-n-push.log"
  grepString="uniqueTagFromTest"
  docker images | grep $grepString | awk '{print $3}' | xargs -n1 docker rmi -f > /dev/null 2>&1
  $wercker build "$testDir" --docker-local --working-dir "$workingDir" &> "$logFile" && docker images | grep -q "$grepString"
  if [ $? -eq 0 ]; then
    echo "passed"
    return 0
  else
      echo 'failed'
      cat "$logFile"
      docker images
      return 1
  fi
}


runTests1() {
  # Since most tests run with the --docker-local parameter we need to make sure that the required base images are pulled into the daemon
  pullIfNeeded "alpine"
  pullIfNeeded "ubuntu"
  pullIfNeeded "postgres:9.6"
  pullIfNeeded "nginx"
  pullIfNeeded "interactivesolutions/eatmydata-mysql-server"  

  #  The following tests must be skipped when run in a wercker pipeline 
  if [ -z ${WERCKER_ROOT} ]; then 
    # local services test cannot be run in wercker because the pipeline cannot connect to the local service (despite being in the same network)
    basicTest "local services"    build "$testsDir/local-service/service-consumer" --docker-local || return 1
    # The shellstep test cannot be run in wercker as the lack of a terminal causes it to fail with "invalid ioctl"
    basicTest "shellstep" build --docker-local --enable-dev-steps "$testsDir/shellstep" || return 1
  fi

  source $testsDir/rdd/test.sh || return 1
  source $testsDir/rdd-volumes/test.sh || return 1
  source $testsDir/privileged/test.sh || return 1
  source $testsDir/enable-volumes/test.sh || return 1
  source $testsDir/direct-mount-test/test.sh || return 1
  source $testsDir/docker-push/test.sh || return 1
  source $testsDir/docker-build/test.sh || return 1
  source $testsDir/docker-push-image/test.sh || return 1
  source $testsDir/docker-networks/test.sh || return 1
  source $testsDir/docker-kill/test.sh || return 1

  export X_TEST_SERVICE_VOL_PATH=$testsDir/test-service-vol
  basicTest "docker run" build "$testsDir/docker-run" --docker-local || return 1

  basicTest "source-path"       build "$testsDir/source-path" --docker-local || return 1
  # The source-path test messes up subsequent tests, so clean out its working directory
  rm -rf "${workingDir}"; mkdir -p "$workingDir"

  basicTest "rm pipeline --artifacts" build "$testsDir/rm-pipeline" --docker-local --artifacts  || return 1
  basicTest "rm pipeline"       build "$testsDir/rm-pipeline" --docker-local || return 1
  basicTest "deploy"            deploy "$testsDir/deploy-no-targets" --docker-local || return 1
  basicTest "deploy target"     deploy "$testsDir/deploy-targets" --docker-local  --deploy-target test || return 1
  basicTest "after steps"       build "$testsDir/after-steps-fail" --docker-local --pipeline build_true  || return 1
  basicTest "relative symlinks" build "$testsDir/relative-symlinks" --docker-local || return 1

  export X_BACKSLASH='back\slash'
  export X_BACKTICK='back`tick'
  basicTest "special char in envvar escaped" build "$testsDir/envvars" --docker-local --pipeline test-special || return 1

  # test different shells
  basicTest "bash_or_sh alpine"   build "$testsDir/bash_or_sh" --docker-local --pipeline test-alpine  || return 1
  basicTest "bash_or_sh busybox"  build "$testsDir/bash_or_sh" --docker-local --pipeline test-busybox || return 1
  basicTest "bash_or_sh ubuntu"   build "$testsDir/bash_or_sh" --docker-local --pipeline test-ubuntu || return 1

  # test for a specific bug around failures
  basicTestFail "bash_or_sh alpine failures" --no-colors build "$testsDir/bash_or_sh" --docker-local --pipeline test-alpine-fail || return 1
  grep -q "second fail" "${workingDir}/bash_or_sh alpine failures.log" && echo "^^ failed" && return 1
  basicTestFail "bash_or_sh ubuntu failures" --no-colors build "$testsDir/bash_or_sh" --docker-local --pipeline test-ubuntu-fail || return 1
  grep -q "second fail" "${workingDir}/bash_or_sh ubuntu failures.log" && echo "^^ failed" && return 1

  # this one will fail but we'll grep the log for After-step passed: test
  basicTestFail "after steps fail" --no-colors build "$testsDir/after-steps-fail" --docker-local --pipeline build_fail  || return 1
  grep -q "After-step passed: test" "${workingDir}/after steps fail.log" || return 1

  # make sure we get some human understandable output if the wercker file is wrong
  basicTestFail "empty wercker file" build "$testsDir/invalid-config" --docker-local || return 1
  grep -q "Your wercker.yml is empty." "${workingDir}/empty wercker file.log" || return 1

 }

runTests2() {
  # Since most tests run with the --docker-local parameter we need to make sure that the required base images are pulled into the daemon
  pullIfNeeded "alpine"
  pullIfNeeded "alpine:3.8"
  pullIfNeeded "golang"

  basicTest "multiple services with the same image" build "$testsDir/multidb" || return 1

  testScratchPush || return 1

   # The following test fails if we don't first clean out the working directory
  rm -rf "${workingDir}"; mkdir -p "$workingDir"

  # make sure the build successfully completes when cache is too big
  basicTest "cache size too big" build "$testsDir/cache-size" --docker-local || return 1  

  # The following test fails if we don't first clean out the working directory
  rm -rf "${workingDir}"; mkdir -p "$workingDir"

  # make sure the build fails when an artifact is too big
  basicTestFail "artifact size too big" build "$testsDir/artifact-size" --docker-local --artifacts || return 1
  grep -q "Storing artifacts failed: Size exceeds maximum size of 5000MB" "${workingDir}/artifact size too big.log" || return 1

  basicTest "artifact empty file" build "$testsDir/artifact-empty-file" --docker-local --artifacts || return 1

  # test deploy behavior with different levels of specificity
  cd "$testsDir/local-deploy/latest-no-yml"
  basicTest "local deploy using latest build not containing wercker.yml" deploy --docker-local || return 1
  cd "$testsDir/local-deploy/latest-no-yml"
  basicTest "local build setup for local deploy tests" build --docker-local --pipeline deploy --artifacts || return 1
  cd "$testsDir/local-deploy/latest-yml"
  basicTest "local deploy using latest build containing wercker.yml" deploy --docker-local || return 1
  cd "$testsDir/local-deploy/specific-no-yml"
  basicTest "local deploy using specific build not containing wercker.yml" deploy --docker-local ./last_build || return 1
  cd "$testsDir/local-deploy/specific-yml"
  basicTest "local deploy using specific build containing wercker.yml" deploy --docker-local ./last_build || return 1

  cd "$rootDir"

  # test checkpointers
  basicTest "checkpoint, part 1"      build "$testsDir/checkpoint" --docker-local --enable-dev-steps || return 1
  basicTestFail "checkpoint, part 2"  build "$testsDir/checkpoint" --docker-local --enable-dev-steps=false --checkpoint foo || return 1
  basicTest "checkpoint, part 3"      build "$testsDir/checkpoint" --docker-local --enable-dev-steps --checkpoint foo || return 1

  # fetching and pushing
  if [ -n "$TEST_PUSH" ]; then
    basicTest "fetch from amazon"         build "$testsDir/amzn-test" || return 1
    basicTest "fetch from docker hub"     build "$testsDir/docker-hub-test" || return 1
    basicTest "fetch from gcr"            build "$testsDir/gcr-test" || return 1
    basicTest "fetch from docker hub v1"  build "$testsDir/reg-v1-test" || return 1
  fi

  source $testsDir/sync-env-alpine/test.sh || return 1
}

# The tests in this file are divided into two, runTest1 and runTest2,
# which can be run in parallel pipelines
# This script takes one optional argument, which must be runTest1 or runTest2
if [ -z $1 ]; then
  # no parameter supplied - run all tests
  runTests1 || exit 1
  runTests2 || exit 1
else
  # parameter supplied - run the specified set of tests
  $1 || exit 1
fi
rm -rf "$workingDir"
