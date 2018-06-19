#
# NOTE: This file is meant to be sourced from other bash scripts/shells
#
# It provides all the scripting around building Consul and the release process

pushd $(dirname ${BASH_SOURCE[0]}) > /dev/null
pushd ../functions > /dev/null
FUNC_DIR=$(pwd)
popd > /dev/null
popd > /dev/null

func_sources=$(find ${FUNC_DIR} -mindepth 1 -maxdepth 1 -name "*.sh" -type f | sort -n)

for src in $func_sources
do
   source $src
done