/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { isLegacy } from 'consul-ui/helpers/token/is-legacy';
import { module, test } from 'qunit';

module('Unit | Helper | token/is-legacy', function () {
  test('it returns true if the token has a Legacy=true', function (assert) {
    const actual = isLegacy([{ Legacy: true }]);
    assert.ok(actual);
  });
  test('it returns false if the token has a Legacy=false', function (assert) {
    const actual = isLegacy([{ Legacy: false }]);
    assert.notOk(actual);
  });
  test('it returns true if the token has Rules', function (assert) {
    const actual = isLegacy([{ Rules: 'some rules' }]);
    assert.ok(actual);
  });
  test('it returns false if the token has Rules but those rules are empty', function (assert) {
    const actual = isLegacy([{ Rules: '' }]);
    assert.notOk(actual);
  });
  test('it returns false if the token has Rules but those rules is null', function (assert) {
    const actual = isLegacy([{ Rules: null }]);
    assert.notOk(actual);
  });
  // passing arrays
  test("it returns false if things don't have Legacy or Rules", function (assert) {
    const actual = isLegacy([[{}, {}]]);
    assert.notOk(actual);
  });
  test('it returns true if one token has Legacy=true', function (assert) {
    const actual = isLegacy([[{}, { Legacy: true }]]);
    assert.ok(actual);
  });
  test('it returns false if one token as Legacy=false', function (assert) {
    const actual = isLegacy([[{}, { Legacy: false }]]);
    assert.notOk(actual);
  });
  test('it returns true if one token has Rules', function (assert) {
    const actual = isLegacy([[{}, { Rules: 'some rules' }]]);
    assert.ok(actual);
  });
  test('it returns false if tokens have no Rules, or has Rules but those rules are empty', function (assert) {
    const actual = isLegacy([[{}, { Rules: '' }]]);
    assert.notOk(actual);
  });
  test('it returns false if a token is marked as legacy, has Rules but those rules are empty', function (assert) {
    // this may seem strange, but empty Rules should override Legacy, this only happens
    // when a legacy token that has already been loaded has its rules wiped out
    // WITHOUT then the ui refreshing
    const actual = isLegacy([{ Legacy: true, Rules: '' }]);
    assert.notOk(actual);
  });
});
