#!/bin/bash

# This is intended to be called from wercker/test-all.sh, which sets the required environment variables
# if you run this file directly, you need to set $wercker, $workingDir and $testDir
# as a convenience, if these are not set then assume we're running from the local directory 
if [ -z ${wercker} ]; then wercker=$PWD/../../../wercker; fi
if [ -z ${workingDir} ]; then workingDir=$PWD/../../../.werckertests; mkdir -p "$workingDir"; fi
if [ -z ${testsDir} ]; then testsDir=$PWD/..; fi

testRDD () {
  testName=rdd
  testDir=$testsDir/rdd
  printf "testing %s... " "$testName"
  
  # now run the build pipeline  
  export X_TAG=tag-`date +%s` 
  export X_CONTAINER_NAME=container-`date +%s` 
  $wercker build "$testDir" --docker-local --working-dir "$workingDir" &> "${workingDir}/${testName}.log"
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

testRDDAll () {
  testRDD || return 1 
}

testRDDAll
