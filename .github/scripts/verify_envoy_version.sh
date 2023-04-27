#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

current_branch=$GITHUB_REF
GITHUB_DEFAULT_BRANCH='main'

if [ -z "$GITHUB_TOKEN" ]; then
  echo "GITHUB_TOKEN must be set"
  exit 1
fi

if [ -z "$current_branch" ]; then
  echo "GITHUB_REF must be set"
  exit 1
fi

# Get Consul and Envoy version 
SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
pushd $SCRIPT_DIR/../.. # repository root
consul_envoy_data_json=$(echo go run ./test/integration/consul-container/test/consul_envoy_version/consul_envoy_version.go)
# go back to where you started when finished
popd

if [ -z "$consul_envoy_data_json" ]; then
  echo "Error! Consul and Envoy versions not returned: $consul_envoy_data_json"
  exit 1
fi

# sanitize_consul_envoy_version removes characters from result that may contain new lines, spaces, and [...]
# example envoyVersions:[1.25.4 1.24.6 1.23.8 1.22.11] => 1.25.4 1.24.6 1.23.8 1.22.11
sanitize_consul_envoy_version() {
  local _consul_version=$(eval "$consul_envoy_data_json" | jq -r '.ConsulVersion')
  local _envoy_version=$(eval "$consul_envoy_data_json" | jq -r '.EnvoyVersions' | tr -d '"' | tr -d '\n' | tr -d ' '| tr -d '[]')
  echo "${_consul_version}" "${_envoy_version}"
}

# get major version for Consul and Envoy
get_major_version(){
  local _verison="$1"
  local _abbrVersion="$(cut -d "." -f1-2 <<< $_verison)"
  echo "${_abbrVersion}"
}

