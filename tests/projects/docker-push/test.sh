#!/bin/bash

# This is intended to be called from wercker/test-all.sh, which sets the required environment variables
# if you run this file directly, you need to set $wercker, $workingDir and $testDir
# as a convenience, if these are not set then assume we're running from the local directory 
if [ -z ${wercker} ]; then wercker=$PWD/../../../wercker; fi
if [ -z ${workingDir} ]; then workingDir=$PWD/../../../.werckertests; mkdir -p "$workingDir"; fi
if [ -z ${testsDir} ]; then testsDir=$PWD/..; fi

testDockerPush () {
  testName=docker-push
  testDir=$testsDir/docker-push
  printf "testing %s... " "$testName"
  # this test will create an image with the following repository: should match the repository setting in wercker.yml
  repo=my.registry.com/someuser/test-docker-push
  # stop any existing container started by a previous run
  docker kill ${testName}-container > /dev/null 2>&1
  # delete any existing image built by a previous run
  docker images | grep $repo | awk '{print $3}' | xargs -n1 docker rmi -f > /dev/null 2>&1
  # check no existing image with the specified repository (column 1 is the repository)
  docker images | awk '{print $1}' | grep -q "$repo"
  if [ $? -eq 0 ]; then
    echo "An image with repository $repo already exists"
    return 1
  fi
  # now run the build pipeline - this creates an image with the specified repository setting
  $wercker build "$testDir" --docker-local --working-dir "$workingDir" &> "${workingDir}/${testName}.log"
  if [ $? -ne 0 ]; then
    printf "failed\n"
    if [ "${workingDir}/${testName}.log" ]; then
      cat "${workingDir}/${testName}.log"
    fi
    return 1
  fi
  # verify that an image was created with the expected repository setting (column 1 is the repository)
  label1=`docker images | awk '{print $1}' | grep -q "$repo"`
  if [ $? -ne 0 ]; then
    echo "An image with repository $repo was not found"
    return 1
  fi
  # Verify that the labels have been set as specified 
  val1Expected="value1"
  val1Actual=`docker inspect ${repo}:latest -f '{{index .Config.Labels "Label1"}}'`
  if [ $val1Actual != $val1Expected ]; then
    echo "Label incorrect: expected " $val1Expected " but was " $val1Actual
    return 1
  fi 
  val2Expected="value2"
  val2Actual=`docker inspect ${repo}:latest -f '{{index .Config.Labels "Three word key"}}'`
  if [ $val2Actual != $val2Expected ]; then
    echo "Label incorrect: expected " $val2Expected " but was " $val2Actual
    return 1
  fi 
  val3Expected="Three word value"
  val3Actual=`docker inspect ${repo}:latest -f '{{index .Config.Labels "Label3"}}'`
  if [ "$val3Actual" != "$val3Expected" ]; then
    echo "Label incorrect: expected " $val3Expected " but was " $val3Actual
    return 1
  fi  
  # start the image using the docker CLI
  docker run --name ${testName}-container --rm -d -p 5000:5000 ${repo}:latest >> "${workingDir}/${testName}.log" 2>&1
  # test the image
  curlOutput1=`curl -s localhost:5000`         # should return "Hello World!"
  curlOutput2=`curl -s localhost:5000/env/foo` # should return value of build-arg foo (set in wercker.yml)"
  curlOutput3=`curl -s localhost:5000/env/bar` # should return value of build-arg bar (set in wercker.yml)"
  # stop the container
  docker kill ${testName}-container >> "${workingDir}/${testName}.log" 2>&1
  # delete the image we've just created
  docker images | grep $repo | awk '{print $3}' | xargs -n1 docker rmi -f >> "${workingDir}/${testName}.log" 2>&1
  # now the container and image have been cleaned up, check whether the test worked
  if [ "$curlOutput1" != "Hello World!" ]; then
    cat "${workingDir}/${testName}.log"
    echo "Unexpected response from test container for localhost:5000 " $curlOutput1
    return 1
  fi
  if [ "$curlOutput2" != "val1" ]; then
    cat "${workingDir}/${testName}.log"
    echo "Unexpected response from test container for localhost:5000/env/foo " $curlOutput2
    return 1
  fi
  if [ "$curlOutput3" != "Three word value" ]; then
    cat "${workingDir}/${testName}.log"
    echo "Unexpected response from test container for localhost:5000/env/bar " $curlOutput3
    return 1
  fi    
  # test passed
  #cat "${workingDir}/${testName}.log"
  printf "passed\n"
  return 0
}

testDockerPushAll () {
  testDockerPush || return 1 
}

testDockerPushAll
