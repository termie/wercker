#!/bin/bash

# This is intended to be called from wercker/test-all.sh, which sets the required environment variables
# if you run this file directly, you need to set $wercker, $workingDir and $testDir
# as a convenience, if these are not set then assume we're running from the local directory 
if [ -z ${wercker} ]; then wercker=$PWD/../../../wercker; fi
if [ -z ${workingDir} ]; then workingDir=$PWD/../../../.werckertests; mkdir -p "$workingDir"; fi
if [ -z ${testsDir} ]; then testsDir=$PWD/..; mkdir -p "$testsDir"; fi

testSyncEnv () {
  testName=sync-env-alpine
  testDir=$testsDir/sync-env-alpine
  printf "testing %s... " "$testName"
  # now run the build pipeline 
  $wercker --environment "$testDir/ENVIRONMENT" build "$testDir" --working-dir "$workingDir" --docker-local &> "${workingDir}/${testName}.log"
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

testSyncEnv
