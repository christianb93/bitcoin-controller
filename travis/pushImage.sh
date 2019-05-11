#!/bin/bash

#
# Push image to Docker hub. This script assumes that there is a file tag
# which contains the name of the tag to use for the push
# We assume that the Docker credentials are provided via the
# environment variables DOCKER_USER and DOCKER_PASSWORD
#

#
# Login to Docker hub
#
echo "$DOCKER_PASSWORD" | docker login --username $DOCKER_USER --password-stdin

#
# Get tag
#
tag=$(cat .$TRAVIS_BUILD_DIR/tag)

#
# Push
#
docker push christianb93/bitcoin-controller:$tag
docker push christianb93/bitcoin-controller:latest
