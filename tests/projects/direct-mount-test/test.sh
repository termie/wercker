#!/bin/bash

# This is intended to be called from wercker/test-all.sh, which sets the required environment variables
# if you run this file directly, you need to set $wercker, $workingDir and $testDir
# as a convenience, if these are not set then assume we're running from the local directory 
if [ -z ${wercker} ]; then wercker=$PWD/../../../wercker; fi
if [ -z ${workingDir} ]; then workingDir=$PWD/../../../.werckertests; mkdir -p "$workingDir"; fi
if [ -z ${testsDir} ]; then testsDir=$PWD/..; fi

# Test direct-mount in the non-RDD case by running the test-direct-mount-normal pipeline
testDirectMountNormal () {
  testName=direct-mount-test-normal
  testDir=$testsDir/direct-mount-test
  printf "testing %s... " "$testName"
  
  testFile=${testDir}/testfile-normal
  > "$testFile"
  echo "hello" > "$testFile"
  logFile="${workingDir}/direct-mount.log"
  $wercker build "$testDir" --pipeline test-direct-mount-normal --direct-mount --docker-local --working-dir "$workingDir" &> "$logFile"
  expected="normal"
  contents=$(cat "$testFile")
  if [ "$contents" == "$expected" ]
      then echo "passed"
      rm $testFile 2> /dev/null
      return 0
  else
      echo 'failed'
      echo expected $expected got $contents
      cat "$logFile"
      return 1
  fi

  printf "passed\n"
  return 0
}

# Test direct-mount in the RDD case by running the test-direct-mount-rdd pipeline
testDirectMountRDD () {
  testName=direct-mount-test-rdd
  testDir=$testsDir/direct-mount-test
  printf "testing %s... " "$testName"

  testFile=${testDir}/testfile-rdd
  > "$testFile"
  echo "hello" > "$testFile"
  logFile="${workingDir}/direct-mount.log"
  $wercker build "$testDir" --pipeline test-direct-mount-rdd --direct-mount --docker-local --working-dir "$workingDir" &> "$logFile"
  expected="rdd"
  contents=$(cat "$testFile")
  if [ "$contents" == "$expected" ]
      then echo "passed"
      rm $testFile 2> /dev/null
      return 0
  else
      echo 'failed'
      echo expected $expected got $contents
      cat "$logFile"
      return 1
  fi
  
  printf "passed\n"
  return 0
}

testDirectMountAll () {
  testDirectMountNormal || return 1 
  testDirectMountRDD || return 1 
}

testDirectMountAll 
