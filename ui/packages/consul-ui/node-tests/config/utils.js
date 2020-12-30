/* eslint-env node */

const test = require('tape');

const utils = require('../../config/utils.js');

test(
  'utils.respositoryYear parses the year out correctly',
  function(t) {
    const expected = '2020';
    const actual = utils.repositoryYear('2020-10-14 16:34:57 -0700')
    t.equal(actual, expected, 'It parses the year correctly');
    t.end();
  }
);
test(
  'utils.binaryVersion parses the version out correctly',
  function(t) {
    const expected = '1.9.0';
    const actual = utils.binaryVersion()(`

	Version = "1.9.0"

	VersionPrerelease = "dev"

`)
    t.equal(actual, expected, 'It parses the version correctly');
    t.end();
  }
);
