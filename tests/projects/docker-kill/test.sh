#!/bin/bash

# This is intended to be called from wercker/test-all.sh, which sets the required environment variables
# if you run this file directly, you need to set $wercker, $workingDir and $testDir
# as a convenience, if these are not set then assume we're running from the local directory 
if [ -z ${wercker} ]; then wercker=$PWD/../../../wercker; fi
if [ -z ${workingDir} ]; then workingDir=$PWD/../../../.werckertests; mkdir -p "$workingDir"; fi
if [ -z ${testsDir} ]; then testsDir=$PWD/..; fi

testDockerKill () {
  echo -n "testing docker-kill.."
  testDir=$testsDir/docker-kill
  logFile="${workingDir}/docker-kill.log"
  $wercker build "$testDir" --docker-local --working-dir "$workingDir" &> "$logFile"
  if [ $? -eq 0 ]; then
    cName=`docker ps -a | grep myTestContainer | awk '{print $NF}'`
    if [ ! "$cName" ]; then
      echo "passed"
      return 0
    else
      echo 'failed'
      cat "$logFile"
      docker images
      return 1
    fi
  else
    echo 'failed'
    cat "$logFile"
    docker images
    return 1
  fi
}

testDockerKillAll() {
  testDockerKill || return 1 
}

testDockerKillAll
