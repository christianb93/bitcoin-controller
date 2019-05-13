#!/bin/bash
#
# Clean up tasks and cache maintenance after build 
# Assumes that the following environment variables are set
# TRAVIS_BUILD_DIR    - directory into which Travis clones the bitcoin-controller-helm-qa repo
# TRAVIS_HOME         - home directory, we use $TRAVIS_HOME/cache as the cache directory

# 
# Clean up after test. At this point, our kind cluster should still be running, so we stop it
#
kind delete cluster
# Save docker image if there is no cached instance yet
# We skip this if CACHE_DOCKER_IMAGES is not on
if [ "X$CACHE_DOCKER_IMAGES" == "Xon" ]; then
  if [ -f "$TRAVIS_HOME/cache/kind_node_image.tar" ]; then
    echo "There is already a version of the kind node image in the cache, not replacing"
  else
    echo "Unloading kindest/node to cache"
    docker save --output $TRAVIS_HOME/cache/kind_node_image.tar kindest/node
  fi
else 
  echo "Set CACHE_DOCKER_IMAGES to on to enable caching of kind node image"
fi 

