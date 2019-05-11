#!/bin/bash

#
# This script builds a Docker image for the controller
# and pushes it to the local Docker repository and to the
# Docker Hub.
# It assumes that the following environment variables are set
# TRAVIS_TAG - if the current build is for a tag push, this should be the tag name, otherwise
#              this is assumed to be empty
# TRAVIS_BUILD_DIR - the absolute path name of the cloned repository
# DCOKER_PASSWORD - the password for the Docker hub
# DOCKER_USER - the username for the Docker hub

# Fail if a line fails
set -e

#
# Get short form of git hash for current commit
#
hash=$(git log --pretty=format:'%h' -n 1)
#
# Determine tag. If the build is from a tag push, use tag name, otherwise
# use commit hash
#
if [ "X$TRAVIS_TAG" == "X" ]; then
  tag=$hash
else
  tag=$TRAVIS_TAG
fi

#
# Log in to Docker Hub
#
echo "$DOCKER_PASSWORD" | docker login --username $DOCKER_USER --password-stdin

#
# Create image locally
#
cd $TRAVIS_BUILD_DIR/cmd/controller
CGO_ENABLED=0 go build
docker build --rm -f $TRAVIS_BUILD_DIR/build/controller/Dockerfile -t christianb93/bitcoin-controller:$tag .
docker push christianb93/bitcoin-controller:$tag

#
# Return tag
#
echo $tag
