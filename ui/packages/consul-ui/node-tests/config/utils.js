/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-env node */

const test = require('tape');

const utils = require('../../config/utils.js');

test('utils.respositoryYear parses the year out correctly', function (t) {
  const expected = '2020';
  const actual = utils.repositoryYear('2020-10-14 16:34:57 -0700');
  t.equal(actual, expected, 'It parses the year correctly');
  t.end();
});
test('utils.binaryVersion parses the version out correctly', function (t) {
  const expected = '1.15.0-dev';
  const actual = utils.binaryVersion()(`1.15.0-dev`);
  t.equal(actual, expected, 'It parses the version correctly');
  t.end();
});
