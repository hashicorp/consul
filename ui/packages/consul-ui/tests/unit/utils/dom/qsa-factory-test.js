/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import qsaFactory from 'consul-ui/utils/dom/qsa-factory';
import { module, test } from 'qunit';

module('Unit | Utility | qsa factory', function () {
  test('querySelectorAll is called on `document` when called with document', function (assert) {
    const expected = 'html';
    const $$ = qsaFactory({
      querySelectorAll: function (sel) {
        assert.strictEqual(sel, expected);
        return true;
      },
    });
    assert.ok($$(expected));
  });
  test('querySelectorAll is called on `context` when called with context', function (assert) {
    const expected = 'html';
    const context = {
      querySelectorAll: function (sel) {
        assert.strictEqual(sel, expected);
        return true;
      },
    };
    const $$ = qsaFactory({
      // this should never be called
      querySelectorAll: function (sel) {
        assert.strictEqual(sel, expected);
        return false;
      },
    });
    assert.ok($$(expected, context));
  });
});
