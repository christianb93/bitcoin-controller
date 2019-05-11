#!/bin/bash

#
# Push images to Docker hub and update Helm chart repository
#
# This script assumes that there is a file tag
# which contains the name of the tag to use for the push
#
# We assume that the following environment variables are set
# DOCKER_USER               User for docker hub
# DOCKER_PASSWORD           Password
# TRAVIS_BUILD_DIR          Travis build directory
# TRAVIS_TAG                Tag if the build is caused by a git tag
#

#
# Login to Docker hub
#
echo "$DOCKER_PASSWORD" | docker login --username $DOCKER_USER --password-stdin

#
# Get tag
#
tag=$(cat $TRAVIS_BUILD_DIR/tag)

#
# Push images
#
docker push christianb93/bitcoin-controller:$tag
docker push christianb93/bitcoin-controller:latest


#
# Now clone into the repository that contains the Helm chart
#
cd /tmp
git clone https://www.github.com/christianb93/bitcoin-controller-helm-qa
cd bitcoin-controller-helm-qa

#
# Replace image version in values.yaml by new tag
#
cat values.yaml | sed "s/controller_image_tag.*/controller_image_tag: $tag/" > /tmp/values.yaml.patched
cp /tmp/values.yaml.patched values.yaml

#
# Get current version from Chart file
#
current_version=$(cat Chart.yaml | grep "version" | awk '{ print $2 }')
echo "Current chart version: $current_version"
#
# Update version in chart file as well
#
if [ "X$TRAVIS_TAG" != "X" ]; then
  chart_version=$TRAVIS_TAG
else
  chart_version="$current_version-dev$tag"
fi
echo "Using chart version $chart_version"
cat Chart.yaml | sed "s/version.*/version: $chart_version/" > /tmp/Chart.yaml.patched
cp /tmp/Chart.yaml.patched Chart.yaml

#
# Package again
#
helm package .
git add --all
