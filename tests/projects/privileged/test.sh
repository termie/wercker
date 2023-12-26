#!/bin/bash

# This is intended to be called from wercker/test-all.sh, which sets the required environment variables
# if you run this file directly, you need to set $wercker, $workingDir and $testDir
# as a convenience, if these are not set then assume we're running from the local directory 
if [ -z ${wercker} ]; then wercker=$PWD/../../../wercker; fi
if [ -z ${workingDir} ]; then workingDir=$PWD/../../../.werckertests; mkdir -p "$workingDir"; fi
if [ -z ${testsDir} ]; then testsDir=$PWD/..; fi

# Test that if docker:true is not specified then the pipeline container is NOT run in privileged mode
testPrivilegedModeDockerFalse () {
  testName=privileged-mode-docker-false
  testDir=$testsDir/privileged
  printf "testing %s... " "$testName"
  
  logFile="${workingDir}/$testName.log"
  $wercker build "$testDir" --pipeline privileged-mode-docker-false --working-dir "$workingDir" &> "$logFile"
  if [ $? -ne 0 ]; then
    printf "failed\n"
    if [ "${workingDir}/${testName}.log" ]; then
      cat "${workingDir}/${testName}.log"
    fi
    return 1
  fi  
  printf "passed\n"
  return 0
}

# Test that if docker:true is not specified then the pipeline container is run in privileged mode
testPrivilegedModeDockerTrue () {
  testName=privileged-mode-docker-true
  testDir=$testsDir/privileged
  printf "testing %s... " "$testName"

  logFile="${workingDir}/$testName.log"
  $wercker build "$testDir" --pipeline privileged-mode-docker-true --working-dir "$workingDir" &> "$logFile"
  if [ $? -ne 0 ]; then
    printf "failed\n"
    if [ "${workingDir}/${testName}.log" ]; then
      cat "${workingDir}/${testName}.log"
    fi
    return 1
  fi  
  printf "passed\n"
  return 0
}

testPrivilegedModeAll () {
  testPrivilegedModeDockerFalse || return 1 
  testPrivilegedModeDockerTrue || return 1 
}

testPrivilegedModeAll 
