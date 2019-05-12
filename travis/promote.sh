#!/bin/bash
#
# Package the Helm chart that we just tested and
# push into final target repository
#
# Assumes that the following environment variables are set
# TRAVIS_BUILD_DIR    - directory into which Travis clones the bitcoin-controller-helm-qa repo

#
# Get chart version
#
cd $TRAVIS_BUILD_DIR
chart_version=$(cat Chart.yaml  | grep "version" | awk '{ print $2 }')
echo "Found chart version $chart_version"

#
# Is this a stable version, i.e. of the form X.Y without a build id?
#
stripped_chart_version=$(echo $chart_version | sed 's/-dev[a-z,0-9]*//')
if [ "$stripped_chart_version" != "$chart_version" ]; then
  # This is not a stable build - exit
  echo "Not a stable build - exiting"
  exit 0
fi

#
# If we get to this point, this is a stable version
#
echo "This looks like a stable version - packaging"
helm package .
#
# Now get the current repository
#
cd ..
git clone https://github.com/christianb93/bitcoin-controller-helm
cd bitcoin-controller-helm
cp $TRAVIS_BUILD_DIR/*.tgz .
ls -l
helm repo index .
cat index.yaml
