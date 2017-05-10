#!/usr/bin/env bash
set -e

# Get the parent directory of where this script is and cd there
DIR="$(cd "$(dirname "$(readlink -f "$0")")/.." && pwd)"
cd "$DIR"

echo "--> Ensuring dependencies are up to date..."
bundle install --quiet

# Generate the tag
DEPLOY="../pkg/web_ui"

echo "--> Clearing existing deploy..."
rm -rf $DEPLOY
mkdir -p $DEPLOY

echo "--> Compiling scss..."
bundle exec sass --quiet styles/base.scss static/base.css

echo "--> Compiling web ui..."
bundle exec ruby scripts/compile.rb

# Copy into deploy
echo "--> Moving into deploy"
shopt -s dotglob
cp -r "$DIR/static" "$DEPLOY/"
cp index.html "$DEPLOY/index.html"

# Magic scripting
echo "--> Running magic scripts"
sed -E -e "/ASSETS/,/\/ASSETS/ d" -ibak "$DEPLOY/index.html"
sed -E -e "s#<\/body>#<script src=\"static/application.min.js\"></script></body>#" -ibak $DEPLOY/index.html

# Remove the backup file from sed
echo "--> Cleaning up..."
rm -f "$DEPLOY/index.htmlbak"
