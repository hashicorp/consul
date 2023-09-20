/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import domClosest from 'consul-ui/utils/dom/closest';
import { module, skip, test } from 'qunit';
import sinon from 'sinon';

module('Unit | Utility | dom/closest', function () {
  test('it calls Element.closest with the specified selector', function (assert) {
    const el = {
      closest: sinon.stub().returnsArg(0),
    };
    const expected = 'selector';
    const actual = domClosest(expected, el);
    assert.equal(actual, expected);
    assert.ok(el.closest.calledOnce);
  });
  test("it fails silently/null if calling closest doesn't work/exist", function (assert) {
    const expected = null;
    const actual = domClosest('selector', {});
    assert.equal(actual, expected);
  });
  skip('polyfill closest');
});
