#!/bin/bash

# This is intended to be called from wercker/test-all.sh, which sets the required environment variables
# if you run this file directly, you need to set $wercker, $workingDir and $testDir
# as a convenience, if these are not set then assume we're running from the local directory 
if [ -z ${wercker} ]; then wercker=$PWD/../../../wercker; fi
if [ -z ${workingDir} ]; then workingDir=$PWD/../../../.werckertests; mkdir -p "$workingDir"; fi
if [ -z ${testsDir} ]; then testsDir=$PWD/..; fi

# Test enable-volumes in the non-RDD case by running the test-enable-volumes-normal pipeline
testEnableVolumesNormal () {
  testName=enable-volumes-normal
  testDir=$testsDir/enable-volumes
  printf "testing %s... " "$testName"

  logFile="${workingDir}/${testName}.log"

  export X_BOX_VOL_PATH=$testDir/volume-normal
  export X_SVC_VOL_PATH=$testDir/volume-normal
  rm -r $X_BOX_VOL_PATH  2> /dev/null
  mkdir $X_BOX_VOL_PATH 

  testFileBox=$X_BOX_VOL_PATH/testfile-box-normal
  > "$testFileBox"
  echo "NoNoNo" > "$testFileBox"

  testFileSvc=$X_SVC_VOL_PATH/testfile-svc-normal
  > "$testFileSvc"
  echo "NoNoNo" > "$testFileSvc"

  $wercker build "$testDir" --pipeline test-enable-volumes-normal --enable-volumes --docker-local --working-dir "$workingDir" &> "$logFile"

  # Verify the *script step* has created the expected file in the expected place
  expected="Wombat"
  contents=$(cat "$testFileBox")
  if [ "$contents" != "$expected" ]
  then
      echo 'failed'
      echo expected $expected got $contents
      cat "$logFile"
      return 1
  fi
  # Verify the *service* has created the expected file in the expected place
  expected="Unicorn"
  contents=$(cat "$testFileSvc")
  if [ "$contents" != "$expected" ]
  then
      echo 'failed'
      echo expected $expected got $contents
      cat "$logFile"
      return 1
  fi 

  rm -r $X_BOX_VOL_PATH  2> /dev/null
  printf "passed\n"
  return 0
}

# Test enable-volumes in the RDD case by running the test-enable-volumes-rdd pipeline
testEnableVolumesRDD () {
  testName=enable-volumes-rdd
  testDir=$testsDir/enable-volumes
  printf "testing %s... " "$testName"

  logFile="${workingDir}/${testName}.log"

  export X_BOX_VOL_PATH=$testDir/volume-rdd
  export X_SVC_VOL_PATH=$testDir/volume-rdd
  rm -r $X_BOX_VOL_PATH  2> /dev/null
  mkdir $X_BOX_VOL_PATH 

  testFileBox=$X_BOX_VOL_PATH/testfile-box-rdd
  > "$testFileBox"
  echo "NoNoNo" > "$testFileBox"

  testFileSvc=$X_SVC_VOL_PATH/testfile-svc-rdd
  > "$testFileSvc"
  echo "NoNoNo" > "$testFileSvc"

  $wercker build "$testDir" --pipeline test-enable-volumes-rdd --enable-volumes --docker-local --working-dir "$workingDir" &> "$logFile"

  # Verify the *script step* has created the expected file in the expected place
  expected="Antelope"
  contents=$(cat "$testFileBox")
  if [ "$contents" != "$expected" ]
  then
      echo 'failed'
      echo expected $expected got $contents
      cat "$logFile"
      return 1
  fi
  # Verify the *service* has created the expected file in the expected place
  expected="Giraffe"
  contents=$(cat "$testFileSvc")
  if [ "$contents" != "$expected" ]
  then 
      echo 'failed'
      echo expected $expected got $contents
      cat "$logFile"
      return 1
  fi 

  printf "passed\n"
  return 0
}

testEnableVolumesAll () {
  testEnableVolumesNormal || return 1 
  testEnableVolumesRDD || return 1 
}

testEnableVolumesAll || exit 1 
