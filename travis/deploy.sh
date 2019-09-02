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

set -e

#
# Login to Docker hub
#

echo "$DOCKER_PASSWORD" | docker login --username $DOCKER_USER --password-stdin
# Not a good idea, as it makes our password visible in the build log - just for testing purposes
cat /home/travis/.docker/config.json

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
# Get current version from Chart file and remove build tag
#
current_version=$(cat Chart.yaml | grep "version" | awk '{ print $2 }' | sed 's/-dev[a-z,0-9]*//')
echo "Current chart version: $current_version"
#
# Update chart version and appVersion  in chart file
#
if [ "X$TRAVIS_TAG" != "X" ]; then
  chart_version=$TRAVIS_TAG
else
  chart_version="$current_version-dev$tag"
fi
echo "Using chart version $chart_version"
cat Chart.yaml | sed "s/version.*/version: $chart_version/"  | sed "s/appVersion.*/appVersion: $tag/" > /tmp/Chart.yaml.patched
cp /tmp/Chart.yaml.patched Chart.yaml


git add --all
git config --global user.name christianb93
git config --global user.email me@unknown
git config remote.origin.url https://$GITHUB_USER:$GITHUB_PASSWORD@github.com/christianb93/bitcoin-controller-helm-qa
git commit -m "Automated deployment of chart version $chart_version"
git push origin master
