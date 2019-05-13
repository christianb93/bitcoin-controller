#!/bin/bash
#
# Package the Helm chart that we just tested and
# push into final target repository
#
# Assumes that the following environment variables are set
# TRAVIS_BUILD_DIR    - directory into which Travis clones the bitcoin-controller-helm-qa repo
# TRAVIS_HOME         - home directory, we use $TRAVIS_HOME/cache as the cache directory



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
# If we get to this point, this is a stable version. Let us package this.
# To make sure that the final package has the correct name, we need to
# rename the build directory first
#
echo "This looks like a stable version - packaging"
cd ..
ls
rm -rf bitcoin-controller
mv bitcoin-controller-helm-qa bitcoin-controller
cd bitcoin-controller
pwd
cat Chart.yaml | sed "s/bitcoin-controller-helm-qa/bitcoin-controller/" > /tmp/Chart.yaml.patched
cp /tmp/Chart.yaml.patched Chart.yaml
cat Chart.yaml
# Before we package, we remove a few files that we no longer need and that should not end up in the archive
rm -rf setupVMAndCluster.sh
rm -rf afterBuild.sh
rm -rf .travis.yml
rm -rf .git
# Ask helm to create a package
helm package .

#
# Now get the current repository, copy the generated tar file there
# and rebuild the index. Be careful, $TRAVIS_BUILD_DIR now points
# into the old repo dir that we just renamed, and the tar file
# is in ../bitcoin-controller 
#
cd ..
git clone https://github.com/christianb93/bitcoin-controller-helm
cd bitcoin-controller-helm
cp ../bitcoin-controller/*.tgz .
ls -l
helm repo index .
cat index.yaml

#
# Now push this change back
#
git add --all
git config --global user.name christianb93
git config --global user.email me@unknown
git config remote.origin.url https://$GITHUB_USER:$GITHUB_PASSWORD@github.com/christianb93/bitcoin-controller-helm
git commit -m "Automated deployment of packaged chart version $chart_version"
git push origin master