get_latest_envoy_version() {
  OUTPUT_FILE=$(mktemp)
  HTTP_CODE=$(curl -L --silent --output "$OUTPUT_FILE" -w "%{http_code}" \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer ${GITHUB_TOKEN}"\
    -H "X-GitHub-Api-Version: 2022-11-28" \
    https://api.github.com/repos/envoyproxy/envoy/releases/latest)
  if [[ ${HTTP_CODE} -lt 200 || ${HTTP_CODE} -gt 299 ]]; then
    cat >&2 "$OUTPUT_FILE"
    rm "$OUTPUT_FILE"
    exit 1
  fi
  _latest_envoy_version=$(jq -r '.tag_name' "$OUTPUT_FILE")
  echo "$_latest_envoy_version" 
  rm "$OUTPUT_FILE"
}

# major_envoy_versions takes multiple arguments
major_envoy_versions(){
  version=("$@")
  for i in "${version[@]}";
  do
    envoy_versions_array+="$(cut -d "." -f1-2 <<< $i)"
  done
  echo "${envoy_versions_array}"
}

# Get latest Envoy version from envoyproxy repo
released_envoy_version=$(get_latest_envoy_version)
major_released_envoy_version="${released_envoy_version[@]:1:4}"

validate_envoy_version_main(){
  echo "verify "main" GitHub branch has latest envoy version"
  # Get envoy version for current branch
  ENVOY_VERSIONS=$(sanitize_consul_envoy_version | awk '{print $2}' | tr ',' ' ')
  envoy_version_main_branch=$(get_major_version ${ENVOY_VERSIONS})

  if [[ "$envoy_version_main_branch" != "$major_released_envoy_version" ]]; then
    echo
    echo "Latest released Envoy version is: "$released_envoy_version""
    echo "ERROR! Branch $current_branch; Envoy versions: "$ENVOY_VERSIONS" needs to be updated."
    exit 1
    else
      echo "#### SUCCESS! ##### Compatible Envoy versions found: ${ENVOY_VERSIONS}"
      exit 0
  fi
}

if [[ "$current_branch" == *"$GITHUB_DEFAULT_BRANCH"* ]]; then
  validate_envoy_version_main
fi 

# filter consul and envoy version 
CONSUL_VERSION=$(sanitize_consul_envoy_version | awk '{print $1}')
ENVOY_VERSIONS=$(sanitize_consul_envoy_version | awk '{print $2}' | tr ',' ' ') 

# Get Consul and Envoy version from default branch
echo checking out "${GITHUB_DEFAULT_BRANCH}" branch
git checkout "${GITHUB_DEFAULT_BRANCH}"

# filter consul and envoy version from default branch 
CONSUL_VERSION_DEFAULT_BRANCH=$(sanitize_consul_envoy_version | awk '{print $1}')
ENVOY_VERSIONS_DEFAULT_BRANCH=$(sanitize_consul_envoy_version | awk '{print $2}' | tr ',' ' ') 

# Ensure required values are not empty
if [ -z "$CONSUL_VERSION" ] || [ -z "$CONSUL_VERSION_DEFAULT_BRANCH" ] || [ -z "$ENVOY_VERSIONS" ] || [ -z "$ENVOY_VERSIONS_DEFAULT_BRANCH" ]; then
  echo "Error! Consul version: $CONSUL_VERSION | Consul version default branch: $CONSUL_VERSION_DEFAULT_BRANCH | Envoy version: $ENVOY_VERSIONS | Envoy version default branch: $ENVOY_VERSIONS_DEFAULT_BRANCH cannot be empty"
  exit 1
fi

echo checking out branch: "${current_branch}"
git checkout "${current_branch}"

echo
echo "Branch ${current_branch} =>Consul version: ${CONSUL_VERSION}; Envoy Version: ${ENVOY_VERSIONS}" 
echo "Branch ${GITHUB_DEFAULT_BRANCH} =>Consul version: ${CONSUL_VERSION_DEFAULT_BRANCH}; Envoy Version: ${ENVOY_VERSIONS_DEFAULT_BRANCH}" 

## Get major Consul and Envoy versions on release and default branch
MAJOR_CONSUL_VERSION=$(get_major_version ${CONSUL_VERSION})
MAJOR_CONSUL_VERSION_DEFAULT_BRANCH=$(get_major_version ${CONSUL_VERSION_DEFAULT_BRANCH})
MAJOR_ENVOY_VERSION_DEFAULT_BRANCH=$(get_major_version ${ENVOY_VERSIONS_DEFAULT_BRANCH})

_envoy_versions=($ENVOY_VERSIONS)
_envoy_versions_default=($ENVOY_VERSIONS_DEFAULT_BRANCH)

## Validate supported envoy versions available - should be 4
echo
echo "Validating supported envoy versions available on branches: $current_branch and $GITHUB_DEFAULT_BRANCH"
if [ "${#_envoy_versions_default[@]}" != 4 ] || [ "${#_envoy_versions[@]}" != 4 ]; then
  echo "Branch $GITHUB_DEFAULT_BRANCH =>Consul version: ${CONSUL_VERSION_DEFAULT_BRANCH}; Envoy versions: $ENVOY_VERSIONS_DEFAULT_BRANCH"
  echo "Branch $current_branch =>Consul version: ${CONSUL_VERSION}; Envoy versions: $_envoy_versions"
  echo "ERROR! Envoy should have 4 compatible versions."
  exit 1
fi 

echo "Checking if branch $GITHUB_DEFAULT_BRANCH has latest Envoy version"
## 1. Check "main" GitHub branch has latest envoy version
if [[ "$MAJOR_ENVOY_VERSION_DEFAULT_BRANCH" != "$major_released_envoy_version" ]]; then
  echo
  echo "Latest released Envoy version is: "$released_envoy_version""
  echo "ERROR! Branch $GITHUB_DEFAULT_BRANCH; Envoy versions: "$ENVOY_VERSIONS_DEFAULT_BRANCH" needs to be updated."
  exit 1
  else
    echo "#### SUCCESS! #####. Compatible Envoy versions found: ${ENVOY_VERSIONS_DEFAULT_BRANCH}"
    echo

    ## 2. Check main branch and release branch support the same Envoy major versions
    ## Get the major Consul version on the main and release branch. If both branches have
    ## the same major Consul version, verify both branches have the same major Envoy versions.
    ## Return error if major envoy versions are not the same.
    echo "Checking branch $current_branch and $GITHUB_DEFAULT_BRANCH have the same compatible major Envoy versions."
    consul_version_diff=$(echo "$MAJOR_CONSUL_VERSION_DEFAULT_BRANCH $MAJOR_CONSUL_VERSION" | awk '{print $1 - $2}')
    check=$(echo "$consul_version_diff == 0" | bc -l)

    if (( $check )); then
      echo "Branch $current_branch and $GITHUB_DEFAULT_BRANCH have the same major Consul version "$MAJOR_CONSUL_VERSION""
      echo "Validating branches have the same Envoy major versions..."
      _major_envoy_versions=$(major_envoy_versions $ENVOY_VERSIONS)
      _major_envoy_versions_default=$(major_envoy_versions $ENVOY_VERSIONS_DEFAULT_BRANCH)

      if [[ "$_major_envoy_versions_default" != "$_major_envoy_versions" ]]; then
          echo "Branch $GITHUB_DEFAULT_BRANCH =>Envoy versions: $_major_envoy_versions"
          echo "Branch $current_branch =>Envoy versions: $_major_envoy_versions_default"
          echo "ERROR! Branches should support the same major versions for envoy."
          exit 1
          else
            echo "#### SUCCESS! #####. Compatible Envoy major versions found: $ENVOY_VERSIONS_DEFAULT_BRANCH"
      fi
      else 
        echo "No validation needed. Branches have different Consul versions"
    fi
fi