/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import domClickFirstAnchor from 'consul-ui/utils/dom/click-first-anchor';
import { module, test } from 'qunit';

module('Unit | Utility | dom/click first anchor', function () {
  test('it does nothing if the clicked element is generally a clickable thing', function (assert) {
    assert.expect(4);

    const closest = function () {
      return {
        querySelector: function () {
          assert.ok(false);
        },
      };
    };
    const click = domClickFirstAnchor(closest);
    ['INPUT', 'LABEL', 'A', 'Button'].forEach(function (item) {
      const expected = null;
      const actual = click({
        target: {
          nodeName: item,
        },
      });
      assert.equal(actual, expected);
    });
  });
  test("it does nothing if an anchor isn't found", function (assert) {
    const closest = function () {
      return {
        querySelector: function () {
          return null;
        },
      };
    };
    const click = domClickFirstAnchor(closest);
    const expected = null;
    const actual = click({
      target: {
        nodeName: 'DIV',
      },
    });
    assert.equal(actual, expected);
  });
  test('it dispatches the result of `mouseup`, `mousedown`, `click` if an anchor is found', function (assert) {
    assert.expect(3);
    const expected = ['mousedown', 'mouseup', 'click'];
    const closest = function () {
      return {
        querySelector: function () {
          return {
            dispatchEvent: function (ev) {
              const actual = ev.type;
              assert.equal(actual, expected.shift());
            },
          };
        },
      };
    };
    const click = domClickFirstAnchor(closest);
    click({
      target: {
        nodeName: 'DIV',
      },
    });
  });
});
