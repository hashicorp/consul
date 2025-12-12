/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import getComponentFactory from 'consul-ui/utils/dom/get-component-factory';
import { module, test } from 'qunit';

module('Unit | Utility | dom/get component factory', function () {
  test("it uses lookup to locate the instance of the component based on the DOM element's id", function (assert) {
    const expected = 'name';
    let getComponent = getComponentFactory({
      lookup: function () {
        return { id: expected };
      },
    });
    assert.strictEqual(typeof getComponent, 'function', 'returns a function');
    const actual = getComponent({
      getAttribute: function (name) {
        return 'id';
      },
    });
    assert.strictEqual(actual, expected, 'performs a lookup based on the id');
  });
  test("it returns null if it can't find it", function (assert) {
    const expected = null;
    let getComponent = getComponentFactory({
      lookup: function () {
        return { id: '' };
      },
    });
    const actual = getComponent({
      getAttribute: function (name) {
        return 'non-existent';
      },
    });
    assert.strictEqual(actual, expected);
  });
  test('it returns null if there is no id', function (assert) {
    const expected = null;
    let getComponent = getComponentFactory({
      lookup: function () {
        return { id: '' };
      },
    });
    const actual = getComponent({
      getAttribute: function (name) {
        return;
      },
    });
    assert.strictEqual(actual, expected);
  });
});
