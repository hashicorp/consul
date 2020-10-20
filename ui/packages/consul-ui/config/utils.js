const read = require('fs').readFileSync;
const exec = require('child_process').execSync;

// See tests ../node-tests/config/utils.js
const repositoryYear = function(date = exec('git show -s --format=%ci HEAD')) {
  return date
    .toString()
    .trim()
    .split('-')
    .shift();
};
const repositorySHA = function(sha = exec('git rev-parse --short HEAD')) {
  return sha.toString().trim();
};
const binaryVersion = function(repositoryRoot) {
  return function(versionFileContents = read(`${repositoryRoot}/version/version.go`)) {
    // see /scripts/dist.sh:8
    return versionFileContents
      .toString()
      .split('\n')
      .find(function(item, i, arr) {
        return item.indexOf('Version =') !== -1;
      })
      .trim()
      .split('"')[1];
  };
};
module.exports = {
  repositoryYear: repositoryYear,
  repositorySHA: repositorySHA,
  binaryVersion: binaryVersion,
};
