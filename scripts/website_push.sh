#!/bin/bash

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that directory
cd $DIR

# Add the git remote if it doesn't exist
git remote | grep heroku || {
  git remote add heroku git@heroku.com:consul-www.git
}

# Push the subtree (force)
git push heroku `git subtree split --prefix website master`:master --force
