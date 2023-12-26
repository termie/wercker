#!/bin/bash

# This is intended to be called from wercker/test-all.sh, which sets the required environment variables
# if you run this file directly, you need to set $wercker, $workingDir and $testDir
# as a convenience, if these are not set then assume we're running from the local directory 
if [ -z ${wercker} ]; then wercker=$PWD/../../../wercker; fi
if [ -z ${workingDir} ]; then workingDir=$PWD/../../../.werckertests; mkdir -p "$workingDir"; fi
if [ -z ${testsDir} ]; then testsDir=$PWD/..; fi

# Test the use of bind mounts with a RDD
testRDDBindMounts () {
  testName=rdd-bind-mounts
  testDir=$testsDir/rdd-volumes
  printf "testing %s... " "$testName"
  
  # now run the build pipeline  
  export X_ID=`date +%s` 
  $wercker build "$testDir" --pipeline rdd-bind-mounts --docker-local --working-dir "$workingDir" &> "${workingDir}/${testName}.log"
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

 # Test the use of volumes with a RDD
 testRDDVolumes () {
  testName=rdd-volumes
  testDir=$testsDir/rdd-volumes
  printf "testing %s... " "$testName"
  
  # now run the build pipeline  
  export X_ID=`date +%s` 
  $wercker build "$testDir" --pipeline test-rdd-volumes --docker-local --working-dir "$workingDir" &> "${workingDir}/${testName}.log"
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

testRDDBindMountsAll () {
  testRDDBindMounts || return 1 
  testRDDVolumes || return 1 
}

testRDDBindMountsAll
